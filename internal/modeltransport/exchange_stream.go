package modeltransport

import (
	"bufio"
	"bytes"
	"context"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/translator/builtin"
	"github.com/tidwall/gjson"
)

func (e *exchange) Event(ctx context.Context, payload []byte) ([][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e.failure != nil {
		return nil, e.failure
	}
	payload = bytes.TrimSpace(payload)
	if e.terminal && bytes.Equal(payload, []byte("[DONE]")) && e.target != Anthropic {
		return nil, nil
	}
	if err := e.observe(payload); err != nil {
		return nil, err
	}
	outputs := [][]byte{payload}
	// A usage-only tail following finish_reason is an explicit upstream end
	// used by compatible APIs that omit [DONE]. Pass that end to the SDK.
	if e.target == ChatCompletions && e.terminal && !bytes.Equal(payload, []byte("[DONE]")) {
		outputs = append(outputs, []byte("[DONE]"))
	}
	for i := len(e.steps) - 1; i >= 0; i-- {
		step := &e.steps[i]
		var next [][]byte
		for _, data := range outputs {
			chunks := builtin.Registry().TranslateStream(ctx, step.to, step.from, e.model, step.original, step.request, append([]byte("data: "), data...), &step.state)
			for _, chunk := range chunks {
				if len(bytes.TrimSpace(chunk)) == 0 {
					continue
				}
				payloads, err := sdkPayloads(chunk)
				if err != nil {
					return nil, e.fail()
				}
				next = append(next, payloads...)
			}
		}
		// Claude->Chat's SDK emits the usage chunk at message_stop; the Chat
		// stream transport sentinel is supplied at that same confirmed boundary.
		if e.terminal && step.from == clientFormat(ChatCompletions) {
			hasDone := false
			for _, data := range next {
				hasDone = hasDone || bytes.Equal(data, []byte("[DONE]"))
			}
			if !hasDone {
				next = append(next, []byte("[DONE]"))
			}
		}
		outputs = next
	}
	var frames [][]byte
	for _, data := range outputs {
		frames = append(frames, frame(e.source, data))
	}
	return e.releaseFrames(frames), nil
}

func upstreamError(r gjson.Result) bool {
	return r.Get("error").Type != gjson.Null || r.Get("response.error").Type != gjson.Null ||
		r.Get("type").String() == "error" || r.Get("type").String() == "response.failed"
}

// Observe only transport lifecycle, never content, tool schemas, or token math.
func (e *exchange) observe(payload []byte) error {
	e.streamBytes += int64(len(payload))
	if e.failure != nil {
		return e.failure
	}
	if e.streamBytes > 64<<20 || e.terminal {
		return e.fail()
	}
	if bytes.Equal(payload, []byte("[DONE]")) {
		if e.target != ChatCompletions || !e.stopped {
			return e.fail()
		}
		e.terminal = true
		return nil
	}
	if !jsonObject(payload) {
		return e.fail()
	}
	r := gjson.ParseBytes(payload)
	if upstreamError(r) {
		return e.fail()
	}
	switch e.target {
	case ChatCompletions:
		if !r.Get("choices").IsArray() {
			return e.fail()
		}
		for _, choice := range r.Get("choices").Array() {
			e.started = true
			if choice.Get("finish_reason").String() != "" {
				e.stopped = true
			}
		}
		if e.stopped && len(r.Get("choices").Array()) == 0 && r.Get("usage").IsObject() {
			e.terminal = true
		}
	case Anthropic:
		switch r.Get("type").String() {
		case "message_start":
			e.started = true
		case "message_delta":
			if r.Get("delta.stop_reason").String() != "" {
				e.stopped = true
			}
		case "message_stop":
			if !e.started || !e.stopped {
				return e.fail()
			}
			e.terminal = true
		}
	case Responses:
		switch r.Get("type").String() {
		case "response.completed", "response.incomplete":
			if !r.Get("response").IsObject() {
				return e.fail()
			}
			e.started, e.stopped, e.terminal = true, true, true
		}
	}
	return nil
}

func (e *exchange) Finish() error {
	if e.failure != nil {
		return e.failure
	}
	if !e.terminal || !e.stopped {
		return e.fail()
	}
	return nil
}

func frame(p Protocol, payload []byte) []byte {
	var out []byte
	if p != ChatCompletions && jsonObject(payload) {
		if typ := gjson.GetBytes(payload, "type").String(); typ != "" {
			out = append(out, []byte("event: "+typ+"\n")...)
		}
	}
	out = append(out, []byte("data: ")...)
	out = append(out, payload...)
	return append(out, '\n', '\n')
}

func sdkPayloads(raw []byte) ([][]byte, error) {
	raw = bytes.TrimSpace(raw)
	if jsonObject(raw) || bytes.Equal(raw, []byte("[DONE]")) {
		return [][]byte{raw}, nil
	}
	return ssePayloads(append(bytes.Clone(raw), '\n', '\n'))
}

func ssePayloads(raw []byte) ([][]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 8<<20)
	var payloads [][]byte
	var data []string
	size := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(data) > 0 {
				payloads = append(payloads, []byte(strings.Join(data, "\n")))
				data = nil
				size = 0
			}
		} else if strings.HasPrefix(line, "data:") {
			size += len(line)
			if size > 8<<20 {
				return nil, ErrExchange
			}
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if scanner.Err() != nil || len(data) > 0 {
		return nil, ErrExchange
	}
	return payloads, nil
}

// Completion may precede the upstream terminator in the SDK. Delay only that
// boundary so an interrupted transport cannot publish a successful completion.
func (e *exchange) releaseFrames(outputs [][]byte) [][]byte {
	var ready [][]byte
	for _, f := range outputs {
		if len(e.pending) > 0 || e.completionFrame(f) {
			e.pending = append(e.pending, f)
		} else {
			ready = append(ready, f)
		}
	}
	if e.terminal {
		ready = append(ready, e.pending...)
		e.pending = nil
		if e.source == ChatCompletions {
			hasDone := false
			for _, f := range ready {
				hasDone = hasDone || bytes.Contains(f, []byte("data: [DONE]"))
			}
			if !hasDone {
				ready = append(ready, frame(ChatCompletions, []byte("[DONE]")))
			}
		}
	}
	return ready
}
func (e *exchange) completionFrame(f []byte) bool {
	for _, line := range bytes.Split(f, []byte("\n")) {
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(line[5:])
		if bytes.Equal(data, []byte("[DONE]")) {
			return true
		}
		r := gjson.ParseBytes(data)
		switch e.source {
		case Anthropic:
			if r.Get("type").String() == "message_stop" || r.Get("delta.stop_reason").String() != "" {
				return true
			}
		case Responses:
			if r.Get("type").String() == "response.completed" || r.Get("type").String() == "response.incomplete" {
				return true
			}
		case ChatCompletions:
			for _, choice := range r.Get("choices").Array() {
				if choice.Get("finish_reason").String() != "" {
					return true
				}
			}
		}
	}
	return false
}
