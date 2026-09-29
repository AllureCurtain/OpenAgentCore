package builtin

import (
	"testing"

	"github.com/MiniMax-AI-Dev/parsar/internal/modeltransport"
)

func TestDeclaredNativeRoutesAndConfigurationSupport(t *testing.T) {
	for _, kind := range []string{"codex", "claude_sdk", "mcode"} {
		config, ok := Registry().Lookup(kind)
		if !ok {
			t.Fatal("missing declaration")
		}
		if config.AcceptsHarnessConfig() != (kind != "mcode") {
			t.Fatal("native configuration support not derived from validator")
		}
		for _, provider := range config.Providers {
			native := false
			for _, protocol := range config.NativeProtocols() {
				native = native || string(protocol) == provider.Protocol
			}
			if native != provider.Native {
				t.Fatal("route source drift")
			}
			err := config.ValidateRoute(provider.Protocol, modeltransport.Requirements{StructuredOutput: true})
			if (err == nil) != native {
				t.Fatal("declaration and route admission differ")
			}
		}
	}
}
