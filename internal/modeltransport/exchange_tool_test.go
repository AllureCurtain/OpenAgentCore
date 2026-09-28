package modeltransport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func toolEvents(p Protocol) [][]byte {
	var values []string
	switch p {
	case Anthropic:
		values = []string{
			`{"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"upstream","content":[],"usage":{"input_tokens":5,"output_tokens":0}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_a","name":"read","input":{}}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"a\"}"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":7}}`,
			`{"type":"message_stop"}`}
	case ChatCompletions:
		values = []string{
			`{"id":"c1","model":"upstream","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"read","arguments":"{\"path\":"}}]}}]}`,
			`{"id":"c1","model":"upstream","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"a\"}"}}]}}]}`,
			`{"id":"c1","model":"upstream","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`,
			"[DONE]"}
	case Responses:
		values = []string{
			`{"type":"response.created","response":{"id":"r1","model":"upstream","status":"in_progress","output":[]}}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc1","type":"function_call","call_id":"call_a","name":"read","arguments":""}}`,
			`{"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc1","delta":"{\"path\":"}`,
			`{"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc1","delta":"\"a\"}"}`,
			`{"type":"response.function_call_arguments.done","output_index":0,"item_id":"fc1","arguments":"{\"path\":\"a\"}"}`,
			`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc1","type":"function_call","call_id":"call_a","name":"read","arguments":"{\"path\":\"a\"}"}}`,
			`{"type":"response.completed","response":{"id":"r1","object":"response","model":"upstream","status":"completed","output":[{"id":"fc1","type":"function_call","call_id":"call_a","name":"read","arguments":"{\"path\":\"a\"}"}],"usage":{"input_tokens":5,"output_tokens":7,"total_tokens":12}}}`}
	}
	out := make([][]byte, len(values))
	for i, v := range values {
		out[i] = []byte(v)
	}
	return out
}
func TestExchangeToolStreamMatrix(t *testing.T) {
	for _, source := range protocols() {
		for _, target := range protocols() {
			t.Run(string(source)+"_"+string(target), func(t *testing.T) {
				e, err := NewExchange(source, target, "upstream", []byte(exchangeRequests[source]), true)
				if err != nil {
					t.Fatal(err)
				}
				var argument strings.Builder
				var terminal, identity, usage bool
				for _, event := range toolEvents(target) {
					out, err := e.Event(context.Background(), event)
					if err != nil {
						t.Fatal(err)
					}
					for _, frame := range out {
						for _, line := range strings.Split(string(frame), "\n") {
							if !strings.HasPrefix(line, "data:") {
								continue
							}
							data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
							if data == "[DONE]" {
								terminal = source == ChatCompletions
								continue
							}
							r := gjson.Parse(data)
							if strings.Contains(data, "call_a") {
								identity = true
							}
							switch source {
							case Anthropic:
								if r.Get("type").String() == "content_block_delta" {
									argument.WriteString(r.Get("delta.partial_json").String())
								}
								if r.Get("type").String() == "message_stop" {
									terminal = true
								}
								if r.Get("usage.output_tokens").Int() == 7 {
									usage = true
								}
							case ChatCompletions:
								for _, c := range r.Get("choices.0.delta.tool_calls").Array() {
									argument.WriteString(c.Get("function.arguments").String())
								}
								if r.Get("usage.completion_tokens").Int() == 7 {
									usage = true
								}
							case Responses:
								if r.Get("type").String() == "response.function_call_arguments.delta" {
									argument.WriteString(r.Get("delta").String())
								}
								if r.Get("type").String() == "response.completed" {
									terminal = true
								}
								if r.Get("response.usage.output_tokens").Int() == 7 {
									usage = true
								}
							}
						}
					}
				}
				if err := e.Finish(); err != nil {
					t.Fatal(err)
				}
				if !terminal || !identity || !usage {
					t.Fatalf("missing terminal=%v identity=%v usage=%v", terminal, identity, usage)
				}
				if !json.Valid([]byte(argument.String())) || gjson.Get(argument.String(), "path").String() != "a" {
					t.Fatalf("tool arguments lost: %q", argument.String())
				}
			})
		}
	}
}

// This deterministic fixture keeps the shape of Codex 0.153.4's request without
// retaining captured instructions, prompts, client IDs, or local paths.
const codexNamespaceRequest = `{
 "model":"client","input":[
  {"type":"message","role":"user","content":[{"type":"input_text","text":"Read a skill."}]},
  {"type":"function_call","call_id":"call_old","namespace":"skills","name":"list","arguments":"{}"},
  {"type":"function_call_output","call_id":"call_old","output":"fixture"},
  {"type":"message","role":"user","content":[{"type":"input_text","text":"Continue."}]}
 ],
 "instructions":"Fixture only.","stream":true,"store":false,
 "client_metadata":{"fixture_id":"not-a-real-client"},
 "prompt_cache_key":"fixture-cache","include":["reasoning.encrypted_content"],
 "reasoning":{"summary":"auto"},"parallel_tool_calls":true,"tool_choice":"auto",
 "tools":[
  {"type":"function","name":"lookup","parameters":{"type":"object","properties":{}}},
  {"type":"namespace","name":"skills","description":"Fixture skills","tools":[
   {"type":"function","name":"list","description":"List fixture skills","parameters":{"type":"object","properties":{}}},
   {"type":"function","name":"read","description":"Read fixture skill","parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}}
  ]}
 ]}`

func TestExchangeCodexNamespaceRoundTrip(t *testing.T) {
	for _, target := range []Protocol{ChatCompletions, Anthropic} {
		t.Run(string(target), func(t *testing.T) {
			e, err := NewExchange(Responses, target, "upstream", []byte(codexNamespaceRequest), true)
			if err != nil {
				t.Fatal(err)
			}
			request := gjson.ParseBytes(e.Request())
			if len(request.Get("tools").Array()) != 3 {
				t.Fatal("namespace leaves lost")
			}
			for _, name := range []string{"lookup", "skills__list", "skills__read", "call_old"} {
				if !strings.Contains(string(e.Request()), name) {
					t.Fatal("SDK namespace or history lost")
				}
			}
			var completed gjson.Result
			for _, event := range toolEvents(target) {
				event = []byte(strings.ReplaceAll(string(event), `"name":"read"`, `"name":"skills__read"`))
				frames, err := e.Event(context.Background(), event)
				if err != nil {
					t.Fatal(err)
				}
				for _, frame := range frames {
					for _, line := range strings.Split(string(frame), "\n") {
						if !strings.HasPrefix(line, "data:") {
							continue
						}
						data := gjson.Parse(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
						if data.Get("type").String() == "response.completed" {
							completed = data.Get("response")
						}
						if item := data.Get("item"); item.Get("type").String() == "function_call" {
							if item.Get("namespace").String() != "skills" || item.Get("name").String() != "read" {
								t.Fatal("streamed namespace identity lost")
							}
						}
					}
				}
			}
			if err := e.Finish(); err != nil {
				t.Fatal(err)
			}
			call := completed.Get("output.0")
			if call.Get("namespace").String() != "skills" || call.Get("name").String() != "read" || call.Get("call_id").String() != "call_a" {
				t.Fatal("completed namespace identity lost")
			}

			e, err = NewExchange(Responses, target, "upstream", []byte(codexNamespaceRequest), false)
			if err != nil {
				t.Fatal(err)
			}
			response := []byte(`{"id":"fixture","model":"upstream","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_a","type":"function","function":{"name":"skills__read","arguments":"{\"path\":\"a\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`)
			if target == Anthropic {
				chunks := toolEvents(target)
				for i, chunk := range chunks {
					chunks[i] = []byte(strings.ReplaceAll(string(chunk), `"name":"read"`, `"name":"skills__read"`))
				}
				response = eventBody(chunks)
			}
			out, err := e.Response(context.Background(), []byte(response))
			if err != nil {
				t.Fatal(err)
			}
			call = gjson.GetBytes(out, "output.0")
			if call.Get("namespace").String() != "skills" || call.Get("name").String() != "read" {
				t.Fatal("nonstream namespace identity lost")
			}
		})
	}
}

func TestExchangeSDKVisibleReasoning(t *testing.T) {
	for _, target := range []Protocol{ChatCompletions, Anthropic} {
		e, err := NewExchange(Responses, target, "upstream", []byte(`{"input":"Hello","reasoning":{"summary":"auto"}}`), false)
		if err != nil {
			t.Fatal(err)
		}
		response := []byte(`{"id":"fixture","model":"upstream","choices":[{"message":{"role":"assistant","content":"Answer","reasoning_content":"Visible fixture reasoning"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`)
		if target == Anthropic {
			response = eventBody([][]byte{
				[]byte(`{"type":"message_start","message":{"id":"fixture","type":"message","role":"assistant","model":"upstream","content":[],"usage":{"input_tokens":5,"output_tokens":0}}}`),
				[]byte(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`),
				[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Visible fixture reasoning"}}`),
				[]byte(`{"type":"content_block_stop","index":0}`),
				[]byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`),
				[]byte(`{"type":"message_stop"}`),
			})
		}
		out, err := e.Response(context.Background(), response)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range gjson.GetBytes(out, "output").Array() {
			if item.Get("type").String() == "reasoning" {
				for _, part := range item.Get("summary").Array() {
					found = found || part.Get("text").String() == "Visible fixture reasoning"
				}
			}
		}
		if !found {
			t.Fatal("SDK lost visible reasoning")
		}
	}
}
