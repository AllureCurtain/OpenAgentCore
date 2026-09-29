package v1

import (
	"encoding/json"
	"fmt"
)

// AgentsCore selects an existing Core harness independently of model identity.
type AgentsCore struct {
	Harness       string          `json:"harness,omitempty" enums:"codex,claude_sdk,mcode"`
	HarnessConfig json.RawMessage `json:"harness_config,omitempty" swaggertype:"object"`
}

func (x *AgentsCore) Validate() error {
	if x == nil {
		return nil
	}
	if x.Harness == "" && len(x.HarnessConfig) == 0 {
		return fmt.Errorf("x_agents_core requires harness or harness_config")
	}
	switch x.Harness {
	case "", "codex", "claude_sdk", "mcode":
		return ValidateHarnessConfig(x.Harness, x.HarnessConfig)
	default:
		return fmt.Errorf("x_agents_core.harness must be codex, claude_sdk or mcode")
	}
}
