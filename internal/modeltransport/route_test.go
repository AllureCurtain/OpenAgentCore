package modeltransport

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

func TestRouteQualification(t *testing.T) {
	requirements := []Requirements{{NativeConfig: true}, {StructuredOutput: true}, {ToolSearch: true}, {Images: true}, {ImageToolResults: true}, {WebSearch: true}, {NonDefaultVerbosity: true}}
	for _, upstream := range protocols() {
		for _, native := range protocols() {
			route, err := ResolveRoute(upstream, []Protocol{native})
			if err != nil {
				t.Fatal(err)
			}
			if route.Upstream != upstream || route.Native != native || route.Validate(Requirements{}) != nil {
				t.Fatal("route changed identity or rejected text")
			}
			for _, required := range requirements {
				err := route.Validate(required)
				if (err == nil) != (upstream == native) {
					t.Fatalf("unexpected qualification %s -> %s: %v", native, upstream, err)
				}
			}
		}
	}
	route, err := ResolveRoute(Responses, []Protocol{Anthropic, Responses, ChatCompletions})
	if err != nil || route.Converted() {
		t.Fatal("native route was not preferred")
	}
	if _, err = ResolveRoute("unknown", []Protocol{Anthropic}); err == nil {
		t.Fatal("unknown protocol accepted")
	}
}

func TestPreparationRejectsUnqualifiedFeaturesWithoutEndpoint(t *testing.T) {
	base := Provider{Protocol: ChatCompletions, BaseURL: "https://model.invalid/v1", APIKey: "private-sentinel"}
	requests := []proto.PromptRequestPayload{
		{AgentOptions: map[string]any{"harness_config": map[string]any{"effort": "high"}}},
		{ToolSearch: true},
		{ExecutionControls: &proto.ExecutionControls{OutputFormat: &proto.OutputFormat{Type: "json_schema"}}},
		{ExecutionControls: &proto.ExecutionControls{WebSearch: "live"}},
		{ExecutionControls: &proto.ExecutionControls{TextVerbosity: "high"}},
	}
	for _, request := range requests {
		if request.AgentOptions == nil {
			request.AgentOptions = map[string]any{}
		}
		request.AgentOptions["model_provider"] = base
		request.AgentOptions["model"] = "fixture"
		options, endpoint, err := PrepareRequestOptions(request, Anthropic)
		if err != ErrUnsupported || options != nil || endpoint != nil {
			t.Fatalf("unsupported preparation allocated a route: %v", err)
		}
	}
}

func TestProxyRejectsImagesAndAdvancedFeaturesBeforeUpstream(t *testing.T) {
	var calls atomic.Int32
	endpoint, _ := endpointFixture(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) })
	for _, body := range []string{
		`{"model":"chosen-model","messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"private-image"}}]}]}`,
		`{"model":"chosen-model","messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"call","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"private-image"}}]}]}]}`,
		`{"model":"chosen-model","messages":[],"output_config":{"format":{"type":"json_schema","schema":{"type":"object"}}}}`,
		`{"model":"chosen-model","messages":[],"tools":[{"type":"tool_search_tool_regex_20251119","name":"search"}]}`,
		`{"model":"chosen-model","messages":[],"tools":[{"type":"web_search_20250305","name":"search"}]}`,
	} {
		request, err := http.NewRequest(http.MethodPost, endpoint.BaseURL+"/v1/messages", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Api-Key", endpoint.APIKey)
		status, body := readEndpoint(t, request)
		if status != http.StatusBadRequest || strings.Contains(body, "private-image") {
			t.Fatalf("invalid rejection: %d %s", status, body)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("unqualified content reached upstream")
	}
}

func TestRequestImageGuardFollowsProtocolBlocks(t *testing.T) {
	cases := []struct {
		protocol Protocol
		raw      string
		image    bool
	}{
		{Responses, `{"input":[{"type":"message","content":[{"type":"input_image","image_url":"private"}]}]}`, true},
		{Responses, `{"input":[{"type":"function_call_output","output":[{"type":"input_image","image_url":"private"}]}]}`, true},
		{ChatCompletions, `{"messages":[{"role":"tool","content":[{"type":"image_url","image_url":{"url":"private"}}]}]}`, true},
		{Anthropic, `{"messages":[{"content":[{"type":"text","text":"input_image"}]}],"tools":[{"input_schema":{"type":"image"}}]}`, false},
	}
	for _, tc := range cases {
		if requestRequirements(tc.protocol, []byte(tc.raw)).Images != tc.image {
			t.Fatal("content guard confused tools/text and image blocks")
		}
	}
}
