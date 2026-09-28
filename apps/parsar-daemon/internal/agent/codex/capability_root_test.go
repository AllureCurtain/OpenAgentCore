package codex

import (
	"testing"
)

func TestCapabilityRootIsOneQuotedReadOnlyNativeKey(t *testing.T) {
	var plan SessionPlan
	configureCapabilityRoot(&plan, "managed-workspace-enabled", "/tmp/root.with.dot/quote\"inside")
	if len(plan.ExtraConfig) != 1 || plan.ExtraConfig[0][0] != "permissions.\"managed-workspace-enabled\".filesystem.\"/tmp/root.with.dot/quote\\\"inside\"" || plan.ExtraConfig[0][1] != "\"read\"" {
		t.Fatal("root escaped its read-only key", plan.ExtraConfig)
	}
}
