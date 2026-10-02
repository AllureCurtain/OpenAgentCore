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

// buildRevision is set by the release build. The host binary is copied from that image.
var buildRevision = "development"

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

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: oac status|start|stop|apply|core-key|rotate-core-key|backup|uninstall|domain|setup-sandbox|domain-serve")
}

func run(ctx context.Context, command string, args []string) error {
	if command == "setup-sandbox" && os.Getenv("OAC_INNER_SETUP") == "1" {
		return setupSandboxInner(ctx)
	}
	root, err := installDir()
	if err != nil {
		return err
	}
	in := installation{root: root, data: dataDir(root)}
	runner := execRunner{dir: root}
	switch command {
	case "status":
		return status(ctx, in, runner)
	case "start":
		return withLock(root, func() error { return runner.Run(ctx, "start") })
	case "stop":
		return withLock(root, func() error { return runner.Run(ctx, "stop") })
	case "apply":
		return withLock(root, func() error { return apply(ctx, runner) })
	case "core-key":
		return coreKeyCommand(ctx, in, runner, args)
	case "rotate-core-key":
		return withLock(root, func() error { return rotateCoreKey(ctx, in, runner) })
	case "backup":
		if len(args) != 1 {
			return errors.New("Usage: oac backup DEST")
		}
		return withLock(root, func() error { return backup(ctx, root, runner, args[0]) })
	case "uninstall":
		return withLock(root, func() error { return uninstall(ctx, root, runner, args) })
	case "domain":
		if len(args) != 1 {
			return errors.New("Usage: oac domain HOSTNAME")
		}
		return domainClient(in, args[0])
	case "domain-serve":
		return serveDomain(ctx, in, liveEffects(in.data, runner))
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

func status(ctx context.Context, in installation, runner Runner) error {
	public, _ := envValue(in.envPath(), "OAC_PUBLIC_URL")
	if public == "" {
		public = "http://localhost:8080"
	}
	fmt.Println("Public URL:", public)
	if buildRevision != "" && buildRevision != "development" {
		fmt.Println("Revision:", buildRevision)
	}
	domain := in.readStatus()
	fmt.Printf("Domain: %s\n", domain.State)
	if domain.Message != nil {
		fmt.Println(*domain.Message)
	}
	return runner.Run(ctx, "ps")
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

func backup(ctx context.Context, root string, runner Runner, dest string) error {
	if err := runner.Run(ctx, "stop"); err != nil {
		return err
	}
	started := false
	defer func() {
		if !started {
			_ = runner.Run(context.Background(), "start")
		}
	}()
	if err := archiveInstall(root, dest); err != nil {
		return err
	}
	started = true
	return runner.Run(ctx, "start")
}

func archiveInstall(root, dest string) error {
	archive := exec.Command("tar", "-C", filepath.Dir(root), "-czf", dest, filepath.Base(root))
	archive.Stdout, archive.Stderr = os.Stdout, os.Stderr
	if err := archive.Run(); err != nil {
		return errors.New("backup archive failed")
	}
	return nil
}

func uninstall(ctx context.Context, root string, runner Runner, args []string) error {
	if len(args) != 1 || args[0] != "--yes" {
		fmt.Fprintln(os.Stderr, "This deletes the installation directory, its containers and its images.")
		fmt.Fprintln(os.Stderr, "Run oac uninstall --yes to continue.")
		return errors.New("uninstallation was not confirmed")
	}
	_ = runner.Run(ctx, "down", "--rmi", "all", "--remove-orphans")
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	fmt.Println("Installation removed.")
	return nil
}

func domainClient(in installation, hostname string) error {
	key, err := readKeyViaFile(in.data)
	if err != nil {
		return errors.New("cannot read the Core key; run oac core-key --show from the installation account")
	}
	payload, _ := json.Marshal(map[string]string{"hostname": hostname})
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
		return errors.New("domain service is unavailable; run oac status")
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode == 202 {
		fmt.Println("Requesting and verifying HTTPS. This can take a few minutes.")
		return nil
	}
	var failure struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &failure)
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
