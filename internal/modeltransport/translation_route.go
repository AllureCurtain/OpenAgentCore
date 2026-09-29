package modeltransport

import "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator/builtin"

func translationRoute(source, target Protocol) []Protocol {
	// The SDK's direct Responses->Claude route drops custom apply_patch; its Chat
	// route owns the freeform bridge and reverse namespace/custom mapping.
	if source == Responses && target == Anthropic {
		return []Protocol{source, ChatCompletions, target}
	}
	return []Protocol{source, target}
}
func translationAvailable(source, target Protocol) bool {
	registry := builtin.Registry()
	route := translationRoute(source, target)
	for i := 1; i < len(route); i++ {
		from, to := clientFormat(route[i-1]), providerFormat(route[i])
		if !registry.HasRequestTransformer(from, to) || !registry.HasStreamResponseTransformer(from, to) || !registry.HasNonStreamResponseTransformer(from, to) {
			return false
		}
	}
	return true
}
