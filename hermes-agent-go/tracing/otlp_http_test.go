package tracing

import (
	"testing"
	"time"

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
			Name:       "retry",
			Time:       start.Add(100 * time.Millisecond),
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
}
