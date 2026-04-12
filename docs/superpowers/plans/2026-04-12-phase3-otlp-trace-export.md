# Phase 3: OTLP/HTTP Trace Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an OTLP/HTTP exporter that plugs into the existing `Exporter` interface, enabling trace export to any OTLP-compatible backend (Jaeger, Grafana Tempo, etc.).

**Architecture:** Use pre-generated OTLP proto types from `go.opentelemetry.io/proto/otlp` (avoids running protoc). Implement `OTLPHTTPExporter` that batches spans, converts to OTLP protobuf, and POSTs to `{endpoint}/v1/traces`. Batching uses a background goroutine with flush-on-interval and flush-on-capacity.

**Tech Stack:** Go 1.25, `go.opentelemetry.io/proto/otlp` (pre-generated OTLP types), `google.golang.org/protobuf` (proto marshaling)

---

## File Structure

```
hermes-agent-go/
├── tracing/
│   ├── span.go           # (existing, unchanged)
│   ├── exporter.go       # (existing, unchanged)
│   ├── tracer.go         # (existing, unchanged)
│   ├── id.go             # (existing, unchanged)
│   ├── context.go        # (existing, unchanged)
│   ├── tracer_test.go    # (existing, unchanged)
│   ├── otlp_http.go      # OTLP/HTTP exporter with batching
│   └── otlp_http_test.go # Tests against mock HTTP server
```

Two new files in the existing `tracing` package. No changes to existing files.

---

### Task 1: Add OTLP proto dependencies

**Files:**
- Modify: `hermes-agent-go/go.mod`

- [ ] **Step 1: Add dependencies**

```bash
cd hermes-agent-go && go get go.opentelemetry.io/proto/otlp@latest google.golang.org/protobuf@latest
```

- [ ] **Step 2: Tidy**

```bash
cd hermes-agent-go && go mod tidy
```

- [ ] **Step 3: Verify dependencies appear**

```bash
grep -E "opentelemetry|protobuf" hermes-agent-go/go.mod
```

Expected: Lines for `go.opentelemetry.io/proto/otlp` and `google.golang.org/protobuf`.

- [ ] **Step 4: Run existing tests**

```bash
cd hermes-agent-go && go test ./tracing/...
```

Expected: All existing tracing tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/go.mod hermes-agent-go/go.sum
git commit -m "feat(deps): add OTLP proto types and protobuf for trace export"
```

---

### Task 2: Implement span-to-OTLP conversion

**Files:**
- Create: `hermes-agent-go/tracing/otlp_http.go`
- Create: `hermes-agent-go/tracing/otlp_http_test.go`

Start with just the conversion function — no HTTP, no batching yet. This is the core logic that maps internal `*Span` to OTLP protobuf types.

- [ ] **Step 1: Write the failing test for span conversion**

Create `hermes-agent-go/tracing/otlp_http_test.go`:

```go
package tracing

import (
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestSpanToOTLP(t *testing.T) {
	traceID := NewTraceID()
	spanID := NewSpanID()
	parentID := NewSpanID()
	start := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	end := start.Add(500 * time.Millisecond)

	span := &Span{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentID,
		Name:         "test-op",
		StartTime:    start,
		EndTime:      end,
		Status:       StatusError,
		StatusMsg:    "boom",
		Attributes:   []Attribute{String("platform", "telegram"), Int64("retries", 3)},
		Events: []Event{{
			Name: "retry",
			Time: start.Add(100 * time.Millisecond),
			Attributes: []Attribute{String("reason", "timeout")},
		}},
	}

	otlpSpan := spanToOTLP(span)

	if string(otlpSpan.TraceId) != string(traceID[:]) {
		t.Errorf("trace id mismatch")
	}
	if string(otlpSpan.SpanId) != string(spanID[:]) {
		t.Errorf("span id mismatch")
	}
	if string(otlpSpan.ParentSpanId) != string(parentID[:]) {
		t.Errorf("parent span id mismatch")
	}
	if otlpSpan.Name != "test-op" {
		t.Errorf("name = %q", otlpSpan.Name)
	}
	if otlpSpan.StartTimeUnixNano != uint64(start.UnixNano()) {
		t.Errorf("start time mismatch")
	}
	if otlpSpan.EndTimeUnixNano != uint64(end.UnixNano()) {
		t.Errorf("end time mismatch")
	}
	if otlpSpan.Status.Code != tracepb.Status_STATUS_CODE_ERROR {
		t.Errorf("status = %v", otlpSpan.Status.Code)
	}
	if otlpSpan.Status.Message != "boom" {
		t.Errorf("status msg = %q", otlpSpan.Status.Message)
	}
	if len(otlpSpan.Attributes) != 2 {
		t.Fatalf("attributes len = %d", len(otlpSpan.Attributes))
	}
	if otlpSpan.Attributes[0].Key != "platform" {
		t.Errorf("attr[0] key = %q", otlpSpan.Attributes[0].Key)
	}
	strVal := otlpSpan.Attributes[0].Value.GetStringValue()
	if strVal != "telegram" {
		t.Errorf("attr[0] value = %q", strVal)
	}
	intVal := otlpSpan.Attributes[1].Value.GetIntValue()
	if intVal != 3 {
		t.Errorf("attr[1] value = %d", intVal)
	}
	if len(otlpSpan.Events) != 1 {
		t.Fatalf("events len = %d", len(otlpSpan.Events))
	}
	if otlpSpan.Events[0].Name != "retry" {
		t.Errorf("event name = %q", otlpSpan.Events[0].Name)
	}

	// Suppress unused import warnings.
	_ = commonpb.AnyValue{}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./tracing/ -run TestSpanToOTLP -v
```

Expected: Compilation error — `spanToOTLP` is undefined.

- [ ] **Step 3: Implement the conversion function**

Create `hermes-agent-go/tracing/otlp_http.go`:

```go
package tracing

import (
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// spanToOTLP converts an internal Span to an OTLP protobuf Span.
func spanToOTLP(s *Span) *tracepb.Span {
	otlp := &tracepb.Span{
		TraceId:           s.TraceID[:],
		SpanId:            s.SpanID[:],
		Name:              s.Name,
		StartTimeUnixNano: uint64(s.StartTime.UnixNano()),
		EndTimeUnixNano:   uint64(s.EndTime.UnixNano()),
		Attributes:        attributesToOTLP(s.Attributes),
		Events:            eventsToOTLP(s.Events),
		Status:            statusToOTLP(s.Status, s.StatusMsg),
	}
	if !s.ParentSpanID.IsZero() {
		otlp.ParentSpanId = s.ParentSpanID[:]
	}
	return otlp
}

func attributesToOTLP(attrs []Attribute) []*commonpb.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]*commonpb.KeyValue, len(attrs))
	for i, a := range attrs {
		out[i] = &commonpb.KeyValue{
			Key:   a.Key,
			Value: anyValueToOTLP(a.Value),
		}
	}
	return out
}

func anyValueToOTLP(v any) *commonpb.AnyValue {
	switch val := v.(type) {
	case string:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}}
	case int64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: val}}
	case float64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: val}}
	case bool:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: val}}
	case int:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(val)}}
	default:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: ""}}
	}
}

func eventsToOTLP(events []Event) []*tracepb.Span_Event {
	if len(events) == 0 {
		return nil
	}
	out := make([]*tracepb.Span_Event, len(events))
	for i, e := range events {
		out[i] = &tracepb.Span_Event{
			TimeUnixNano: uint64(e.Time.UnixNano()),
			Name:         e.Name,
			Attributes:   attributesToOTLP(e.Attributes),
		}
	}
	return out
}

func statusToOTLP(s Status, msg string) *tracepb.Status {
	var code tracepb.Status_StatusCode
	switch s {
	case StatusOK:
		code = tracepb.Status_STATUS_CODE_OK
	case StatusError:
		code = tracepb.Status_STATUS_CODE_ERROR
	default:
		code = tracepb.Status_STATUS_CODE_UNSET
	}
	return &tracepb.Status{Code: code, Message: msg}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd hermes-agent-go && go test ./tracing/ -run TestSpanToOTLP -v
```

Expected: PASS

- [ ] **Step 5: Run all tracing tests**

```bash
cd hermes-agent-go && go test ./tracing/...
```

Expected: All pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/tracing/otlp_http.go hermes-agent-go/tracing/otlp_http_test.go
git commit -m "feat(tracing): add span-to-OTLP conversion functions"
```

---

### Task 3: Implement OTLPHTTPExporter with batching

**Files:**
- Modify: `hermes-agent-go/tracing/otlp_http.go`
- Modify: `hermes-agent-go/tracing/otlp_http_test.go`

Add the exporter struct that implements the `Exporter` interface with background batching.

- [ ] **Step 1: Write the failing test for basic export**

Add to `otlp_http_test.go`:

```go
import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

func TestOTLPHTTPExporterSendsSpans(t *testing.T) {
	var received int32
	var lastBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Errorf("content-type = %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		lastBody = body
		atomic.AddInt32(&received, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	exp := NewOTLPHTTPExporter(OTLPHTTPConfig{
		Endpoint:      srv.URL,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     10,
	})

	// Create and export a span.
	span := &Span{
		TraceID:   NewTraceID(),
		SpanID:    NewSpanID(),
		Name:      "test-op",
		StartTime: time.Now().UTC(),
		EndTime:   time.Now().UTC().Add(100 * time.Millisecond),
		Status:    StatusOK,
	}
	exp.Export(span)

	// Wait for flush.
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&received) < 1 {
		t.Fatalf("expected at least 1 request, got %d", received)
	}

	// Verify the body is valid protobuf.
	var req collectorpb.ExportTraceServiceRequest
	if err := proto.Unmarshal(lastBody, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(req.ResourceSpans) != 1 {
		t.Fatalf("resource spans = %d", len(req.ResourceSpans))
	}
	scopeSpans := req.ResourceSpans[0].ScopeSpans
	if len(scopeSpans) != 1 {
		t.Fatalf("scope spans = %d", len(scopeSpans))
	}
	spans := scopeSpans[0].Spans
	if len(spans) != 1 {
		t.Fatalf("spans = %d", len(spans))
	}
	if spans[0].Name != "test-op" {
		t.Errorf("name = %q", spans[0].Name)
	}

	// Shutdown should flush remaining.
	exp.Shutdown(context.Background())
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hermes-agent-go && go test ./tracing/ -run TestOTLPHTTPExporterSendsSpans -v
```

Expected: Compilation error — `NewOTLPHTTPExporter`, `OTLPHTTPConfig` undefined.

- [ ] **Step 3: Implement the exporter**

Add to `hermes-agent-go/tracing/otlp_http.go` (append after the existing conversion functions):

```go
import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// OTLPHTTPConfig configures the OTLP/HTTP exporter.
type OTLPHTTPConfig struct {
	// Endpoint is the base URL (e.g. "http://localhost:4318").
	// Spans are POSTed to {Endpoint}/v1/traces.
	Endpoint string

	// Headers are sent with every request (e.g. for auth tokens).
	Headers map[string]string

	// BatchSize is the max spans per flush (default 256).
	BatchSize int

	// FlushInterval is how often to flush (default 5s).
	FlushInterval time.Duration
}

// OTLPHTTPExporter exports spans to an OTLP/HTTP endpoint.
// Implements the Exporter interface.
type OTLPHTTPExporter struct {
	cfg    OTLPHTTPConfig
	client *http.Client

	mu      sync.Mutex
	buffer  []*Span
	done    chan struct{}
	stopped bool
}

// NewOTLPHTTPExporter creates and starts a batching OTLP/HTTP exporter.
func NewOTLPHTTPExporter(cfg OTLPHTTPConfig) *OTLPHTTPExporter {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 256
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	e := &OTLPHTTPExporter{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		buffer: make([]*Span, 0, cfg.BatchSize),
		done:   make(chan struct{}),
	}
	go e.flushLoop()
	return e
}

// Export adds a span to the batch buffer. If the buffer is full,
// triggers an immediate flush.
func (e *OTLPHTTPExporter) Export(s *Span) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return
	}
	e.buffer = append(e.buffer, s)
	if len(e.buffer) >= e.cfg.BatchSize {
		e.flushLocked()
	}
}

// Shutdown flushes remaining spans and stops the background goroutine.
func (e *OTLPHTTPExporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return nil
	}
	e.stopped = true
	e.flushLocked()
	e.mu.Unlock()
	close(e.done)
	return nil
}

func (e *OTLPHTTPExporter) flushLoop() {
	ticker := time.NewTicker(e.cfg.FlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-e.done:
			return
		case <-ticker.C:
			e.mu.Lock()
			e.flushLocked()
			e.mu.Unlock()
		}
	}
}

// flushLocked sends all buffered spans. Must be called with mu held.
func (e *OTLPHTTPExporter) flushLocked() {
	if len(e.buffer) == 0 {
		return
	}
	batch := e.buffer
	e.buffer = make([]*Span, 0, e.cfg.BatchSize)

	// Convert to OTLP.
	otlpSpans := make([]*tracepb.Span, len(batch))
	for i, s := range batch {
		otlpSpans[i] = spanToOTLP(s)
	}

	req := &collectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: otlpSpans,
			}},
		}},
	}

	// Marshal and send in a goroutine to avoid holding the lock during I/O.
	go e.send(req)
}

func (e *OTLPHTTPExporter) send(req *collectorpb.ExportTraceServiceRequest) {
	body, err := proto.Marshal(req)
	if err != nil {
		slog.Warn("otlp: marshal error", "err", err)
		return
	}

	url := e.cfg.Endpoint + "/v1/traces"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		slog.Warn("otlp: request error", "err", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	for k, v := range e.cfg.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		slog.Warn("otlp: send error", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		slog.Warn("otlp: transient error, spans dropped", "status", resp.StatusCode)
		return
	}
	if resp.StatusCode >= 300 {
		slog.Warn("otlp: export failed", "status", resp.StatusCode)
	}
}
```

**IMPORTANT:** The full file needs a single combined import block. Merge the imports from the conversion section and the exporter section into one block at the top of the file:

```go
import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)
```

Similarly, merge the test file imports into one block.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd hermes-agent-go && go test ./tracing/ -run TestOTLPHTTPExporter -v
```

Expected: PASS

- [ ] **Step 5: Write test for batch-on-capacity**

Add to `otlp_http_test.go`:

```go
func TestOTLPHTTPExporterFlushesOnBatchSize(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	exp := NewOTLPHTTPExporter(OTLPHTTPConfig{
		Endpoint:      srv.URL,
		FlushInterval: 10 * time.Second, // Long interval — should not trigger.
		BatchSize:     3,
	})

	for i := 0; i < 3; i++ {
		exp.Export(&Span{
			TraceID:   NewTraceID(),
			SpanID:    NewSpanID(),
			Name:      "op",
			StartTime: time.Now().UTC(),
			EndTime:   time.Now().UTC(),
			Status:    StatusOK,
		})
	}

	// The batch should have flushed immediately at size 3.
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&received) < 1 {
		t.Errorf("expected flush on batch size, got %d requests", received)
	}

	exp.Shutdown(context.Background())
}
```

- [ ] **Step 6: Write test for custom headers**

Add to `otlp_http_test.go`:

```go
func TestOTLPHTTPExporterSendsHeaders(t *testing.T) {
	var headerOK int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer my-token" {
			atomic.AddInt32(&headerOK, 1)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	exp := NewOTLPHTTPExporter(OTLPHTTPConfig{
		Endpoint:      srv.URL,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     10,
		Headers:       map[string]string{"Authorization": "Bearer my-token"},
	})

	exp.Export(&Span{
		TraceID: NewTraceID(), SpanID: NewSpanID(),
		Name: "op", StartTime: time.Now().UTC(), EndTime: time.Now().UTC(),
	})

	time.Sleep(200 * time.Millisecond)
	exp.Shutdown(context.Background())

	if atomic.LoadInt32(&headerOK) < 1 {
		t.Error("custom header not received")
	}
}
```

- [ ] **Step 7: Run all tracing tests**

```bash
cd hermes-agent-go && go test ./tracing/... -v
```

Expected: All pass (existing + 4 new tests).

- [ ] **Step 8: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/tracing/otlp_http.go hermes-agent-go/tracing/otlp_http_test.go
git commit -m "feat(tracing): add OTLP/HTTP exporter with batching"
```

---

### Task 4: Integration test — verify Exporter interface compliance

**Files:**
- Modify: `hermes-agent-go/tracing/otlp_http_test.go`

Verify that `OTLPHTTPExporter` works with the existing `Tracer` (end-to-end: `Tracer.Start` → `Tracer.End` → export via OTLP).

- [ ] **Step 1: Write the integration test**

Add to `otlp_http_test.go`:

```go
func TestOTLPHTTPExporterWithTracer(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req collectorpb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &req); err != nil {
			t.Errorf("unmarshal: %v", err)
		}
		spans := req.ResourceSpans[0].ScopeSpans[0].Spans
		for _, s := range spans {
			if s.Name == "parent" || s.Name == "child" {
				atomic.AddInt32(&received, 1)
			}
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	exp := NewOTLPHTTPExporter(OTLPHTTPConfig{
		Endpoint:      srv.URL,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     10,
	})
	tr := NewTracer(exp)

	ctx, parent := tr.Start(context.Background(), "parent", String("key", "val"))
	_, child := tr.Start(ctx, "child")
	tr.End(child)
	tr.End(parent)

	// Wait for flush.
	time.Sleep(200 * time.Millisecond)
	tr.Shutdown(context.Background())

	if atomic.LoadInt32(&received) < 2 {
		t.Errorf("expected 2 spans, got %d", received)
	}
}
```

- [ ] **Step 2: Run all tests**

```bash
cd hermes-agent-go && go test ./tracing/... -v
```

Expected: All pass.

- [ ] **Step 3: Run full project tests to check for regressions**

```bash
cd hermes-agent-go && go test ./...
```

Expected: All pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/tracing/otlp_http_test.go
git commit -m "test(tracing): add OTLP/HTTP integration test with Tracer"
```
