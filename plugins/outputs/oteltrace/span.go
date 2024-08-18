package oteltrace

import (
	"encoding/json"
	"fmt"
	"time"

	influxcommon "github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/telegraf"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func (o *OtelTrace) handleSpan(metric telegraf.Metric) (ptrace.Span, error) {
	span := ptrace.NewSpan()

	tags := metric.TagList()
	for _, tag := range tags {
		if tag.Key == influxcommon.AttributeTraceID {
			traceID := pcommon.TraceID(([]byte(tag.Value)))
			span.SetTraceID(traceID)
		}
		if tag.Key == influxcommon.AttributeSpanID {
			spanID := pcommon.SpanID([]byte(tag.Value))
			span.SetSpanID(spanID)
		}
		// TODO: There are other things that can be configured to be span dimensions, but whatever``
	}

	fields := metric.FieldList()
	for _, field := range fields {
		if field.Key == influxcommon.AttributeTraceState {
			// TODO: convert interface into the correct shiz
			traceStateRaw := field.Value
			traceState, ok := traceStateRaw.(string)
			if !ok {
				return span, fmt.Errorf("invalid type for span trace_state %v", traceStateRaw)
			}
			span.TraceState().FromRaw(traceState)
		}
		if field.Key == influxcommon.AttributeParentSpanID {
			parentSpanIDRaw := field.Value
			parentSpanID, ok := parentSpanIDRaw.([]byte)
			if !ok {
				return span, fmt.Errorf("invalid type for parent_span_id %v", parentSpanIDRaw)
			}
			pSid := pcommon.SpanID(parentSpanID)
			span.SetParentSpanID(pSid)
		}
		if field.Key == influxcommon.AttributeSpanName {
			spanNameRaw := field.Value
			spanName, ok := spanNameRaw.(string)
			if !ok {
				return span, fmt.Errorf("invalid type for span name %v", spanNameRaw)
			}
			span.SetName(spanName)
		}
		if field.Key == influxcommon.AttributeSpanKind {
			spanKindRaw := field.Value
			spanKindStr, ok := field.Value.(string)
			if !ok {
				return span, fmt.Errorf("invalid type for span kind %v", spanKindRaw)
			}
			span.SetKind(ptrace.SpanKind(tracepb.Span_SpanKind_value[spanKindStr]))
		}
		if field.Key == influxcommon.AttributeEndTimeUnixNano {
			endTimeRaw := field.Value
			endTime, ok := field.Value.(int64)
			if !ok {
				return span, fmt.Errorf("invalid type for span end_time_unix_nano %v", endTimeRaw)
			}
			et := time.Unix(0, endTime)
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(et))
		}
		if field.Key == semconv.OtelStatusCode {
			statusCodeRaw := field.Value
			statusCodeStr, ok := statusCodeRaw.(string)
			if !ok {
				return span, fmt.Errorf("invalid type for status status_code %v", statusCodeRaw)
			}
			sc := ptrace.StatusCode(tracepb.Status_StatusCode(tracepb.Status_StatusCode_value[statusCodeStr]))
			span.Status().SetCode(sc)
		}
		if field.Key == semconv.OtelStatusDescription {
			statusMessageRaw := field.Value
			statusMessage, ok := statusMessageRaw.(string)
			if !ok {
				return span, fmt.Errorf("invalid type for status message %v", statusMessageRaw)
			}
			span.Status().SetMessage(statusMessage)
		}
		if field.Key == influxcommon.AttributeDroppedAttributesCount {
			droppedAttrCountRaw := field.Value
			droppedAttrCount, ok := droppedAttrCountRaw.(uint32)
			if !ok {
				return span, fmt.Errorf("invalid type for dropped attributes count %v", droppedAttrCountRaw)
			}
			span.SetDroppedAttributesCount(droppedAttrCount)
		}
		if field.Key == influxcommon.AttributeAttributes {
			attributesRaw := field.Value
			attributesRawStr, ok := attributesRaw.(string)
			if !ok {
				return span, fmt.Errorf("invalid type for attributes %v", attributesRaw)
			}
			attributesField := make(map[string]any)
			if err := json.Unmarshal([]byte(attributesRawStr), attributesField); err != nil {
				return span, fmt.Errorf("failed to unmarshal attributes to map %w", err)
			}
			span.Attributes().FromRaw(attributesField)
		}
	}
	return span, nil
}
