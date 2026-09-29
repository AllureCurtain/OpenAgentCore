package modeltransport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Read SSE events incrementally, preserving multi-line data without buffering
// the complete answer. The exchange validates protocol termination separately.
func (e *Endpoint) serveStream(w http.ResponseWriter, request *http.Request, response *http.Response, exchange Exchange) {
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		modelError(w, e.Protocol, http.StatusBadGateway, "upstream model did not return an event stream")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	controller := http.NewResponseController(w)
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	var data []string
	eventBytes := 0
	emit := func() bool {
		if len(data) == 0 {
			return true
		}
		frames, err := exchange.Event(request.Context(), []byte(strings.Join(data, "\n")))
		data = nil
		eventBytes = 0
		if err != nil {
			e.streamError(w)
			_ = controller.Flush()
			return false
		}
		for _, frame := range frames {
			if _, err := w.Write(frame); err != nil {
				return false
			}
		}
		return controller.Flush() == nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if !emit() {
				return
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			eventBytes += len(line)
			if eventBytes > 8<<20 {
				e.streamError(w)
				_ = controller.Flush()
				return
			}
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if request.Context().Err() != nil {
		return
	}
	if scanner.Err() != nil || len(data) != 0 || exchange.Finish() != nil {
		e.streamError(w)
		_ = controller.Flush()
	}
}

func (e *Endpoint) streamError(w http.ResponseWriter) {
	body := errorBody(e.Protocol, http.StatusBadGateway, "upstream model stream failed or ended incompletely")
	body["type"] = "error"
	raw, _ := json.Marshal(body)
	_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", raw)
}
