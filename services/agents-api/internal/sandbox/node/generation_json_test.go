package node

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/google/uuid"
)

func TestGenerationGrantJSONRefusesAmbiguousDeletionAuthority(t *testing.T) {
	f := frame{Version: 2, Type: "retention_ack", Deployment: &sandbox.NodeDeployment{Generation: 2, SpecificationDigest: strings.Repeat("a", 64)}, Control: &generationControl{ID: uuid.NewString(), ConnectionID: uuid.NewString(), Sequence: 1, OwnerEpoch: 1, Retentions: []sandbox.GenerationRetention{{GenerationReference: sandbox.GenerationReference{Generation: 1, SpecificationDigest: strings.Repeat("b", 64)}, Keep: true}}}}
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	original := string(raw)
	if _, err := decodeFrame(raw); err != nil {
		t.Fatal("valid grant rejected", err)
	}
	for name, replacement := range map[string]string{
		"duplicate":             strings.Replace(original, `"keep":true`, `"keep":true,"keep":false`, 1),
		"case alias":            strings.Replace(original, `"keep":true`, `"Keep":false`, 1),
		"null grant":            strings.Replace(original, `"keep":true`, `"keep":null`, 1),
		"missing grant":         strings.Replace(original, `,"keep":true`, ``, 1),
		"foreign null":          strings.TrimSuffix(original, "}") + `,"request":null}`,
		"mixed null control":    strings.Replace(original, `"retentions":`, `"references":null,"retentions":`, 1),
		"null serving omission": strings.Replace(original, `,"serving_generation":null`, ``, 1),
		"duplicate envelope":    strings.TrimSuffix(original, "}") + `,"version":2}`,
		"sequence overflow":     strings.Replace(original, `"sequence":1`, `"sequence":9223372036854775808`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if replacement == original {
				t.Fatal("fixture did not change")
			}
			if _, err := decodeFrame([]byte(replacement)); err == nil {
				t.Fatal("ambiguous grant accepted")
			}
		})
	}
}

func TestGenerationHealthRejectsUnboundedOrAmbiguousNumbers(t *testing.T) {
	f := frame{Version: 2, Type: "heartbeat", ConnectionID: uuid.NewString(), OwnerEpoch: 1, Health: &Health{ObservedAt: time.Now().UTC()}}
	good, _ := json.Marshal(f)
	if _, err := decodeFrame(good); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []struct{ from, to string }{
		{`"active_operations":0`, `"active_operations":-1`},
		{`"active_operations":0`, `"active_operations":33`},
		{`"cpu_utilization":null`, `"cpu_utilization":1.1`},
		{`"total_memory_bytes":null`, `"total_memory_bytes":9007199254740992`},
		{`"effective_cpu_cores":null`, `"effective_cpu_cores":0`},
		{`"provider_ready":false`, `"Provider_Ready":false`},
	} {
		if _, err := decodeFrame([]byte(strings.Replace(string(good), replacement.from, replacement.to, 1))); err == nil {
			t.Fatal("invalid health accepted", replacement.to)
		}
	}
}
