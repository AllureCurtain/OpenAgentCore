package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	domainCooldown = 5 * time.Minute
	verifyTimeout  = 3 * time.Minute
)

var hostnamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// Status is the domain document Web polls.
type Status struct {
	Supported bool    `json:"supported"`
	State     string  `json:"state"`
	PublicURL *string `json:"public_url"`
	TargetURL *string `json:"target_url"`
	Message   *string `json:"message"`
}

type domainError struct {
	Code    string
	Message string
	Status  int
}

func (e *domainError) Error() string { return e.Message }

func domainErr(status int, code, message string) *domainError {
	return &domainError{Status: status, Code: code, Message: message}
}

// Effects are the host operations a domain change performs. Tests stub them.
type Effects struct {
	Now      func() time.Time
	Resolve  func(host string) bool
	Bindings func(ctx context.Context) (int, error)
	Reload   func(ctx context.Context) error
	Verify   func(ctx context.Context, host, installationID string) error
	Recreate func(ctx context.Context) error
}

func (e Effects) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

type installation struct {
	root string
	data string
}

func (in installation) envPath() string { return filepath.Join(in.root, ".env") }

func (in installation) statusPath() string { return filepath.Join(in.data, "domain", "status.json") }

func (in installation) sitePath() string { return filepath.Join(in.data, "caddy", "site.caddy") }

func (in installation) readStatus() Status {
	var status Status
	raw, err := os.ReadFile(in.statusPath())
	if err != nil || json.Unmarshal(raw, &status) != nil || status.State == "" {
		return Status{Supported: true, State: "unconfigured"}
	}
	status.Supported = true
	return status
}

func (in installation) writeStatus(status Status) error {
	status.Supported = true
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return writePrivate(in.statusPath(), append(raw, '\n'))
}

func writePrivate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func canonicalHostname(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) > 253 || !strings.Contains(value, ".") {
		return "", domainErr(400, "invalid_hostname", "Enter a DNS hostname, without a scheme, port or path")
	}
	if strings.HasSuffix(value, ".localhost") || strings.HasSuffix(value, ".local") || strings.HasSuffix(value, ".internal") {
		return "", domainErr(400, "invalid_hostname", "Use a publicly registered DNS hostname")
	}
	for _, label := range strings.Split(value, ".") {
		if !hostnamePattern.MatchString(label) {
			return "", domainErr(400, "invalid_hostname", "Enter a DNS hostname, without a scheme, port or path")
		}
	}
	if net.ParseIP(value) != nil {
		return "", domainErr(400, "invalid_hostname", "Use a DNS hostname rather than an IP address")
	}
	return value, nil
}

func siteDocument(installationID string, hosts []string) string {
	var builder strings.Builder
	seen := map[string]bool{}
	for _, host := range hosts {
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		fmt.Fprintf(&builder, "https://%s {\n\thandle /_oac/installation/verify {\n\t\trespond %q 200\n\t}\n\timport /etc/oac/routes.caddy\n}\n", host, installationID)
	}
	if builder.Len() == 0 {
		return "\n"
	}
	return builder.String()
}

func httpsHost(origin string) string {
	if !strings.HasPrefix(origin, "https://") {
		return ""
	}
	host := strings.TrimPrefix(origin, "https://")
	if strings.ContainsAny(host, "/:?#") || host == "" {
		return ""
	}
	return host
}

type domainJob struct {
	host, target, current, installationID string
	previous, next                        []byte
	status                                Status
}

// acceptDomain validates a domain change and records it as checking.
// Certificate issuance and the Core restart happen in finishDomain, so Web's
// short request can return before they complete.
func acceptDomain(ctx context.Context, in installation, effects Effects, hostname, confirmation string) (domainJob, error) {
	host, err := canonicalHostname(hostname)
	if err != nil {
		return domainJob{}, err
	}
	target := "https://" + host
	if confirmation != "" && confirmation != target {
		return domainJob{}, domainErr(400, "invalid_confirmation", "The confirmation must equal the new HTTPS URL")
	}
	ingress, _ := envValue(in.envPath(), "OAC_INGRESS")
	if ingress != "managed" {
		return domainJob{}, domainErr(400, "domain_setup_unavailable", "This installation uses an external reverse proxy. Configure HTTPS there, then set OAC_PUBLIC_URL and run oac apply.")
	}
	current, _ := envValue(in.envPath(), "OAC_PUBLIC_URL")
	status := in.readStatus()
	if status.State == "failed" && status.TargetURL != nil && *status.TargetURL == target && status.Message != nil {
		if failedAt, ok := failedTime(in); ok && effects.now().Sub(failedAt) < domainCooldown {
			wait := domainCooldown - effects.now().Sub(failedAt)
			return domainJob{}, domainErr(409, "domain_cooldown", fmt.Sprintf("Wait %s before retrying %s. Repeating a failed certificate request too quickly can exhaust the certificate authority limit.", wait.Round(time.Second), host))
		}
	}
	if effects.Resolve != nil && !effects.Resolve(host) {
		return domainJob{}, domainErr(409, "hostname_unresolved", host+" does not resolve. Add A/AAAA records for it that point at this server, then retry when DNS returns them.")
	}
	bound := 0
	if effects.Bindings != nil {
		bound, err = effects.Bindings(ctx)
		if err != nil {
			return domainJob{}, domainErr(409, "installation_not_running", "Start the installation with oac start before configuring its domain")
		}
	}
	if bound > 0 && confirmation != target {
		return domainJob{}, domainErr(409, "public_url_confirmation_required", "Changing this address can disconnect existing nodes and executors. Confirm the new URL to continue; retain the old route while existing work uses it.")
	}
	installationID, err := installationIdentity(in.data)
	if err != nil {
		return domainJob{}, domainErr(409, "installation_not_ready", "The installation identity is missing")
	}
	previous, _ := os.ReadFile(in.sitePath())
	if len(previous) == 0 {
		previous = []byte("\n")
	}
	job := domainJob{
		host: host, target: target, current: current, installationID: strings.TrimSpace(installationID),
		previous: previous, next: []byte(siteDocument(strings.TrimSpace(installationID), []string{httpsHost(current), host})),
		status: Status{State: "checking", PublicURL: httpsOrNil(current), TargetURL: &target},
	}
	if err := in.writeStatus(job.status); err != nil {
		return domainJob{}, err
	}
	return job, nil
}

func finishDomain(ctx context.Context, in installation, effects Effects, job domainJob) error {
	host, target, current := job.host, job.target, job.current
	installationID := job.installationID
	previous := job.previous
	if err := writePrivate(in.sitePath(), job.next); err != nil {
		return err
	}
	restore := func(cause error) error {
		_ = writePrivate(in.sitePath(), previous)
		if effects.Reload != nil {
			_ = effects.Reload(ctx)
		}
		message := cause.Error()
		failed := Status{State: "failed", PublicURL: httpsOrNil(current), TargetURL: &target, Message: &message}
		_ = in.writeStatus(failed)
		_ = writeFailedTime(in, effects.now())
		var domain *domainError
		if errors.As(cause, &domain) {
			return domain
		}
		return domainErr(502, "domain_setup_failed", message)
	}
	if effects.Reload != nil {
		if err := effects.Reload(ctx); err != nil {
			return restore(domainErr(502, "domain_setup_failed", "HTTPS gateway refused the configuration; inspect gateway logs and retry"))
		}
	}
	if effects.Verify != nil {
		if err := effects.Verify(ctx, host, installationID); err != nil {
			return restore(domainErr(502, "https_not_ready", "HTTPS verification failed: "+host+" did not reach this installation with a trusted certificate. Check that its A/AAAA records point at this server, that no firewall or NAT blocks inbound ports 80 and 443, and the gateway logs for certificate errors. The previous address is kept; retry after the fix."))
		}
	}
	applying := Status{State: "applying", PublicURL: httpsOrNil(current), TargetURL: &target}
	if err := in.writeStatus(applying); err != nil {
		return restore(err)
	}
	if err := updateEnv(in.envPath(), map[string]string{"OAC_PUBLIC_URL": target, "OAC_WEB_BOOTSTRAP": "0"}); err != nil {
		return restore(err)
	}
	if effects.Recreate != nil {
		if err := effects.Recreate(ctx); err != nil {
			_ = updateEnv(in.envPath(), map[string]string{"OAC_PUBLIC_URL": current, "OAC_WEB_BOOTSTRAP": bootstrapFor(current)})
			return restore(domainErr(502, "domain_setup_failed", "Core and Web did not restart with the new address"))
		}
	}
	ready := Status{State: "ready", PublicURL: &target}
	return in.writeStatus(ready)
}

// configure runs acceptance and the certificate change together. Tests use it;
// the domain socket returns after acceptance and finishes in the background.
func configure(ctx context.Context, in installation, effects Effects, hostname, confirmation string) (Status, error) {
	job, err := acceptDomain(ctx, in, effects, hostname, confirmation)
	if err != nil {
		return Status{}, err
	}
	if err := finishDomain(ctx, in, effects, job); err != nil {
		return Status{}, err
	}
	return in.readStatus(), nil
}

func httpsOrNil(origin string) *string {
	if httpsHost(origin) == "" {
		return nil
	}
	value := origin
	return &value
}

func bootstrapFor(origin string) string {
	if strings.HasPrefix(origin, "https://") {
		return "0"
	}
	return "1"
}

func installationIdentity(data string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(data, "secrets", "core", "installation.id"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func failedTime(in installation) (time.Time, bool) {
	raw, err := os.ReadFile(filepath.Join(in.data, "domain", "failed-at"))
	if err != nil {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(string(raw)))
	return parsed, err == nil
}

func writeFailedTime(in installation, now time.Time) error {
	return writePrivate(filepath.Join(in.data, "domain", "failed-at"), []byte(now.UTC().Format(time.RFC3339)+"\n"))
}

func coreKey(data string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(data, "secrets", "web", "core.key"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func keyDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func authorized(r *http.Request, key string) bool {
	presented := r.Header.Get("Authorization")
	if len(r.Header.Values("Authorization")) != 1 {
		return false
	}
	return hmac.Equal([]byte(presented), []byte("Bearer "+key))
}

func liveEffects(data string, run Runner) Effects {
	return Effects{
		Resolve: func(host string) bool {
			_, err := net.LookupHost(host)
			return err == nil
		},
		Bindings: func(ctx context.Context) (int, error) {
			return addressBindings(ctx, data)
		},
		Reload: func(ctx context.Context) error {
			return reloadCaddy(ctx, filepath.Join(data, "caddy", "admin.sock"), "/etc/caddy/Caddyfile")
		},
		Verify: func(ctx context.Context, host, installationID string) error {
			return verifyHTTPS(ctx, host, installationID, verifyTimeout)
		},
		Recreate: func(ctx context.Context) error {
			return run.Run(ctx, "up", "-d", "--no-deps", "--wait", "core", "web")
		},
	}
}

func addressBindings(ctx context.Context, data string) (int, error) {
	key, err := coreKey(data)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://core:8091/core/v1/installation", nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode != 200 {
		return 0, errors.New("installation is unreachable")
	}
	var body struct {
		AddressBindings struct {
			NodesOnOtherAddress int `json:"nodes_on_other_address"`
			HostedSandboxes     int `json:"hosted_sandboxes"`
			SelfHostedExecutors int `json:"self_hosted_executors"`
		} `json:"address_bindings"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return 0, errors.New("installation report is invalid")
	}
	return body.AddressBindings.NodesOnOtherAddress + body.AddressBindings.HostedSandboxes + body.AddressBindings.SelfHostedExecutors, nil
}

func reloadCaddy(ctx context.Context, socket, caddyfile string) error {
	document, err := os.ReadFile(caddyfile)
	if err != nil {
		return err
	}
	dialer := &net.Dialer{}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/load", strings.NewReader(string(document)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "text/caddyfile")
	response, err := transport.RoundTrip(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<16))
	if response.StatusCode != 200 {
		return errors.New("gateway refused the configuration")
	}
	return nil
}

func verifyHTTPS(ctx context.Context, host, installationID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/_oac/installation/verify", nil)
		if err == nil {
			response, err := client.Do(request)
			if err == nil {
				body, _ := io.ReadAll(io.LimitReader(response.Body, 256))
				response.Body.Close()
				if response.StatusCode == 200 && string(body) == installationID {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return errors.New("https verification timed out")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
