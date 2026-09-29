package claudesdk

import (
	"strings"

	"github.com/MiniMax-AI-Dev/parsar/internal/modeltransport"
)

// The factory prepares the native endpoint before producing launch variables.
// Upstream credentials remain in Runtime when translation is required.
func providerEnvironment(value any) ([]string, error) {
	provider, err := modeltransport.ParseProvider(value)
	if err != nil || provider.Protocol != modeltransport.Anthropic {
		return nil, modeltransport.ErrConfiguration
	}
	return []string{"ANTHROPIC_BASE_URL=" + provider.BaseURL, "ANTHROPIC_AUTH_TOKEN=" + provider.APIKey}, nil
}

func withProvider(env, provider []string) []string {
	if provider == nil {
		return env
	}
	out := make([]string, 0, len(env)+len(provider))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if key != "ANTHROPIC_API_KEY" && key != "ANTHROPIC_AUTH_TOKEN" && key != "ANTHROPIC_BASE_URL" {
			out = append(out, entry)
		}
	}
	return append(out, provider...)
}
