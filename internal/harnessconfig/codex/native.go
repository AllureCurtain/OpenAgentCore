package codex

import "github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"

func validateNativeConfig(config proto.HarnessConfig) bool {
	for key, value := range config {
		if key != "model_reasoning_effort" {
			return false
		}
		text, ok := value.(string)
		if !ok {
			return false
		}
		switch text {
		case "none", "minimal", "low", "medium", "high", "xhigh":
		default:
			return false
		}
	}
	return true
}
