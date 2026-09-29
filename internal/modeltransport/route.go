package modeltransport

import (
	"encoding/json"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

// Requirements describes requested behavior, not capabilities inferred from model IDs.
// Harness profiles still own native feature admission.
type Requirements struct {
	NativeConfig        bool
	StructuredOutput    bool
	ToolSearch          bool
	Images              bool
	ImageToolResults    bool
	WebSearch           bool
	NonDefaultVerbosity bool
}

// Route freezes logical protocol selection, never a proxy address or credential.
type Route struct {
	Upstream Protocol
	Native   Protocol
}

func ResolveRoute(upstream Protocol, native []Protocol) (Route, error) {
	if upstream.Path() == "" || len(native) == 0 {
		return Route{}, ErrConfiguration
	}
	for _, protocol := range native {
		if protocol.Path() == "" {
			return Route{}, ErrConfiguration
		}
	}
	for _, protocol := range native {
		if protocol == upstream {
			return Route{Upstream: upstream, Native: protocol}, nil
		}
	}
	route := Route{Upstream: upstream, Native: native[0]}
	if !translationAvailable(route.Native, route.Upstream) {
		return Route{}, ErrUnsupported
	}
	return route, nil
}
func (r Route) Converted() bool { return r.Native != r.Upstream }

// Native connections preserve semantics without asserting model support.
// Converted routes admit only the qualified text/function profile.
func (r Route) Validate(required Requirements) error {
	if r.Converted() && (required.NativeConfig || required.StructuredOutput || required.ToolSearch || required.Images || required.ImageToolResults || required.WebSearch || required.NonDefaultVerbosity) {
		return ErrUnsupported
	}
	return nil
}
func RequirementsFromOptions(options map[string]any) Requirements {
	required := Requirements{}
	if raw, present := options["harness_config"]; present {
		encoded, err := json.Marshal(raw)
		var object map[string]any
		required.NativeConfig = err != nil || json.Unmarshal(encoded, &object) != nil || object == nil || len(object) > 0
	}
	search, _ := options["web_search"].(string)
	verbosity, _ := options["model_verbosity"].(string)
	required.WebSearch = search != "" && search != "disabled"
	required.NonDefaultVerbosity = verbosity != "" && verbosity != "medium"
	return required
}

// RequirementsFromRequest uses the existing wire fields.
func RequirementsFromRequest(request proto.PromptRequestPayload) Requirements {
	required := RequirementsFromOptions(request.AgentOptions)
	required.ToolSearch = request.ToolSearch
	required.Images = request.Input.HasImages()
	if controls := request.ExecutionControls; controls != nil {
		required.StructuredOutput = controls.OutputFormat != nil
		required.WebSearch = controls.WebSearch != "" && controls.WebSearch != "disabled"
		required.NonDefaultVerbosity = controls.TextVerbosity != "" && controls.TextVerbosity != "medium"
	}
	return required
}
func (r Route) ValidateInput(input proto.MessageInput) error {
	return r.Validate(Requirements{Images: input.HasImages()})
}
func (r Route) ValidateFunctionResult(result proto.FunctionResultPayload) error {
	return r.Validate(Requirements{ImageToolResults: (proto.MessageInput{{Content: result.Content}}).HasImages()})
}
