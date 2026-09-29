package modeltransport

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxBody = 32 << 20

// Endpoint belongs to one Session Executor. Close it only after the native
// executor stops using it, including preparation failure and final teardown.
type Endpoint struct {
	Route     Route
	Protocol  Protocol
	BaseURL   string
	APIKey    string
	server    *http.Server
	transport *http.Transport
	cancel    context.CancelFunc
	mu        sync.Mutex
	closing   bool
	requests  sync.WaitGroup
	once      sync.Once
}

// Prepare chooses native support first. Only a protocol mismatch creates a
// local conversion endpoint; no provider request or model execution happens here.
func Prepare(provider Provider, model string, native ...Protocol) (*Endpoint, error) {
	if provider.Validate() != nil || strings.TrimSpace(model) == "" || len(native) == 0 {
		return nil, ErrConfiguration
	}
	route, err := ResolveRoute(provider.Protocol, native)
	if err != nil {
		return nil, err
	}
	if !route.Converted() {
		return &Endpoint{Route: route, Protocol: route.Native, BaseURL: provider.BaseURL, APIKey: provider.APIKey}, nil
	}
	return newEndpoint(provider, model, route.Native)
}

func newEndpoint(provider Provider, model string, native Protocol) (*Endpoint, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("model conversion endpoint unavailable")
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		listener.Close()
		return nil, errors.New("model conversion credential unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = time.Minute
	endpoint := &Endpoint{Route: Route{Upstream: provider.Protocol, Native: native}, Protocol: native, BaseURL: "http://" + listener.Addr().String(),
		APIKey: base64.RawURLEncoding.EncodeToString(token), transport: transport, cancel: cancel}
	if native != Anthropic {
		endpoint.BaseURL += "/v1"
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	endpoint.server = &http.Server{ReadHeaderTimeout: 10 * time.Second, ReadTimeout: time.Minute,
		BaseContext: func(net.Listener) context.Context { return ctx },
		Handler: http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			endpoint.mu.Lock()
			if endpoint.closing {
				endpoint.mu.Unlock()
				http.Error(w, "model endpoint closed", http.StatusServiceUnavailable)
				return
			}
			endpoint.requests.Add(1)
			endpoint.mu.Unlock()
			defer endpoint.requests.Done()
			endpoint.serve(w, request, client, provider, model)
		})}
	go func() { _ = endpoint.server.Serve(listener) }()
	return endpoint, nil
}

func (e *Endpoint) Close() error {
	e.once.Do(func() {
		e.mu.Lock()
		e.closing = true
		e.mu.Unlock()
		if e.cancel != nil {
			e.cancel()
		}
		if e.server != nil {
			_ = e.server.Close()
		}
		e.requests.Wait()
		if e.transport != nil {
			e.transport.CloseIdleConnections()
		}
	})
	return nil
}

func (e *Endpoint) authenticated(request *http.Request) bool {
	values := request.Header.Values("Authorization")
	keys := request.Header.Values("X-Api-Key")
	if len(values) > 1 || len(keys) > 1 || len(values)+len(keys) == 0 {
		return false
	}
	for _, value := range values {
		if !strings.HasPrefix(value, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(value, "Bearer ")), []byte(e.APIKey)) != 1 {
			return false
		}
	}
	for _, key := range keys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(e.APIKey)) != 1 {
			return false
		}
	}
	return true
}

func (e *Endpoint) serve(w http.ResponseWriter, request *http.Request, client *http.Client, provider Provider, model string) {
	if !e.authenticated(request) {
		modelError(w, e.Protocol, http.StatusUnauthorized, "invalid local model credential")
		return
	}
	if request.Method != http.MethodPost || request.URL.Path != "/v1"+e.Protocol.Path() {
		modelError(w, e.Protocol, http.StatusNotFound, "model protocol operation is unsupported")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxBody)
	raw, err := io.ReadAll(request.Body)
	if err != nil {
		modelError(w, e.Protocol, http.StatusRequestEntityTooLarge, "model request exceeds limit")
		return
	}
	var envelope struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if !json.Valid(raw) || json.Unmarshal(raw, &envelope) != nil || envelope.Model != model {
		modelError(w, e.Protocol, http.StatusBadRequest, "model request does not match the selected model")
		return
	}
	if err := e.Route.Validate(requestRequirements(e.Protocol, raw)); err != nil {
		modelError(w, e.Protocol, http.StatusBadRequest, "model protocol capability is unsupported")
		return
	}
	exchange, err := NewExchange(e.Protocol, provider.Protocol, model, raw, envelope.Stream)
	if err != nil {
		message := "model request capability is unsupported"
		if errors.Is(err, ErrUnsupported) {
			message = err.Error()
		}
		modelError(w, e.Protocol, http.StatusBadRequest, message)
		return
	}
	upstream, err := http.NewRequestWithContext(request.Context(), http.MethodPost, upstreamURL(provider), bytes.NewReader(exchange.Request()))
	if err != nil {
		modelError(w, e.Protocol, http.StatusBadGateway, "model upstream unavailable")
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	if provider.Protocol == Anthropic {
		upstream.Header.Set("X-Api-Key", provider.APIKey)
		upstream.Header.Set("Anthropic-Version", "2023-06-01")
	} else {
		upstream.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}
	var upstreamEnvelope struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(exchange.Request(), &upstreamEnvelope)
	if upstreamEnvelope.Stream {
		upstream.Header.Set("Accept", "text/event-stream")
	}
	response, err := client.Do(upstream)
	if err != nil {
		modelError(w, e.Protocol, http.StatusBadGateway, "model upstream unavailable")
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		status := response.StatusCode
		if status < 400 || status > 599 {
			status = http.StatusBadGateway
		}
		modelError(w, e.Protocol, status, "upstream model request failed")
		return
	}
	if envelope.Stream {
		e.serveStream(w, request, response, exchange)
		return
	}
	if upstreamEnvelope.Stream && !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		modelError(w, e.Protocol, http.StatusBadGateway, "upstream model did not return an event stream")
		return
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		modelError(w, e.Protocol, http.StatusBadGateway, "invalid upstream model response")
		return
	}
	converted, err := exchange.Response(request.Context(), body)
	if err != nil {
		modelError(w, e.Protocol, http.StatusBadGateway, "unsupported upstream model response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(converted)
}

func upstreamURL(provider Provider) string {
	base := strings.TrimRight(provider.BaseURL, "/")
	if provider.Protocol == Anthropic && !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	return base + provider.Protocol.Path()
}

func modelError(w http.ResponseWriter, protocol Protocol, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody(protocol, status, message))
}

func errorBody(protocol Protocol, status int, message string) map[string]any {
	kind := "api_error"
	switch status {
	case 400, 404, 413, 422:
		kind = "invalid_request_error"
	case 401:
		kind = "authentication_error"
	case 403:
		kind = "permission_error"
	case 429:
		kind = "rate_limit_error"
	}
	body := map[string]any{"error": map[string]any{"type": kind, "message": message}}
	if protocol == Anthropic {
		body["type"] = "error"
	}
	return body
}
