package codex

import (
	"github.com/MiniMax-AI-Dev/parsar/internal/harnessconfig"
	"slices"
	"testing"
)

func TestHarnessConfigAppliedWithoutChangingProvider(t *testing.T) {
	t.Setenv("OAC_RUNTIME_HOME", t.TempDir())
	plan, err := BuildSessionPlan("run", "native-config", "", map[string]any{
		"model": "fixture", "harness_config": map[string]any{"model_reasoning_effort": "high"},
		"model_provider": map[string]any{"base_url": "https://provider.invalid/v1", "protocol": "responses", "api_key": "test-key"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Cleanup()
	if plan.Model != "fixture" || plan.ModelProvider != oacProviderSlug || !slices.Contains(plan.ExtraConfig, [2]string{"model_reasoning_effort", `"high"`}) {
		t.Fatalf("native configuration not applied: %+v", plan.ExtraConfig)
	}
}

func TestHarnessConfigConflictFailsBeforePreparation(t *testing.T) {
	_, err := BuildSessionPlan("run", "", "", map[string]any{"harness_config": map[string]any{"model_provider": "bypass"}})
	if err != harnessconfig.ErrHarnessConfig {
		t.Fatalf("configuration must fail before filesystem preparation: %v", err)
	}
}
