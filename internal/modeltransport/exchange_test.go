package modeltransport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

var exchangeRequests = map[Protocol]string{
	ChatCompletions: `{"model":"client","max_tokens":123,"messages":[{"role":"user","content":"Read"},{"role":"assistant","tool_calls":[{"id":"call_a","type":"function","function":{"name":"read","arguments":"{\"path\":\"a\"}"}}]},{"role":"tool","tool_call_id":"call_a","content":"file"},{"role":"user","content":"Continue"}],"tools":[{"type":"function","function":{"name":"read","parameters":{"type":"object","properties":{"path":{"type":"string"}}}}}]}`,
	Anthropic:       `{"model":"client","max_tokens":123,"messages":[{"role":"user","content":"Read"},{"role":"assistant","content":[{"type":"tool_use","id":"call_a","name":"read","input":{"path":"a"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_a","content":"file"},{"type":"text","text":"Continue"}]}],"tools":[{"name":"read","input_schema":{"type":"object","properties":{"path":{"type":"string"}}}}]}`,
	Responses:       `{"model":"client","max_output_tokens":123,"input":[{"role":"user","content":"Read"},{"type":"function_call","call_id":"call_a","name":"read","arguments":"{\"path\":\"a\"}"},{"type":"function_call_output","call_id":"call_a","output":"file"},{"role":"user","content":"Continue"}],"tools":[{"type":"function","name":"read","parameters":{"type":"object","properties":{"path":{"type":"string"}}}}]}`,
}
var exchangeResponses = map[Protocol]string{
	ChatCompletions: `{"id":"chat_1","object":"chat.completion","model":"upstream","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`,
	Anthropic:       `{"id":"msg_1","type":"message","role":"assistant","model":"upstream","content":[{"type":"text","text":"Hello"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":7}}`,
	Responses:       `{"id":"resp_1","object":"response","model":"upstream","status":"completed","error":null,"output":[{"id":"m1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello","annotations":[]}]}],"usage":{"input_tokens":5,"output_tokens":7,"total_tokens":12}}`,
}

func protocols() []Protocol { return []Protocol{Anthropic, Responses, ChatCompletions} }
func TestExchangeRequestAndResponseMatrix(t *testing.T) {
	for _, source := range protocols() {
		for _, target := range protocols() {
			t.Run(string(source)+"_"+string(target), func(t *testing.T) {
				e, err := NewExchange(source, target, "exact/upstream-model", []byte(exchangeRequests[source]), false)
				if err != nil {
					t.Fatal(err)
				}
				r := gjson.ParseBytes(e.Request())
				if r.Get("model").String() != "exact/upstream-model" {
					t.Fatal("request identity or streaming changed")
				}
				if !strings.Contains(string(e.Request()), "call_a") || !strings.Contains(string(e.Request()), "read") || !strings.Contains(string(e.Request()), "file") {
					t.Fatal("tool history lost")
				}
				out, err := e.Response(context.Background(), responseBody(source, target))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(out), "Hello") {
					t.Fatalf("text lost: %s", out)
				}
				inputPath := "usage.input_tokens"
				outputPath := "usage.output_tokens"
				if source == ChatCompletions {
					inputPath = "usage.prompt_tokens"
					outputPath = "usage.completion_tokens"
				}
				if gjson.GetBytes(out, inputPath).Int() != 5 || gjson.GetBytes(out, outputPath).Int() != 7 {
					t.Fatalf("usage lost: %s", out)
				}
			})
		}
	}
}
func textEvents(p Protocol) [][]byte {
	var s []string
	switch p {
	case Anthropic:
		s = []string{
			`{"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"upstream","content":[],"usage":{"input_tokens":5,"output_tokens":0}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
			`{"type":"message_stop"}`}
	case ChatCompletions:
		s = []string{
			`{"id":"c1","model":"upstream","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"}}]}`,
			`{"id":"c1","model":"upstream","choices":[{"index":0,"delta":{"content":"lo"}}]}`,
			`{"id":"c1","model":"upstream","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`,
			"[DONE]"}
	case Responses:
		s = []string{
			`{"type":"response.created","response":{"id":"r1","model":"upstream","status":"in_progress","output":[]}}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"id":"m1","type":"message","role":"assistant","content":[]}}`,
			`{"type":"response.content_part.added","output_index":0,"item_id":"m1","content_index":0,"part":{"type":"output_text","text":""}}`,
			`{"type":"response.output_text.delta","output_index":0,"item_id":"m1","content_index":0,"delta":"Hel"}`,
			`{"type":"response.output_text.delta","output_index":0,"item_id":"m1","content_index":0,"delta":"lo"}`,
			`{"type":"response.output_text.done","output_index":0,"item_id":"m1","content_index":0,"text":"Hello"}`,
			`{"type":"response.output_item.done","output_index":0,"item":{"id":"m1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hello"}]}}`,
			`{"type":"response.completed","response":` + exchangeResponses[Responses] + `}`}
	}
	var out [][]byte
	for _, s := range s {
		out = append(out, []byte(s))
	}
	return out
}
func TestExchangeIncrementalStreamMatrix(t *testing.T) {
	for _, source := range protocols() {
		for _, target := range protocols() {
			t.Run(string(source)+"_"+string(target), func(t *testing.T) {
				e, err := NewExchange(source, target, "upstream", []byte(exchangeRequests[source]), true)
				if err != nil {
					t.Fatal(err)
				}
				chunks := textEvents(target)
				seenBeforeTerminal := false
				var result strings.Builder
				for i, c := range chunks {
					out, err := e.Event(context.Background(), c)
					if err != nil {
						t.Fatalf("event %d: %v", i, err)
					}
					for _, b := range out {
						if !strings.HasSuffix(string(b), "\n\n") {
							t.Fatal("unframed output")
						}
						result.Write(b)
					}
					if i < len(chunks)-1 && strings.Contains(result.String(), "Hel") {
						seenBeforeTerminal = true
					}
				}
				if !seenBeforeTerminal {
					t.Fatal("text buffered until terminal")
				}
				if err := e.Finish(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
func TestExchangeRejectsIncompleteAndError(t *testing.T) {
	for _, target := range protocols() {
		e, err := NewExchange(Responses, target, "upstream", []byte(exchangeRequests[Responses]), true)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = e.Event(context.Background(), textEvents(target)[0]); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(e.Finish(), ErrExchange) {
			t.Fatal("truncated stream accepted")
		}
	}
	e, _ := NewExchange(Responses, Anthropic, "upstream", []byte(exchangeRequests[Responses]), true)
	if _, err := e.Event(context.Background(), []byte(`{"type":"error","error":{"type":"overloaded_error","message":"private"}}`)); !errors.Is(err, ErrExchange) {
		t.Fatal("error accepted")
	}
	if _, err := e.Response(context.Background(), []byte(`{"error":{"message":"private"}}`)); !errors.Is(err, ErrExchange) {
		t.Fatal("HTTP error accepted")
	}
}

// These are upstream wire fixtures, not generated protocol content in production.
func responseBody(source, target Protocol) []byte {
	if source == target || target == ChatCompletions {
		return []byte(exchangeResponses[target])
	}
	return eventBody(textEvents(target))
}
func eventBody(events [][]byte) []byte {
	var out []byte
	for _, event := range events {
		out = append(out, frame(ChatCompletions, event)...)
	}
	return out
}
func TestExchangeCustomPatchTool(t *testing.T) {
	req := `{"model":"client","input":[{"role":"user","content":"Patch"},{"type":"custom_tool_call","call_id":"call_old","name":"apply_patch","input":"old patch"},{"type":"custom_tool_call_output","call_id":"call_old","output":"applied"}],"tools":[{"type":"custom","name":"apply_patch","format":{"type":"grammar","syntax":"lark","definition":"start: patch"}}]}`
	for _, target := range []Protocol{Anthropic, ChatCompletions} {
		for _, stream := range []bool{false, true} {
			e, err := NewExchange(Responses, target, "upstream", []byte(req), stream)
			if err != nil {
				t.Fatal(err)
			}
			request := gjson.ParseBytes(e.Request())
			schema := "tools.0.function.parameters.properties.input.type"
			name := "tools.0.function.name"
			if target == Anthropic {
				schema = "tools.0.input_schema.properties.input.type"
				name = "tools.0.name"
			}
			if request.Get(name).String() != "apply_patch" || request.Get(schema).String() != "string" ||
				!strings.Contains(string(e.Request()), "call_old") || !strings.Contains(string(e.Request()), "old patch") {
				t.Fatal("SDK custom declaration or history bridge lost")
			}
			chunks := toolEvents(target)
			for i, chunk := range chunks {
				value := strings.ReplaceAll(string(chunk), `"name":"read"`, `"name":"apply_patch"`)
				value = strings.ReplaceAll(value, "path", "input")
				chunks[i] = []byte(value)
			}
			var out []byte
			if stream {
				for _, chunk := range chunks {
					frames, err := e.Event(context.Background(), chunk)
					if err != nil {
						t.Fatal(err)
					}
					for _, f := range frames {
						out = append(out, f...)
					}
				}
				if e.Finish() != nil {
					t.Fatal("stream incomplete")
				}
			} else {
				body := eventBody(chunks)
				if target == ChatCompletions {
					body = []byte(`{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_a","type":"function","function":{"name":"apply_patch","arguments":"{\"input\":\"a\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`)
				}
				out, err = e.Response(context.Background(), body)
				if err != nil {
					t.Fatal(err)
				}
			}
			if !strings.Contains(string(out), `"name":"apply_patch"`) || !strings.Contains(string(out), "custom_tool_call") || !strings.Contains(string(out), `"input":"a"`) {
				t.Fatalf("SDK custom round trip lost: %s", out)
			}
		}
	}
}

func TestExchangeImageAndDelegatedParameters(t *testing.T) {
	req := `{"model":"client","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AA=="}}]}],"reasoning_effort":"medium","vendor_future_field":true}`
	e, err := NewExchange(ChatCompletions, Anthropic, "claude-sonnet-4-5", []byte(req), true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(e.Request()), "image") || !json.Valid(e.Request()) {
		t.Fatal("image lost")
	}
}
