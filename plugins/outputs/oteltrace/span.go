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
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func (o *OtelTrace) handleSpan(metric telegraf.Metric) (ptrace.Span, error) {
	o.Log.Debugf("handling span: %s", metric.Name())
	span := ptrace.NewSpan()

	tags := metric.TagList()
	for _, tag := range tags {
		if tag.Key == influxcommon.AttributeTraceID {
			o.Log.Debugf("span trace ID string: %s", tag.Value)
			// TODO: This is where the conversion goes wrong
			decodedTraceID, err := trace.TraceIDFromHex(tag.Value)
			if err != nil {
				wrappedErr := fmt.Errorf("unable to convert trace ID hex string %s: %w", tag.Value, err)
				o.Log.Error(wrappedErr)
				return span, wrappedErr
			}
			traceID := pcommon.TraceID(decodedTraceID)
			span.SetTraceID(traceID)
		}
		if tag.Key == influxcommon.AttributeSpanID {
			o.Log.Debugf("span span ID string: %s", tag.Value)
			decodedSpanID, err := trace.SpanIDFromHex(tag.Value)
			if err != nil {
				wrappedErr := fmt.Errorf("unable to convert span ID hex string %s: %w", tag.Value, err)
				o.Log.Error(wrappedErr)
				return span, wrappedErr
			}
			spanID := pcommon.SpanID(decodedSpanID)
			span.SetSpanID(spanID)
		}
		// TODO: There are other things that can be configured to be span dimensions
	}

	fields := metric.FieldList()
	for _, field := range fields {
		if field.Key == influxcommon.AttributeTraceState {
			o.Log.Debugf("trace state string: %+v", field.Value)
			// TODO: convert interface into the correct state
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
			o.Log.Debugf("span kind: %+v", field.Value)
			spanKindRaw := field.Value
			spanKindStr, ok := field.Value.(string)
			if !ok {
				return span, fmt.Errorf("invalid type for span kind %v", spanKindRaw)
			}
			sk := SpanKindFromString(spanKindStr)
			span.SetKind(ptrace.SpanKind(int32(sk)))
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
			droppedAttrCount, ok := droppedAttrCountRaw.(uint64)
			if !ok {
				return span, fmt.Errorf("invalid type for dropped attributes count %v", droppedAttrCountRaw)
			}
			// influx wants uint64, traces want uint32 - go figure
			span.SetDroppedAttributesCount(uint32(droppedAttrCount))
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
