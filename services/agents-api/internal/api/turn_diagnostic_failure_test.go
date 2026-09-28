package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
)

func TestDiagnosticFailureWhitelist(t *testing.T) {
	cases := map[string][]string{
		"harness_error": {"engine_failed"}, "model_provider_required": {"model_provider_required"}, "runtime_unavailable": {"execution_device_unavailable", "execution_unavailable"}, "runtime_disconnected": {"device_disconnected", "event_stream_incomplete"}, "runtime_preparation_failed": {"preparation_start_failed", "preparation_interrupted"}, "execution_interrupted": {"execution_interrupted"},
		"delivery_unconfirmed":    {"delivery_unknown", "input_outcome_unknown", "cancel_unconfirmed", "cancel_outcome_unavailable", "function_result_unconfirmed"},
		"input_rejected":          {"invalid_input", "input_not_applied", "message_input_unsupported", "input_invalid_input", "input_run_inactive", "input_input_conflict", "input_input_limit", "input_unsupported", "input_rejected", "input_not_ready", "input_busy"},
		"executor_protocol_error": {"invalid_executor_result", "interaction_not_supported", "execution_state_unavailable", "execution_state_changed", "function_call_invalid", "function_result_invalid"}, "core_storage_failed": {"event_persistence_failed", "artifact_capture_failed"}, "internal_error": {"secret-canary", "input_secret-canary", "execution_state_secret-canary", ""},
	}
	catalog, err := os.ReadFile("../../../../contracts/agents-api/core-errors.md")
	if err != nil {
		t.Fatal(err)
	}
	for want, inputs := range cases {
		if !strings.Contains(string(catalog), "`"+want+"`") {
			t.Fatal("uncatalogued diagnostics code", want)
		}
		for _, input := range inputs {
			raw, _ := json.Marshal(map[string]string{"error_code": input, "error": "raw-secret-canary"})
			got := turnDiagnosticFailure(store.Turn{Status: store.TurnFailed, Outcome: raw})
			encoded, _ := json.Marshal(got)
			if got.Code != want || strings.Contains(string(encoded), "canary") {
				t.Fatal(input, got)
			}
		}
	}
	if got := turnDiagnosticFailure(store.Turn{Status: store.TurnFailed, Outcome: json.RawMessage(`{"error_code":"engine_failed",`)}); got.Code != "internal_error" {
		t.Fatal("malformed outcome accepted", got)
	}
	for _, status := range []string{store.TurnQueued, store.TurnInProgress, store.TurnWaiting, store.TurnCompleted, store.TurnCancelled} {
		if got := turnDiagnosticFailure(store.Turn{Status: status, Outcome: json.RawMessage(`{"error_code":"engine_failed"}`)}); got != nil {
			t.Fatal("nonfailure classified", status, got)
		}
	}
}
