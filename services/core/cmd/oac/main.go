package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1], os.Args[2:]); err != nil {
		var domain *domainError
		if errors.As(err, &domain) {
			fmt.Fprintln(os.Stderr, domain.Message)
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		os.Exit(1)
	}
}

// buildRevision is set by the release build.
var buildRevision = "development"

func usage() {
	fmt.Fprintf(os.Stderr, "oac (%s)\nUsage: oac apply|core-key|rotate-core-key|domain|setup-sandbox|init|gateway|healthcheck\n", buildRevision)
}

func run(ctx context.Context, command string, args []string) error {
	if command == "setup-sandbox" && os.Getenv("OAC_INNER_SETUP") == "1" {
		return setupSandboxInner(ctx)
	}
	switch command {
	case "init":
		return initCommand(ctx)
	case "healthcheck":
		return healthcheck(ctx)
	}
	root, err := installDir()
	if err != nil {
		return err
	}
	in := installation{root: root, data: dataDir(root)}
	runner := execRunner{dir: root}
	switch command {
	case "apply":
		return withLock(root, func() error { return apply(ctx, runner) })
	case "core-key":
		return coreKeyCommand(ctx, in, runner, args)
	case "rotate-core-key":
		return withLock(root, func() error { return rotateCoreKey(ctx, in, runner) })
	case "domain":
		return domainCommand(in, args)
	case "gateway":
		return serveGateway(ctx, in, liveEffects(in.data, runner))
	case "setup-sandbox":
		return setupSandbox(ctx, root, runner)
	default:
		usage()
		return errors.New("unknown command")
	}
}

func installDir() (string, error) {
	if dir := os.Getenv("OAC_INSTALL_DIR"); dir != "" {
		return dir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "compose.yaml")); err == nil {
		return dir, nil
	}
	return "", errors.New("run oac from an installation directory that contains compose.yaml")
}

func dataDir(root string) string {
	if dir := os.Getenv("OAC_DATA_MOUNT"); dir != "" {
		return dir
	}
	return filepath.Join(root, "data")
}

func withLock(root string, fn func() error) error {
	file, err := os.OpenFile(filepath.Join(root, ".oac.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return fn()
}

func apply(ctx context.Context, runner Runner) error {
	if err := runner.Run(ctx, "run", "--rm", "--no-deps", "--entrypoint", "/usr/local/bin/oac-core", "core", "check-config"); err != nil {
		return errors.New("configuration check failed; no service was changed")
	}
	return runner.Run(ctx, "up", "-d", "--wait")
}

func coreKeyCommand(ctx context.Context, in installation, runner Runner, args []string) error {
	path := filepath.Join(in.data, "secrets", "web", "core.key")
	if len(args) == 0 {
		fmt.Println(path)
		return nil
	}
	if len(args) == 1 && args[0] == "--show" {
		return runner.Run(ctx, "--profile", "tools", "run", "--rm", "--no-deps", "credentials")
	}
	return errors.New("Usage: oac core-key [--show]")
}

func rotateCoreKey(ctx context.Context, in installation, runner Runner) error {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	key := hex.EncodeToString(buf)
	keyPath := filepath.Join(in.data, "secrets", "web", "core.key")
	digestPath := filepath.Join(in.data, "secrets", "core", "core-key-digests.json")
	if err := writeSecret(keyPath, key+"\n"); err != nil {
		return err
	}
	raw, err := json.Marshal([]string{keyDigest(key)})
	if err != nil {
		return err
	}
	if err := writeSecret(digestPath, string(raw)+"\n"); err != nil {
		return err
	}
	return runner.Run(ctx, "restart", "core", "web")
}

func writeSecret(path, contents string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return os.ErrInvalid
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(contents), 0o600); err != nil {
		return err
	}
	if err := os.Chown(temporary, int(stat.Uid), int(stat.Gid)); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}

func domainCommand(in installation, args []string) error {
	var hostname, confirm string
	for i := 0; i < len(args); i++ {
		if args[i] == "--confirm" && i+1 < len(args) {
			confirm = args[i+1]
			i++
			continue
		}
		if hostname != "" || strings.HasPrefix(args[i], "-") {
			return errors.New("Usage: oac domain HOSTNAME [--confirm https://HOSTNAME]")
		}
		hostname = args[i]
	}
	if hostname == "" {
		return errors.New("Usage: oac domain HOSTNAME [--confirm https://HOSTNAME]")
	}
	return domainClient(in, hostname, confirm)
}

func domainClient(in installation, hostname, confirm string) error {
	key, err := readKeyViaFile(in.data)
	if err != nil {
		return errors.New("cannot read the Core key; run oac core-key --show from the installation account")
	}
	body := map[string]string{"hostname": hostname}
	if confirm != "" {
		body["confirm_public_url_change"] = confirm
	}
	payload, _ := json.Marshal(body)
	request, err := http.NewRequest(http.MethodPost, "http://localhost/domain", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Length", fmt.Sprint(len(payload)))
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", filepath.Join(in.data, "domain", "api.sock"))
	}}
	defer transport.CloseIdleConnections()
	response, err := transport.RoundTrip(request)
	if err != nil {
		return errors.New("domain setup is unavailable; run docker compose ps and check the gateway")
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode == 202 {
		fmt.Println("Requesting and verifying HTTPS. This can take a few minutes.")
		return nil
	}
	var failure struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &failure)
	if failure.Error.Message == "" {
		failure.Error.Message = "Domain setup failed"
	}
	return errors.New(failure.Error.Message)
}

func readKeyViaFile(data string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(data, "secrets", "web", "core.key"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

// gatewayCaddy is the managed gateway's proxy command; a variable for tests.
var gatewayCaddy = []string{"/usr/local/bin/caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"}

// serveGateway runs Caddy without root beside the domain API. The container
// stops when either one stops.
func serveGateway(ctx context.Context, in installation, effects Effects) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	caddy := exec.Command(gatewayCaddy[0], gatewayCaddy[1:]...)
	caddy.Stdout, caddy.Stderr = os.Stdout, os.Stderr
	if os.Geteuid() == 0 {
		caddy.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65532, Gid: 65532, Groups: []uint32{}}}
	}
	if err := caddy.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- caddy.Wait() }()
	served := make(chan error, 1)
	go func() { served <- serveDomain(ctx, in, effects) }()
	select {
	case err := <-exited:
		cancel()
		<-served
		if err == nil {
			err = errors.New("the gateway proxy stopped")
		}
		return err
	case err := <-served:
		_ = caddy.Process.Signal(syscall.SIGTERM)
		<-exited
		return err
	}
}

func serveDomain(ctx context.Context, in installation, effects Effects) error {
	if err := os.MkdirAll(filepath.Join(in.data, "domain"), 0o755); err != nil {
		return err
	}
	socket := filepath.Join(in.data, "domain", "api.sock")
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	if err := os.Chmod(socket, 0o666); err != nil {
		listener.Close()
		return err
	}
	key, err := coreKey(in.data)
	if err != nil {
		listener.Close()
		return err
	}
	current := in.readStatus()
	if current.State == "checking" || current.State == "applying" {
		message := "Domain setup was interrupted. Retry the same hostname."
		current.State = "failed"
		current.Message = &message
		_ = in.writeStatus(current)
	}
	server := &http.Server{Handler: domainHandler(in, effects, key)}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func domainHandler(in installation, effects Effects, key string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, key) {
			writeDomainError(w, 401, "unauthorized", "Installation authentication failed")
			return
		}
		if r.URL.Path != "/domain" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, in.readStatus())
		case http.MethodPost:
			raw, err := io.ReadAll(io.LimitReader(r.Body, 2048))
			if err != nil || len(raw) == 0 {
				writeDomainError(w, 400, "invalid_request", "A small JSON request body is required")
				return
			}
			var body struct {
				Hostname string `json:"hostname"`
				Confirm  string `json:"confirm_public_url_change"`
			}
			if json.Unmarshal(raw, &body) != nil || body.Hostname == "" {
				writeDomainError(w, 400, "invalid_request", "Expected hostname and optional confirm_public_url_change")
				return
			}
			accepted, err := acceptDomain(r.Context(), in, effects, body.Hostname, body.Confirm)
			if err != nil {
				var domain *domainError
				if errors.As(err, &domain) {
					writeDomainError(w, domain.Status, domain.Code, domain.Message)
					return
				}
				writeDomainError(w, 500, "domain_setup_failed", "Domain setup failed. Check gateway and installation logs, then retry.")
				return
			}
			go func() { _ = finishDomain(context.Background(), in, effects, accepted) }()
			writeJSON(w, 202, accepted.status)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func writeDomainError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "invalid response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
