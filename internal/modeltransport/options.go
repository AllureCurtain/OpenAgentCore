package modeltransport

import (
	"maps"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

// PrepareOptions replaces only the confidential provider bundle in a copied
// execution option set. The caller retains and closes the returned endpoint.
func PrepareOptions(options map[string]any, native ...Protocol) (map[string]any, *Endpoint, error) {
	return prepareOptions(options, RequirementsFromOptions(options), native...)
}
func PrepareRequestOptions(request proto.PromptRequestPayload, native ...Protocol) (map[string]any, *Endpoint, error) {
	return prepareOptions(request.AgentOptions, RequirementsFromRequest(request), native...)
}
func prepareOptions(options map[string]any, required Requirements, native ...Protocol) (map[string]any, *Endpoint, error) {
	raw, present := options["model_provider"]
	if !present {
		return options, nil, nil
	}
	provider, err := ParseProvider(raw)
	if err != nil {
		return nil, nil, err
	}
	route, err := ResolveRoute(provider.Protocol, native)
	if err != nil {
		return nil, nil, err
	}
	if err = route.Validate(required); err != nil {
		return nil, nil, err
	}
	model, _ := options["model"].(string)
	endpoint, err := Prepare(provider, model, native...)
	if err != nil {
		return nil, nil, err
	}
	provider.Protocol, provider.BaseURL, provider.APIKey = endpoint.Protocol, endpoint.BaseURL, endpoint.APIKey
	copied := maps.Clone(options)
	copied["model_provider"] = provider
	return copied, endpoint, nil
}
