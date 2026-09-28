package codex

import (
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"testing"
)

func TestRuntimeUsesHostPermissions(t *testing.T) {
	t.Setenv("OAC_RUNTIME_HOME", t.TempDir())
	for _, mode := range []string{"", "native"} {
		req := proto.PromptRequestPayload{AgentStateKey: "session", DisableSubagents: true, LocalEnvironment: &proto.LocalEnvironment{ExecutionMode: mode, NetworkAccess: "enabled"}}
		plan, _, err := prepareSessionPlan(t.Context(), req, sessionConfig{})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Sandbox != "danger-full-access" || plan.Permissions != "" || plan.ApprovalPolicy.String != "never" {
			t.Fatal("Runtime must bypass inner sandbox", plan)
		}
		for _, kv := range plan.ExtraConfig {
			if kv[0] == "default_permissions" {
				t.Fatal("obsolete permission wrapper", kv)
			}
		}
		plan.Cleanup()
		for _, network := range []string{"disabled", "restricted"} {
			req.LocalEnvironment.NetworkAccess = network
			if _, _, err := prepareSessionPlan(t.Context(), req, sessionConfig{}); err == nil {
				t.Fatal("unsupported network admitted")
			}
		}
	}
}
