package codex

import (
	"os"
	"runtime"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/localworkspace"
)

// SupportsLocalEnvironment checks deployment prerequisites, not public admission.
func SupportsLocalEnvironment(version string) bool {
	if !SupportsNativeSessionRecovery(version) || os.Getenv("OAC_RUNTIME_CODEX_HARNESS_BIN") != "" || (runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows") {
		return false
	}
	binding, err := localworkspace.Load()
	return err == nil && binding != nil
}
