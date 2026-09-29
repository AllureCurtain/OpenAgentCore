package modeltransport

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseProviderValidatesFrozenBundle(t *testing.T) {
	valid := map[string]any{"protocol": "responses", "base_url": "https://model.example/v1", "api_key": "fixture-upstream-key", "context_window": int32(64000), "max_output_tokens": int32(4096)}
	for _, protocol := range []Protocol{Anthropic, Responses, ChatCompletions} {
		t.Run(string(protocol), func(t *testing.T) {
			raw := copyBundle(valid)
			raw["protocol"] = string(protocol)
			got, err := ParseProvider(raw)
			want := Provider{Protocol: protocol, BaseURL: "https://model.example/v1", APIKey: "fixture-upstream-key", ContextWindow: 64000, MaxOutputTokens: 4096}
			if err != nil || got != want {
				t.Fatalf("bundle changed or rejected: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"unknown field", "native_options", map[string]any{}}, {"alias", "protocol", "openai"},
		{"missing key", "api_key", ""}, {"newline key", "api_key", "fixture\nkey"},
		{"URL credentials", "base_url", "https://user:secret@model.example/v1"},
		{"remote HTTP", "base_url", "http://model.example/v1"},
		{"query", "base_url", "https://model.example/v1?key=secret"},
		{"fragment", "base_url", "https://model.example/v1#secret"},
		{"negative limit", "context_window", -1}, {"excess output", "max_output_tokens", 64001},
		{"fractional limit", "context_window", 1.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := copyBundle(valid)
			raw[tc.field] = tc.value
			if _, err := ParseProvider(raw); !errors.Is(err, ErrConfiguration) {
				t.Fatalf("invalid bundle accepted: %v", err)
			}
		})
	}
	for _, raw := range []any{nil, []any{}, "provider", map[string]any{}} {
		if _, err := ParseProvider(raw); !errors.Is(err, ErrConfiguration) {
			t.Fatalf("invalid root accepted: %v", err)
		}
	}
}

func copyBundle(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		out[k] = v
	}
	return out
}

func TestPrepareSelectsNativeProtocolWithoutListener(t *testing.T) {
	for _, protocol := range []Protocol{Anthropic, Responses, ChatCompletions} {
		for _, native := range [][]Protocol{{protocol}, {Anthropic, Responses, ChatCompletions}} {
			p := Provider{Protocol: protocol, BaseURL: "https://model.example/v1", APIKey: "fixture-upstream-key"}
			endpoint, err := Prepare(p, "chosen-model", native...)
			if err != nil {
				t.Fatal(err)
			}
			if endpoint.Protocol != protocol || endpoint.BaseURL != p.BaseURL || endpoint.APIKey != p.APIKey || endpoint.server != nil || endpoint.transport != nil || endpoint.cancel != nil {
				t.Fatal("native protocol unnecessarily converted or upstream bundle changed")
			}
			if err := endpoint.Close(); err != nil {
				t.Fatal(err)
			}
			if err := endpoint.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPrepareRejectsInvalidSelection(t *testing.T) {
	p := Provider{Protocol: Responses, BaseURL: "https://model.example/v1", APIKey: "fixture-key"}
	for _, tc := range []struct {
		model  string
		native []Protocol
	}{{"", []Protocol{Responses}}, {"chosen-model", nil}, {"chosen-model", []Protocol{"unknown"}}} {
		if _, err := Prepare(p, tc.model, tc.native...); !errors.Is(err, ErrConfiguration) {
			t.Fatalf("invalid selection accepted: %v", err)
		}
	}
}

func TestPrepareOptionsPreservesCallerBundle(t *testing.T) {
	raw := map[string]any{"protocol": "chat_completions", "base_url": "http://127.0.0.1:4321/v1", "api_key": "fixture-upstream-key", "context_window": 64000, "max_output_tokens": 4096}
	original := copyBundle(raw)
	options := map[string]any{"model": "chosen-model", "model_provider": raw, "system_prompt": "unchanged"}
	mapped, endpoint, err := PrepareOptions(options, Anthropic)
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	p, err := ParseProvider(mapped["model_provider"])
	if err != nil || p.Protocol != Anthropic || p.BaseURL != endpoint.BaseURL || p.APIKey != endpoint.APIKey || p.ContextWindow != 64000 || p.MaxOutputTokens != 4096 {
		t.Fatal("prepared native bundle lost its provider limits")
	}
	if endpoint.APIKey == original["api_key"] || !reflect.DeepEqual(options["model_provider"], original) || mapped["system_prompt"] != "unchanged" {
		t.Fatal("provider preparation mutated caller configuration or reused upstream key")
	}
}
