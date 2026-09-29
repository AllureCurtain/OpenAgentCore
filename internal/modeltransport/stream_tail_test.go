package modeltransport

import (
	"bytes"
	"github.com/tidwall/gjson"
	"testing"
)

func TestChatUsageTailCompletesWithoutSentinel(t *testing.T) {
	for _, source := range []Protocol{Anthropic, Responses} {
		t.Run(string(source), func(t *testing.T) {
			e, err := NewExchange(source, ChatCompletions, "upstream", []byte(exchangeRequests[source]), true)
			if err != nil {
				t.Fatal(err)
			}
			chunks := [][]byte{
				[]byte(`{"id":"c1","choices":[{"index":0,"delta":{"role":"assistant","content":"answer"},"finish_reason":null}]}`),
				[]byte(`{"id":"c1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}`),
				[]byte(`{"id":"c1","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`),
			}
			var frames [][]byte
			for _, chunk := range chunks {
				out, err := e.Event(t.Context(), chunk)
				if err != nil {
					t.Fatal(err)
				}
				frames = append(frames, out...)
			}
			if e.Finish() != nil {
				t.Fatal("explicit usage tail did not complete stream")
			}
			out, err := e.Event(t.Context(), []byte("[DONE]"))
			if err != nil || len(out) != 0 {
				t.Fatal("optional sentinel duplicated completion")
			}
			completed, usage := 0, false
			for _, frame := range frames {
				for _, line := range bytes.Split(frame, []byte("\n")) {
					if !bytes.HasPrefix(line, []byte("data:")) {
						continue
					}
					r := gjson.ParseBytes(bytes.TrimSpace(line[5:]))
					if r.Get("type").String() == "response.completed" {
						completed++
						usage = r.Get("response.usage.input_tokens").Int() == 5 && r.Get("response.usage.output_tokens").Int() == 7
					}
					if r.Get("type").String() == "message_stop" {
						completed++
					}
					if r.Get("type").String() == "message_delta" {
						usage = r.Get("usage.output_tokens").Int() == 7
					}
				}
			}
			if completed != 1 || !usage {
				t.Fatalf("terminal count=%d actual usage=%v", completed, usage)
			}
		})
	}
}
