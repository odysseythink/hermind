# Plan 8c: Distributed Tracing (stdlib-only)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans.

**Goal:** Add lightweight OpenTelemetry-style tracing to the gateway so each inbound message has a trace ID, parent/child span hierarchy, attributes, and durations — all without pulling in the full OpenTelemetry Go SDK. Ship a JSON-lines file exporter for local debugging and an in-memory buffer exporter for tests. Real OTLP/HTTP (protobuf) export is deliberately deferred.

**Architecture:**
- New `tracing/` package with:
  - `TraceID` and `SpanID` (16-byte and 8-byte random values, hex-encoded)
  - `Span` struct with Name, StartTime, EndTime, TraceID, SpanID, ParentSpanID, Attributes, Status, Events
  - `Tracer` type with `Start(ctx, name, attrs...) (context.Context, *Span)` and `End(span)`
  - `SpanFromContext(ctx)` / `ContextWithSpan(ctx, span)`
  - `Exporter` interface with `Export(spans []*Span)` and `Shutdown(ctx)`
  - `JSONLinesExporter` writing one JSON object per span to an `io.Writer`
  - `MemoryExporter` buffering spans for tests
- Gateway integration: `NewGateway` accepts an optional `Tracer`. `handleMessage` starts a `gateway.handleMessage` root span, attaches the platform + user as attributes, and defers `End`. The existing `logging.WithRequestID` becomes the TraceID.

**Non-goals:**
- OTLP/protobuf wire format — later
- Trace context propagation over HTTP headers (W3C traceparent) — later, out of scope since platforms emit their own ids
- Sampling — always on
- Span links and metrics correlation — later

---

## Task 1: tracing package

- [ ] Create `tracing/id.go` with `TraceID [16]byte`, `SpanID [8]byte`, `NewTraceID()`, `NewSpanID()` using `crypto/rand`.
- [ ] Create `tracing/span.go` with `Span` struct, `Status`, `Attribute`, plus `Start`/`End` on `Tracer`.
- [ ] Create `tracing/context.go` with `SpanFromContext` / `ContextWithSpan` using an unexported context key.
- [ ] Create `tracing/exporter.go` with the `Exporter` interface, `NoopExporter`, `MemoryExporter`, `JSONLinesExporter`.
- [ ] Create `tracing/tracer.go` with `Tracer { exporter Exporter }` and the `Start`/`End` implementation that writes completed spans to the exporter.
- [ ] Create `tracing/tracer_test.go` covering:
  - ID generation is random
  - `Start`/`End` records duration
  - Nested spans inherit trace id and set parent span id correctly
  - `MemoryExporter.Spans()` returns recorded spans
  - `JSONLinesExporter` emits one valid JSON object per line
- [ ] Commit `feat(tracing): add stdlib-only span/tracer package`.

---

## Task 2: Gateway integration

- [ ] Add `*tracing.Tracer` field to `gateway.Gateway` with a setter `SetTracer`.
- [ ] In `handleMessage`, after the dedup check, call `tracer.Start(ctx, "gateway.handleMessage", tracing.String("platform", in.Platform), tracing.String("user_id", in.UserID))`. Defer `tracer.End(span)`.
- [ ] If the handler returns an error, mark the span status as error and record the message as an attribute.
- [ ] Commit `feat(gateway): trace handleMessage with tracing package`.

---

## Task 3: CLI wiring

- [ ] Add `TracingConfig { Enabled bool; File string }` to config.
- [ ] In `cli/gateway.go`, if `cfg.Tracing.Enabled`, build a `JSONLinesExporter` pointing at `cfg.Tracing.File` (or stderr if empty) and attach to the Gateway.
- [ ] Commit `feat(cli): wire tracing exporter into hermes gateway`.

---

## Verification Checklist

- [ ] `go test ./tracing/... ./gateway/...` passes
- [ ] A trace file accumulates one JSON span per inbound message
- [ ] Nested spans have distinct span ids and the same trace id
