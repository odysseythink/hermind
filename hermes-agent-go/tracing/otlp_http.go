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
