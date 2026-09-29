package modeltransport

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type productBenchmarkCase struct {
	name string
	run  func() error
}

func productRequest(p Protocol, large bool) []byte {
	var request map[string]any
	_ = json.Unmarshal([]byte(exchangeRequests[p]), &request)
	field := "messages"
	if p == Responses {
		field = "input"
	}
	messages := []any{map[string]any{"role": "user", "content": "Hello"}}
	if large {
		messages = nil
		for i := 0; i < 128; i++ {
			role := "user"
			if i%2 == 1 {
				role = "assistant"
			}
			messages = append(messages, map[string]any{"role": role, "content": strings.Repeat("history ", 256)})
		}
	}
	request[field] = messages
	raw, _ := json.Marshal(request)
	return raw
}

func productStream(source, target Protocol, request []byte, events [][]byte) error {
	e, err := NewExchange(source, target, "upstream", request, true)
	if err != nil {
		return err
	}
	_ = e.Request()
	for _, event := range events {
		if _, err := e.Event(context.Background(), event); err != nil {
			return err
		}
	}
	return e.Finish()
}

func productCases() []productBenchmarkCase {
	var cases []productBenchmarkCase
	for _, source := range protocols() {
		for _, target := range protocols() {
			if source == target {
				continue
			}
			for _, large := range []bool{false, true} {
				kind := "short"
				if large {
					kind = "history256KiB"
				}
				request := productRequest(source, large)
				cases = append(cases, productBenchmarkCase{kind + "/" + string(source) + "_to_" + string(target), func() error {
					e, err := NewExchange(source, target, "upstream", request, false)
					if err == nil {
						_ = e.Request()
					}
					return err
				}})
			}
			initial, followup := productRequest(source, false), []byte(exchangeRequests[source])
			tool, text := toolEvents(target), textEvents(target)
			cases = append(cases, productBenchmarkCase{"toolLoop/" + string(source) + "_to_" + string(target), func() error {
				if err := productStream(source, target, initial, tool); err != nil {
					return err
				}
				return productStream(source, target, followup, text)
			}})
		}
	}
	return cases
}

func BenchmarkProductExchange(b *testing.B) {
	for _, tc := range productCases() {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := tc.run(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Emit 8 KiB in 64 deltas while retaining each protocol's normal terminal frame.
func productLongEvents(p Protocol) [][]byte {
	chunk := strings.Repeat("x", 128)
	var out [][]byte
	for _, event := range textEvents(p) {
		s := string(event)
		if strings.Contains(s, `"Hel"`) {
			for i := 0; i < 64; i++ {
				delta := strings.ReplaceAll(s, `"Hel"`, `"`+chunk+`"`)
				if i > 0 {
					delta = strings.Replace(delta, `"role":"assistant",`, "", 1)
				}
				out = append(out, []byte(delta))
			}
			continue
		}
		if strings.Contains(s, `"lo"`) {
			continue
		}
		out = append(out, []byte(strings.ReplaceAll(s, `"Hello"`, `"`+strings.Repeat(chunk, 64)+`"`)))
	}
	return out
}

func BenchmarkProductConcurrentSSE(b *testing.B) {
	for _, source := range protocols() {
		for _, target := range protocols() {
			if source == target {
				continue
			}
			request, events := productRequest(source, false), productLongEvents(target)
			b.Run(string(source)+"_to_"+string(target), func(b *testing.B) {
				b.ReportAllocs()
				b.SetParallelism(4)
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						if err := productStream(source, target, request, events); err != nil {
							b.Error(err)
							return
						}
					}
				})
			})
		}
	}
}

// Opt-in measurements are separate from ns/op so timing and heap sampling do
// not distort the allocation benchmark. These are process-local samples.
func TestProductBenchmarkProfile(t *testing.T) {
	if os.Getenv("OAC_MODELTRANSPORT_PROFILE") != "1" {
		t.Skip("explicit product benchmark profile required")
	}
	t.Logf("go=%s GOMAXPROCS=%d", runtime.Version(), runtime.GOMAXPROCS(0))
	for _, tc := range productCases() {
		samples := make([]time.Duration, 500)
		for i := 0; i < 20; i++ {
			if err := tc.run(); err != nil {
				t.Fatal(err)
			}
		}
		for i := range samples {
			start := time.Now()
			if err := tc.run(); err != nil {
				t.Fatal(err)
			}
			samples[i] = time.Since(start)
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		t.Logf("latency %s samples=%d p95_ns=%d p99_ns=%d", tc.name, len(samples), samples[474].Nanoseconds(), samples[494].Nanoseconds())
	}
	for _, source := range protocols() {
		for _, target := range protocols() {
			if source == target {
				continue
			}
			t.Run(fmt.Sprint(source, "_to_", target), func(t *testing.T) { profileConcurrentStreams(t, source, target) })
		}
	}
}

func profileConcurrentStreams(t *testing.T, source, target Protocol) {
	request, events := productRequest(source, false), productLongEvents(target)
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	peakHeap, peakInuse := baseline.HeapAlloc, baseline.HeapInuse
	stop, sampled := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(sampled)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				if m.HeapAlloc > peakHeap {
					peakHeap = m.HeapAlloc
				}
				if m.HeapInuse > peakInuse {
					peakInuse = m.HeapInuse
				}
			case <-stop:
				return
			}
		}
	}()
	const workers, perWorker = 32, 25
	start := make(chan struct{})
	errors := make(chan error, workers)
	latencies := make(chan time.Duration, workers*perWorker)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < perWorker; j++ {
				before := time.Now()
				if err := productStream(source, target, request, events); err != nil {
					errors <- err
					return
				}
				latencies <- time.Since(before)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(stop)
	<-sampled
	close(errors)
	close(latencies)
	for err := range errors {
		t.Fatal(err)
	}
	samples := make([]time.Duration, 0, workers*perWorker)
	for d := range latencies {
		samples = append(samples, d)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	t.Logf("concurrent %s_to_%s workers=%d streams=%d deltas=64 text_bytes=8192 p95_ns=%d p99_ns=%d baseline_heap_bytes=%d sampled_peak_heap_bytes=%d sampled_peak_heap_delta_bytes=%d sampled_peak_inuse_delta_bytes=%d", source, target, workers, len(samples), samples[(len(samples)*95+99)/100-1].Nanoseconds(), samples[(len(samples)*99+99)/100-1].Nanoseconds(), baseline.HeapAlloc, peakHeap, peakHeap-baseline.HeapAlloc, peakInuse-baseline.HeapInuse)
}
