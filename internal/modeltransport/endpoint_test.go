package modeltransport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tidwall/gjson"
)

const endpointRequest = `{"model":"chosen-model","max_tokens":256,"messages":[{"role":"user","content":"hello fixture"}],"tools":[{"name":"lookup","description":"Look up a city","input_schema":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}]}`
const chatToolResponse = `{"id":"chat-fixture","object":"chat.completion","created":1,"model":"chosen-model","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_fixture","type":"function","function":{"name":"lookup","arguments":"{\"city\":\"Paris\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`
const chatFirst = `{"id":"chat-fixture","object":"chat.completion.chunk","created":1,"model":"chosen-model","choices":[{"index":0,"delta":{"role":"assistant","content":"early-token"},"finish_reason":null}]}`
const chatLast = `{"id":"chat-fixture","object":"chat.completion.chunk","created":1,"model":"chosen-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`

func endpointFixture(t *testing.T, handler http.HandlerFunc) (*Endpoint, *httptest.Server) {
	t.Helper()
	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	endpoint, err := Prepare(Provider{Protocol: ChatCompletions, BaseURL: upstream.URL + "/v1", APIKey: "fixture-upstream-key"}, "chosen-model", Anthropic)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = endpoint.Close() })
	return endpoint, upstream
}

func localRequest(t *testing.T, ctx context.Context, endpoint *Endpoint, stream bool) *http.Request {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(endpointRequest), &body); err != nil {
		t.Fatal(err)
	}
	body["stream"] = stream
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.BaseURL+"/v1/messages?beta=true", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Api-Key", endpoint.APIKey)
	return request
}

func readEndpoint(t *testing.T, request *http.Request) (int, string) {
	t.Helper()
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(raw)
}

func TestEndpointTranslatesRequestToolsUsageAndCredentials(t *testing.T) {
	type observed struct{ path, auth, apiKey, body string }
	seen := make(chan observed, 1)
	endpoint, _ := endpointFixture(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		seen <- observed{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-Api-Key"), string(raw)}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, chatToolResponse)
	})
	status, body := readEndpoint(t, localRequest(t, t.Context(), endpoint, false))
	if status != http.StatusOK {
		t.Fatalf("conversion failed: %d %s", status, body)
	}
	got := <-seen
	if got.path != "/v1/chat/completions" || got.auth != "Bearer fixture-upstream-key" || got.apiKey != "" || strings.Contains(got.body, endpoint.APIKey) || strings.Contains(got.auth, endpoint.APIKey) {
		t.Fatal("wrong upstream destination or credential boundary")
	}
	if gjson.Get(got.body, "model").String() != "chosen-model" || !strings.Contains(got.body, "hello fixture") || gjson.Get(got.body, "tools.0.function.name").String() != "lookup" {
		t.Fatal("request model, input or declared tool was lost")
	}
	if gjson.Get(body, "content.0.id").String() != "call_fixture" || gjson.Get(body, "content.0.name").String() != "lookup" || gjson.Get(body, "content.0.input.city").String() != "Paris" || gjson.Get(body, "stop_reason").String() != "tool_use" {
		t.Fatalf("tool call was lost: %s", body)
	}
	if gjson.Get(body, "usage.input_tokens").Int() != 11 || gjson.Get(body, "usage.output_tokens").Int() != 5 {
		t.Fatalf("usage changed: %s", body)
	}
	if strings.Contains(body, "fixture-upstream-key") || strings.Contains(body, endpoint.APIKey) {
		t.Fatal("credential leaked in response")
	}
}

func TestEndpointFlushesFirstTextBeforeUpstreamCompletion(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	endpoint, _ := endpointFixture(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if !gjson.GetBytes(raw, "stream_options.include_usage").Bool() || r.URL.RawQuery != "" {
			t.Error("missing usage stream option or native query forwarded upstream")
			http.Error(w, "invalid transport options", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", chatFirst)
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chatLast)
	})
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(localRequest(t, t.Context(), endpoint, true))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	first := make(chan struct{})
	complete := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(response.Body)
		var body strings.Builder
		announced := false
		for scanner.Scan() {
			line := scanner.Text()
			body.WriteString(line)
			body.WriteByte('\n')
			if !announced && strings.Contains(line, "early-token") {
				close(first)
				announced = true
			}
		}
		complete <- body.String()
	}()
	select {
	case <-first:
	case <-time.After(3 * time.Second):
		t.Fatal("first text was buffered until upstream completion")
	}
	select {
	case <-complete:
		t.Fatal("stream finished before upstream completed")
	default:
	}
	unblock()
	select {
	case body := <-complete:
		if !strings.Contains(body, "message_stop") || strings.Contains(body, "event: error") {
			t.Fatalf("stream did not complete: %s", body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("completed upstream did not drain")
	}
}

func TestEndpointCancellationAndCloseStopUpstream(t *testing.T) {
	for _, mode := range []string{"cancel request", "close endpoint"} {
		t.Run(mode, func(t *testing.T) {
			entered, stopped := make(chan struct{}), make(chan struct{})
			endpoint, _ := endpointFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\n", chatFirst)
				w.(http.Flusher).Flush()
				close(entered)
				<-r.Context().Done()
				close(stopped)
			})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			response, err := (&http.Client{Timeout: 5 * time.Second}).Do(localRequest(t, ctx, endpoint, true))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("upstream did not start")
			}
			if mode == "cancel request" {
				cancel()
			} else {
				done := make(chan struct{})
				go func() { _ = endpoint.Close(); close(done) }()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Fatal("Close did not drain requests")
				}
			}
			select {
			case <-stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("upstream outlived its local request")
			}
			_ = endpoint.Close()
		})
	}
}

func TestEndpointRejectsLocalAuthenticationBeforeUpstream(t *testing.T) {
	var requests atomic.Int32
	endpoint, _ := endpointFixture(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.Error(w, "unexpected", 500) })
	for _, mode := range []string{"missing", "wrong", "duplicate", "conflicting"} {
		t.Run(mode, func(t *testing.T) {
			request := localRequest(t, t.Context(), endpoint, false)
			switch mode {
			case "missing":
				request.Header.Del("X-Api-Key")
			case "wrong":
				request.Header.Set("X-Api-Key", "wrong")
			case "duplicate":
				request.Header.Add("X-Api-Key", endpoint.APIKey)
			case "conflicting":
				request.Header.Set("Authorization", "Bearer wrong")
			}
			status, _ := readEndpoint(t, request)
			if status != http.StatusUnauthorized {
				t.Fatalf("invalid credential returned %d", status)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatal("unauthorized request reached upstream")
	}
}

func TestEndpointDoesNotFollowCredentialRedirect(t *testing.T) {
	var redirected atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		_, _ = io.WriteString(w, chatToolResponse)
	}))
	defer destination.Close()
	endpoint, _ := endpointFixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(destination.URL, "127.0.0.1", "localhost", 1)+"/stolen", http.StatusTemporaryRedirect)
	})
	status, body := readEndpoint(t, localRequest(t, t.Context(), endpoint, false))
	if status != http.StatusBadGateway || redirected.Load() != 0 || strings.Contains(body, "fixture-upstream-key") {
		t.Fatalf("redirect was followed or not rejected: status=%d requests=%d", status, redirected.Load())
	}
}

func TestEndpointRedactsUpstreamFailureBody(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			endpoint, _ := endpointFixture(t, func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "fixture-upstream-key secret-diagnostic-canary", status)
			})
			got, body := readEndpoint(t, localRequest(t, t.Context(), endpoint, false))
			if got != status || strings.Contains(body, "secret-diagnostic-canary") || strings.Contains(body, "fixture-upstream-key") {
				t.Fatalf("unsafe upstream failure: status=%d body=%s", got, body)
			}
		})
	}
}

func TestEndpointTruncatedStreamCannotReportSuccess(t *testing.T) {
	for _, tail := range []string{"", "data: " + chatLast + "\n\n"} {
		t.Run(fmt.Sprint(len(tail)), func(t *testing.T) {
			endpoint, _ := endpointFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\n%s", chatFirst, tail)
			})
			status, body := readEndpoint(t, localRequest(t, t.Context(), endpoint, true))
			if status != http.StatusOK || !strings.Contains(body, "event: error") || strings.Contains(body, "event: message_stop") {
				t.Fatalf("truncated stream reported completion: %d %s", status, body)
			}
		})
	}
}

func TestEndpointNonStreamingUsesSDKUpstreamStream(t *testing.T) {
	for _, target := range []Protocol{Anthropic, Responses} {
		for _, complete := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/complete=%t", target, complete), func(t *testing.T) {
				observed := make(chan bool, 1)
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					request, _ := io.ReadAll(r.Body)
					observed <- gjson.GetBytes(request, "stream").Bool() && r.Header.Get("Accept") == "text/event-stream"
					chunks := textEvents(target)
					if !complete {
						chunks = chunks[:len(chunks)-1]
					}
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = w.Write(eventBody(chunks))
				}))
				defer upstream.Close()
				endpoint, err := Prepare(Provider{Protocol: target, BaseURL: upstream.URL + "/v1", APIKey: "fixture-key"}, "chosen-model", ChatCompletions)
				if err != nil {
					t.Fatal(err)
				}
				defer endpoint.Close()
				request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint.BaseURL+"/chat/completions", strings.NewReader(`{"model":"chosen-model","stream":false,"messages":[{"role":"user","content":"Hello"}]}`))
				if err != nil {
					t.Fatal(err)
				}
				request.Header.Set("Authorization", "Bearer "+endpoint.APIKey)
				status, body := readEndpoint(t, request)
				if !<-observed {
					t.Fatal("SDK aggregation did not request upstream SSE")
				}
				if complete {
					if status != http.StatusOK || gjson.Get(body, "choices.0.message.content").String() != "Hello" ||
						gjson.Get(body, "usage.prompt_tokens").Int() != 5 || gjson.Get(body, "usage.completion_tokens").Int() != 7 {
						t.Fatalf("SDK aggregation failed: %d %s", status, body)
					}
				} else if status != http.StatusBadGateway {
					t.Fatal("incomplete upstream SSE accepted")
				}
			})
		}
	}
}
