package modeltransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	sdk "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/translator/builtin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Exchange owns the upstream invocation and the SDK state for one request.
type Exchange interface {
	Request() []byte
	Response(context.Context, []byte) ([]byte, error)
	Event(context.Context, []byte) ([][]byte, error)
	Finish() error
}

var ErrExchange = errors.New("invalid or incomplete model exchange")

type translationStep struct {
	from, to          sdk.Format
	original, request []byte
	state             any
}

type exchange struct {
	source, target             Protocol
	model                      string
	request                    []byte
	steps                      []translationStep
	upstreamStream             bool
	terminal, stopped, started bool
	failure                    error
	pending                    [][]byte
	streamBytes                int64
}

func NewExchange(source, target Protocol, model string, request []byte, stream bool) (Exchange, error) {
	if source.Path() == "" || target.Path() == "" || model == "" || !jsonObject(request) {
		return nil, ErrUnsupported
	}
	e := &exchange{source: source, target: target, model: model}
	current, err := sjson.SetBytes(request, "model", model)
	if err != nil {
		return nil, ErrExchange
	}
	current, _ = sjson.SetBytes(current, "stream", stream)
	route := translationRoute(source, target)
	if source != target {
		registry := builtin.Registry()
		for i := 1; i < len(route); i++ {
			step := translationStep{from: clientFormat(route[i-1]), to: providerFormat(route[i]), original: bytes.Clone(current)}
			if !registry.HasRequestTransformer(step.from, step.to) || !registry.HasStreamResponseTransformer(step.from, step.to) || !registry.HasNonStreamResponseTransformer(step.from, step.to) {
				return nil, ErrUnsupported
			}
			upstreamStream := stream
			// Claude's non-stream SDK entry consumes a complete SSE transcript;
			// Codex's consumes a terminal Responses event. Request those upstream
			// representations rather than synthesizing protocol events ourselves.
			if i == len(route)-1 && (target == Anthropic || target == Responses) {
				upstreamStream = true
			}
			current = registry.TranslateRequest(step.from, step.to, model, current, upstreamStream)
			if !jsonObject(current) {
				return nil, ErrExchange
			}
			step.request = bytes.Clone(current)
			e.steps = append(e.steps, step)
		}
	}
	// Request the usage tail together with streaming. Some Chat APIs close
	// after finish_reason without [DONE] unless this transport option is set.
	if target == ChatCompletions && gjson.GetBytes(current, "stream").Bool() {
		current, _ = sjson.SetBytes(current, "stream_options.include_usage", true)
		if len(e.steps) > 0 {
			e.steps[len(e.steps)-1].request = bytes.Clone(current)
		}
	}
	e.request = bytes.Clone(current)
	e.upstreamStream = gjson.GetBytes(current, "stream").Bool()
	return e, nil
}

func clientFormat(p Protocol) sdk.Format {
	switch p {
	case Anthropic:
		return "claude"
	case Responses:
		return "openai-response"
	default:
		return "openai"
	}
}
func providerFormat(p Protocol) sdk.Format {
	if p == Responses {
		return "codex"
	}
	return clientFormat(p)
}
func (e *exchange) Request() []byte { return bytes.Clone(e.request) }
func jsonObject(raw []byte) bool    { return json.Valid(raw) && gjson.ParseBytes(raw).IsObject() }
func (e *exchange) fail() error     { e.failure = ErrExchange; return e.failure }

func (e *exchange) Response(ctx context.Context, raw []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e.failure != nil {
		return nil, e.failure
	}
	if len(raw) > maxBody {
		return nil, e.fail()
	}
	input := raw
	if e.upstreamStream {
		payloads, err := ssePayloads(raw)
		if err != nil {
			return nil, e.fail()
		}
		for _, payload := range payloads {
			if e.terminal && bytes.Equal(payload, []byte("[DONE]")) && e.target != Anthropic {
				continue
			}
			if err := e.observe(payload); err != nil {
				return nil, err
			}
			if e.target == Responses && e.terminal && jsonObject(payload) {
				input = payload
			}
		}
		if e.Finish() != nil {
			return nil, e.fail()
		}
	} else if !jsonObject(raw) || upstreamError(gjson.ParseBytes(raw)) {
		return nil, e.fail()
	}
	for i := len(e.steps) - 1; i >= 0; i-- {
		step := &e.steps[i]
		input = builtin.Registry().TranslateNonStream(ctx, step.to, step.from, e.model, step.original, step.request, input, &step.state)
		if !jsonObject(input) {
			return nil, e.fail()
		}
	}
	return bytes.Clone(input), nil
}
