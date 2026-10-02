package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigureRejectsBadHostnamesAndRequiresConfirmation(t *testing.T) {
	root := t.TempDir()
	in := installation{root: root, data: filepath.Join(root, "data")}
	writeInstall(t, in, "managed", "")
	effects := Effects{Resolve: func(string) bool { return true }, Bindings: func(context.Context) (int, error) { return 2, nil }}
	if _, err := configure(context.Background(), in, effects, "not a host", ""); err == nil {
		t.Fatal("accepted an invalid hostname")
	}
	_, err := configure(context.Background(), in, effects, "core.example", "")
	var domain *domainError
	if err == nil || !strings.Contains(err.Error(), "Confirm") {
		t.Fatal(err)
	}
	if !errorAs(err, &domain) || domain.Code != "public_url_confirmation_required" {
		t.Fatal(err)
	}
	if _, err := os.Stat(in.sitePath()); !os.IsNotExist(err) {
		t.Fatal("confirmation wrote a site before acceptance", err)
	}
}

func TestConfigureFailureRestoresThePreviousSiteAndSetsCooldown(t *testing.T) {
	root := t.TempDir()
	in := installation{root: root, data: filepath.Join(root, "data")}
	writeInstall(t, in, "managed", "")
	previous := []byte("\n")
	if err := os.MkdirAll(filepath.Dir(in.sitePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(in.sitePath(), previous, 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	effects := Effects{
		Now:     func() time.Time { return now },
		Resolve: func(string) bool { return true },
		Reload:  func(context.Context) error { return nil },
		Verify:  func(context.Context, string, string) error { return os.ErrDeadlineExceeded },
	}
	if _, err := configure(context.Background(), in, effects, "core.example", ""); err == nil || !strings.Contains(err.Error(), "HTTPS verification failed") {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(in.sitePath()); err != nil || string(got) != string(previous) {
		t.Fatalf("site = %q, err %v", got, err)
	}
	if _, err := configure(context.Background(), in, effects, "core.example", ""); err == nil || !strings.Contains(err.Error(), "Wait") {
		t.Fatal(err)
	}
}

func TestConfigureSuccessWritesThePublicURL(t *testing.T) {
	root := t.TempDir()
	in := installation{root: root, data: filepath.Join(root, "data")}
	writeInstall(t, in, "managed", "http://localhost:8080")
	effects := Effects{Resolve: func(string) bool { return true }, Reload: func(context.Context) error { return nil }, Verify: func(context.Context, string, string) error { return nil }, Recreate: func(context.Context) error { return nil }}
	status, err := configure(context.Background(), in, effects, "core.example", "")
	if err != nil {
		t.Fatal(err)
	}
	if status.PublicURL == nil || *status.PublicURL != "https://core.example" {
		t.Fatal(status)
	}
	if got, _ := envValue(in.envPath(), "OAC_PUBLIC_URL"); got != "https://core.example" {
		t.Fatal(got)
	}
	if got, _ := envValue(in.envPath(), "OAC_WEB_BOOTSTRAP"); got != "0" {
		t.Fatal(got)
	}
	site, err := os.ReadFile(in.sitePath())
	if err != nil || !strings.Contains(string(site), "https://core.example") || !strings.Contains(string(site), "11111111-1111-4111-8111-111111111111") {
		t.Fatalf("%s %v", site, err)
	}
}

func TestDomainCommandUsageNamesTheConfirmFlag(t *testing.T) {
	if err := domainCommand(installation{}, []string{"core.example", "--confirm"}); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatal(err)
	}
}

func TestApplyDoesNotStartWhenTheConfigurationCheckFails(t *testing.T) {
	var calls [][]string
	runner := scriptedRunner{run: func(args ...string) error {
		calls = append(calls, args)
		if args[0] == "run" {
			return os.ErrInvalid
		}
		return nil
	}}
	if err := apply(context.Background(), runner); err == nil {
		t.Fatal("apply continued")
	}
	if len(calls) != 1 || calls[0][0] != "run" {
		t.Fatal(calls)
	}
}

func TestRotateCoreKeyDigestDoesNotEchoTheKey(t *testing.T) {
	root := t.TempDir()
	in := installation{root: root, data: filepath.Join(root, "data")}
	if err := os.MkdirAll(filepath.Join(in.data, "secrets", "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(in.data, "secrets", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(in.data, "secrets", "web", "core.key"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(in.data, "secrets", "core", "core-key-digests.json"), []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var restarted bool
	runner := scriptedRunner{run: func(args ...string) error {
		restarted = args[0] == "restart"
		return nil
	}}
	if err := rotateCoreKey(context.Background(), in, runner); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(in.data, "secrets", "web", "core.key"))
	if err != nil {
		t.Fatal(err)
	}
	key := strings.TrimSpace(string(raw))
	digest, err := os.ReadFile(filepath.Join(in.data, "secrets", "core", "core-key-digests.json"))
	if err != nil || !strings.Contains(string(digest), keyDigest(key)) || strings.Contains(string(digest), key) {
		t.Fatalf("digest %s key leaked %v", digest, err)
	}
	if !restarted {
		t.Fatal("core was not restarted")
	}
}

func gatewayInstall(t *testing.T, proxy ...string) installation {
	t.Helper()
	root, err := os.MkdirTemp("", "oac-gw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	in := installation{root: root, data: filepath.Join(root, "data")}
	if err := os.MkdirAll(filepath.Join(in.data, "secrets", "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(in.data, "secrets", "web", "core.key"), []byte(strings.Repeat("k", 64)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := gatewayCaddy
	gatewayCaddy = proxy
	t.Cleanup(func() { gatewayCaddy = previous })
	return in
}

func TestGatewayStopsWhenTheProxyExits(t *testing.T) {
	in := gatewayInstall(t, "/bin/sh", "-c", "exit 3")
	done := make(chan error, 1)
	go func() { done <- serveGateway(t.Context(), in, Effects{}) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("gateway kept running without its proxy")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("gateway did not stop after its proxy exited")
	}
}

func TestGatewayStopsTheProxyOnShutdown(t *testing.T) {
	in := gatewayInstall(t, "/bin/sh", "-c", "trap 'exit 0' TERM; while :; do sleep 0.05; done")
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- serveGateway(ctx, in, Effects{}) }()
	socket := filepath.Join(in.data, "domain", "api.sock")
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("domain socket did not appear")
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("gateway did not stop its proxy")
	}
}

type scriptedRunner struct {
	run func(args ...string) error
}

func (s scriptedRunner) Run(_ context.Context, args ...string) error { return s.run(args...) }
func (s scriptedRunner) Output(context.Context, ...string) ([]byte, error) {
	return nil, nil
}

func writeInstall(t *testing.T, in installation, ingress, public string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(in.data, "secrets", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(in.data, "secrets", "core", "installation.id"), []byte("11111111-1111-4111-8111-111111111111\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := "OAC_INGRESS=" + ingress + "\nCOMPOSE_PROJECT_NAME=oac-test\n"
	if public != "" {
		env += "OAC_PUBLIC_URL=" + public + "\n"
	}
	if err := os.WriteFile(in.envPath(), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
}

func errorAs(err error, target **domainError) bool {
	if err == nil {
		return false
	}
	domain, ok := err.(*domainError)
	if ok {
		*target = domain
	}
	return ok
}
