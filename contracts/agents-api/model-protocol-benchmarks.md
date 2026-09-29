# Model transport benchmarks — historical, retired

> Historical record of the retired built-in model proxy and converter. The
> implementation, dependency choices, commands and results below describe that
> earlier revision only; they are not current setup instructions, dependencies
> or supported protocol combinations. The current native-only contract is
> [model execution](model-execution.md#saved-defaults-and-precedence). No
> compatibility alias, automatic migration or restoration of this converter is supported.

Measured on 2026-09-28 on Linux amd64, Intel Xeon Platinum 8358P @ 2.60 GHz,
Go 1.26.8, GOMAXPROCS=8, CLIProxyAPI v8.0.3. This measures the thin Exchange
integration, including the composed Responses-to-Messages route. The shared host
was not isolated from other workloads; figures are microbenchmarks, not latency SLOs.
No model or network is used by these benchmark workloads.

Short input is one user message and a function declaration. Large input contains
128 alternating messages with 2 KiB each. Tool loop includes a streamed function
call, its result in a second request, and a text stream. Concurrent SSE uses 32
workers, 64 deltas and 8 KiB of text per stream. Outputs are consumed and discarded.
HTTP framing, backpressure and native engine processing are excluded.

The tables report the median of three testing.Benchmark allocation runs with a
200 ms target. Sequential latency uses 20 warmups and 500 samples; concurrent
latency uses 800 streams. p95/p99 include scheduling and GC. Concurrent mean is
aggregate throughput per stream, rather than an individual stream's latency.

## Short request

| Client → upstream | Mean (ms) | p95 (ms) | p99 (ms) | Bytes/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| anthropic → responses | 0.076 | 0.046 | 0.076 | 13,605 | 137 |
| anthropic → chat_completions | 0.059 | 0.065 | 0.099 | 7,614 | 84 |
| responses → anthropic | 0.205 | 0.276 | 0.552 | 19,807 | 239 |
| responses → chat_completions | 0.074 | 0.100 | 0.215 | 10,996 | 120 |
| chat_completions → anthropic | 0.129 | 0.189 | 0.316 | 10,777 | 132 |
| chat_completions → responses | 0.090 | 0.040 | 0.065 | 11,579 | 106 |

## 256 KiB history

| Client → upstream | Mean (ms) | p95 (ms) | p99 (ms) | Bytes/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| anthropic → responses | 15.484 | 18.166 | 34.967 | 4,021,333 | 2,568 |
| anthropic → chat_completions | 15.872 | 18.228 | 19.874 | 3,588,969 | 1,553 |
| responses → anthropic | 56.984 | 63.098 | 75.256 | 8,020,006 | 5,743 |
| responses → chat_completions | 26.077 | 27.209 | 33.847 | 3,631,486 | 1,603 |
| chat_completions → anthropic | 37.060 | 41.171 | 46.010 | 5,740,777 | 4,153 |
| chat_completions → responses | 17.402 | 30.225 | 43.999 | 5,161,877 | 3,370 |

## Two-request tool loop

| Client → upstream | Mean (ms) | p95 (ms) | p99 (ms) | Bytes/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| anthropic → responses | 0.676 | 1.056 | 1.599 | 133,175 | 923 |
| anthropic → chat_completions | 0.555 | 1.006 | 1.515 | 116,540 | 794 |
| responses → anthropic | 1.927 | 3.170 | 3.520 | 315,546 | 2,410 |
| responses → chat_completions | 1.180 | 1.874 | 2.153 | 234,483 | 1,541 |
| chat_completions → anthropic | 0.782 | 0.803 | 0.984 | 95,143 | 993 |
| chat_completions → responses | 0.605 | 0.965 | 1.525 | 83,962 | 808 |

## Concurrent SSE

| Client → upstream | Mean (ms) | p95 (ms) | p99 (ms) | Bytes/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| anthropic → responses | 0.318 | 40.509 | 64.359 | 728,156 | 2,814 |
| anthropic → chat_completions | 0.285 | 22.059 | 33.589 | 593,693 | 3,028 |
| responses → anthropic | 0.625 | 38.772 | 55.790 | 1,493,381 | 6,200 |
| responses → chat_completions | 0.513 | 28.864 | 39.006 | 1,233,740 | 4,003 |
| chat_completions → anthropic | 0.167 | 19.135 | 28.550 | 323,167 | 2,796 |
| chat_completions → responses | 0.211 | 24.962 | 33.010 | 461,607 | 2,814 |

## Sampled heap

During the concurrent run, runtime.MemStats is sampled every 1 ms after a baseline
GC. These are observed Go heap peaks, not exact maxima, OS RSS, per-Session retained
memory or a ceiling. Sampling can miss short spikes and perturbs timings.

| Client → upstream | Baseline heap (B) | Sampled peak (B) | Increase (B) |
| --- | ---: | ---: | ---: |
| anthropic → responses | 2,256,344 | 8,569,920 | 6,313,576 |
| anthropic → chat_completions | 2,260,440 | 7,682,336 | 5,421,896 |
| responses → anthropic | 2,283,664 | 12,052,232 | 9,768,568 |
| responses → chat_completions | 2,310,952 | 10,051,656 | 7,740,704 |
| chat_completions → anthropic | 2,316,000 | 9,032,160 | 6,716,160 |
| chat_completions → responses | 2,437,928 | 7,721,664 | 5,283,736 |

## Binary cost

With Go 1.26.8 and `go build -trimpath -ldflags="-s -w"`, the Runtime executable
is 10,051,849 bytes at baseline `5b5c10fc4afeb20a1f5ac52207a9810ece85dbed` and
19,157,257 bytes with this integration, an increase of 9,105,408 bytes.
This is separate from container image size and process memory. Same-protocol
production connections bypass the converter and are covered by selection tests.

## Reproduce

```sh
GOMAXPROCS=8 go test ./internal/modeltransport -run '^$' \
  -bench '^BenchmarkProduct' -benchmem -benchtime=200ms -count=3
OAC_MODELTRANSPORT_PROFILE=1 GOMAXPROCS=8 go test ./internal/modeltransport \
  -run '^TestProductBenchmarkProfile$' -count=1 -v
```

The profile is opt-in. The source fixtures live in `internal/modeltransport`.
Raw samples are retained as `thin-bench.log` and `thin-profile.log` in the Linux
qualification workspace. The independent library comparison uses Go 1.27 and
is not a toolchain-controlled comparison with these product numbers.
