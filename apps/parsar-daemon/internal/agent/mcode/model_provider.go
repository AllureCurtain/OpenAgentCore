package mcode

import (
	"fmt"

	"github.com/MiniMax-AI-Dev/parsar/internal/modeltransport"
)

func modelProviderConfig(raw any, model string) (map[string]any, error) {
	provider, err := modeltransport.ParseProvider(raw)
	if err != nil {
		return nil, fmt.Errorf("mcode: %w", err)
	}
	if provider.ContextWindow <= 0 || provider.MaxOutputTokens <= 0 {
		return nil, fmt.Errorf("mcode: positive model context_window and max_output_tokens are required")
	}
	var api string
	switch provider.Protocol {
	case modeltransport.Anthropic:
		api = "anthropic-messages"
	case modeltransport.ChatCompletions:
		api = "openai-completions"
	case modeltransport.Responses:
		api = "openai-responses"
	}
	return map[string]any{
		"name": "Configured provider", "kind": "custom", "enabled": true, "api": api,
		"options": map[string]any{"baseURL": provider.BaseURL, "apiKey": provider.APIKey},
		"models": map[string]any{model: map[string]any{
			"name": model, "tool_call": true,
			"limit": map[string]any{"context": provider.ContextWindow, "output": provider.MaxOutputTokens},
		}},
	}, nil
}
