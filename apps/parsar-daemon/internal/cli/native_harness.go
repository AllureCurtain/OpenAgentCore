package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/claudesdk"
	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/clirunner"
)

func nativeExe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
func nativeNode(root string) string {
	base := nativeComponentRoot(root, "node")
	if runtime.GOOS != "windows" {
		base = filepath.Join(base, "bin")
	}
	return filepath.Join(base, nativeExe("node"))
}

func nativeHarnessEnvironment(root string, selected []string) map[string]string {
	node := nativeNode(root)
	values := map[string]string{"PATH": filepath.Dir(node) + string(os.PathListSeparator) + os.Getenv("PATH")}
	for _, name := range selected {
		dir := nativeComponentRoot(root, name)
		switch name {
		case "codex":
			values["OAC_RUNTIME_CODEX_BIN"] = filepath.Join(dir, "bin", nativeExe("codex"))
		case "claude":
			values[claudeSDKEntrypointEnv] = filepath.Join(dir, "dist", "main.js")
			values[claudeSDKNodeEnv] = node
		case "minimax":
			values["OAC_RUNTIME_MCODE_BIN"] = filepath.Join(dir, "native", "cli.js")
			values["OAC_RUNTIME_MCODE_NODE"] = node
			values["OAC_RUNTIME_MCODE_WORKSPACE_BRIDGE"] = filepath.Join(dir, "launch.mjs")
			values["OAC_RUNTIME_MCODE_AGENTS_API"] = "1"
			values["PATH"] = filepath.Join(dir, "bin") + string(os.PathListSeparator) + values["PATH"]
		}
	}
	return values
}

func withNativeEnv(values map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		// Windows environment names are case-insensitive.
		replaced := false
		for k := range values {
			if strings.EqualFold(k, key) {
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, entry)
		}
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

// Native diagnostics are deliberately discarded: dependencies may echo their
// environment. Readiness is separate from model credentials and a live Turn.
func nativeProbeCommand(binary string, args, env []string, dir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	p, err := clirunner.Start(clirunner.StartOptions{Parent: ctx, Binary: binary, Args: args, Env: env, Dir: dir, OwnProcessGroup: true, KillTimeout: 250 * time.Millisecond})
	if err != nil {
		return "", errors.New("native component failed to start")
	}
	defer p.Cancel()
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, p.Stderr); close(done) }()
	raw, err := io.ReadAll(io.LimitReader(p.Stdout, 64*1024+1))
	if err != nil || len(raw) > 64*1024 {
		p.Cancel()
	}
	_, _ = io.Copy(io.Discard, p.Stdout)
	<-done
	waitErr := p.Wait()
	if err != nil || waitErr != nil || ctx.Err() != nil || len(raw) > 64*1024 {
		return "", errors.New("native component compatibility check failed")
	}
	return strings.TrimSpace(string(raw)), nil
}

var probeNativeInstallation = checkNativeInstallation

func checkNativeInstallation(root string, selected []string) error {
	values := nativeHarnessEnvironment(root, selected)
	env := withNativeEnv(values)
	node := nativeNode(root)
	version, err := nativeProbeCommand(node, []string{"--version"}, env, root)
	if err != nil || version != "v"+nativePins["node"] {
		return errors.New("install: bundled Node is unavailable or incompatible; use the distribution for this operating system and architecture")
	}
	for _, name := range selected {
		dir := nativeComponentRoot(root, name)
		switch name {
		case "codex":
			version, err = nativeProbeCommand(values["OAC_RUNTIME_CODEX_BIN"], []string{"--version"}, env, root)
			if err == nil && version != "codex-cli "+nativePins[name] {
				err = errors.New("version mismatch")
			}
		case "claude":
			var info claudesdk.RuntimeInfo
			info, err = claudesdk.CheckRuntime(context.Background(), claudesdk.Config{Node: node, Entrypoint: values[claudeSDKEntrypointEnv], Env: env})
			if err == nil && (info.SDK != nativePins[name] || !info.SupportsLocalRuntime()) {
				err = errors.New("adapter contract mismatch")
			}
		case "minimax":
			version, err = nativeProbeCommand(node, []string{filepath.Join(dir, "native", "cli.js"), "--version"}, env, root)
			if err == nil && version != nativePins[name] {
				err = errors.New("version mismatch")
			}
			if err == nil {
				_, err = nativeProbeCommand(node, []string{filepath.Join(dir, "check.mjs")}, env, root)
			}
		}
		if err != nil {
			return fmt.Errorf("install: %s compatibility check failed; verify OS dependencies (Claude on Windows needs Git Bash; MiniMax needs Bash) and use a matching release; no files were replaced", name)
		}
	}
	return nil
}
