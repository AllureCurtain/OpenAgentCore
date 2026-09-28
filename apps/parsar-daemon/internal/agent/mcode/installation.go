package mcode

import (
	"fmt"
	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent"
	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/installroot"
	"path/filepath"
	"runtime"
)

func Installation() agent.Installation {
	return agent.Installation{Version: SupportedVersion, Supported: func() bool { return runtime.GOOS == "linux" || runtime.GOOS == "darwin" },
		Environment: func(dir, node string) map[string]string {
			return map[string]string{"OAC_RUNTIME_MCODE_BIN": filepath.Join(dir, "native", "cli.js"), "OAC_RUNTIME_MCODE_NODE": node, "OAC_RUNTIME_MCODE_WORKSPACE_BRIDGE": filepath.Join(dir, "launch.mjs"), "OAC_RUNTIME_MCODE_AGENTS_API": "1"}
		},
		Check: func(dir, node string, env []string) error {
			got, err := installroot.Probe(node, []string{filepath.Join(dir, "native", "cli.js"), "--version"}, env, dir)
			if err != nil || got != SupportedVersion {
				return fmt.Errorf("MiniMax installation is incompatible")
			}
			if _, err = installroot.Probe(node, []string{filepath.Join(dir, "check.mjs")}, env, dir); err != nil {
				return fmt.Errorf("MiniMax dependencies are unavailable; Bash is required")
			}
			return nil
		}}
}
