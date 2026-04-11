# Plan 8b: Prometheus Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans.

**Goal:** Expose Prometheus-format metrics from the gateway and cron so operators can scrape per-platform message counts, handler latency histograms, retry totals, and provider-call counts without reaching for the logs.

**Architecture:**
- New `metrics/` package wraps a minimal in-process metrics registry with the Prometheus text exposition format. No third-party dep — we implement `Counter`, `Histogram`, and a `Registry.Serve(w)` method that writes `# HELP`/`# TYPE` lines and sample values. This keeps Plan 8b dep-free and sufficient for basic scraping.
- Metrics instrumentation points:
  - `gateway_messages_total{platform}` — counter, incremented per incoming message
  - `gateway_handler_errors_total{platform}` — counter, incremented on handler error
  - `gateway_handler_retry_total{platform}` — counter, incremented per retry attempt
  - `gateway_handler_duration_seconds{platform}` — histogram, seconds per handleMessage
  - `cron_job_runs_total{job}` — counter, incremented per cron invocation
  - `cron_job_errors_total{job}` — counter, incremented on failure
- HTTP server exposing `/metrics` on a configurable address, started in both `hermes gateway` and `hermes cron` when `metrics.addr` is non-empty.

**Tech Stack:** Go 1.25 stdlib (`sync`, `net/http`, `sort`, `strconv`, `math`). No external deps.

**Deliverable at end of plan:**
```
$ hermes gateway  # with metrics.addr: :9100
$ curl -s http://localhost:9100/metrics | head
# HELP gateway_messages_total Total inbound messages.
# TYPE gateway_messages_total counter
gateway_messages_total{platform="api_server"} 42
gateway_messages_total{platform="telegram"} 7
# HELP gateway_handler_duration_seconds Handler latency histogram.
# TYPE gateway_handler_duration_seconds histogram
gateway_handler_duration_seconds_bucket{platform="api_server",le="0.1"} 12
gateway_handler_duration_seconds_bucket{platform="api_server",le="0.5"} 28
...
```

**Non-goals:**
- OpenTelemetry tracing / OTLP export — later
- prometheus/client_golang dep — intentionally avoided
- Pushgateway support — not needed for scrape model
- Histograms with custom buckets per metric — uses a single default ladder

---

## Task 1: metrics package

- [ ] Create `metrics/metrics.go` with `Counter`, `Histogram`, and `Registry` types. Default histogram buckets: `[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]` seconds.
- [ ] Create `metrics/metrics_test.go` covering register, increment, observe, and scraping.
- [ ] Run `go test ./metrics/...` — PASS.
- [ ] Commit `feat(metrics): add stdlib-only Prometheus-compatible metrics registry`.

---

## Task 2: gateway instrumentation

- [ ] Add `Metrics *metrics.Registry` to `Gateway` (optional; nil means no metrics).
- [ ] In `handleMessage`, record timings and increment counters.
- [ ] Commit `feat(gateway): instrument handleMessage with metrics counters and histogram`.

---

## Task 3: cron instrumentation

- [ ] Thread a `*metrics.Registry` into `cron.Scheduler` (optional).
- [ ] In `runJobLoop`, increment runs_total and errors_total.
- [ ] Commit `feat(cron): instrument job runs and errors`.

---

## Task 4: CLI wiring — metrics HTTP server

- [ ] Add `MetricsConfig { Addr string }` to `config/config.go`.
- [ ] In `cli/gateway.go`, when `cfg.Metrics.Addr != ""`, start an HTTP server exposing `/metrics` and pass the registry into `NewGateway`.
- [ ] Same treatment in `cli/cron.go`.
- [ ] Commit `feat(cli): add /metrics HTTP server for gateway and cron`.

---

## Verification Checklist

- [ ] `go test ./metrics/... ./gateway/... ./cron/...` passes
- [ ] `curl -s http://localhost:9100/metrics | grep gateway_` shows real metrics after sending a message
