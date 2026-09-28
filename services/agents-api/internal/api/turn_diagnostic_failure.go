package api

import (
	"encoding/json"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
)

func turnDiagnosticFailure(turn store.Turn) *DiagnosticFailure {
	if turn.Status != store.TurnFailed {
		return nil
	}
	var outcome struct {
		Code string `json:"error_code"`
	}
	if json.Unmarshal(turn.Outcome, &outcome) != nil {
		outcome.Code = ""
	}
	code := "internal_error"
	switch outcome.Code {
	case "engine_failed":
		code = "harness_error"
	case "model_provider_required":
		code = "model_provider_required"
	case "execution_device_unavailable", "execution_unavailable":
		code = "runtime_unavailable"
	case "device_disconnected", "event_stream_incomplete":
		code = "runtime_disconnected"
	case "preparation_start_failed", "preparation_interrupted":
		code = "runtime_preparation_failed"
	case "execution_interrupted":
		code = "execution_interrupted"
	case "delivery_unknown", "input_outcome_unknown", "cancel_unconfirmed", "cancel_outcome_unavailable", "function_result_unconfirmed":
		code = "delivery_unconfirmed"
	case "invalid_input", "input_not_applied", "message_input_unsupported", "input_invalid_input", "input_run_inactive", "input_input_conflict", "input_input_limit", "input_unsupported", "input_rejected", "input_not_ready", "input_busy":
		code = "input_rejected"
	case "invalid_executor_result", "interaction_not_supported", "execution_state_unavailable", "execution_state_changed", "function_call_invalid", "function_result_invalid":
		code = "executor_protocol_error"
	case "event_persistence_failed", "artifact_capture_failed":
		code = "core_storage_failed"
	}
	return &DiagnosticFailure{Code: code, Params: CoreErrorDetails{}, FailedAt: diagnosticTime(turn.CompletedAt)}
}
