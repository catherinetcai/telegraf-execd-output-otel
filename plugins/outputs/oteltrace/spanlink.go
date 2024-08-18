package oteltrace

import (
	"encoding/json"
	"fmt"

	influxcommon "github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/telegraf"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func (o *OtelTrace) handleSpanLink(metric telegraf.Metric) (ptrace.SpanLink, error) {
	spanLink := ptrace.NewSpanLink()
	tags := metric.TagList()
	var traceID, spanID, linkedTraceID, linkedSpanID string

	for _, tag := range tags {
		if tag.Key == influxcommon.AttributeTraceID {
			pTraceID := pcommon.TraceID(([]byte(tag.Value)))
			spanLink.SetTraceID(pTraceID)
			traceID = pTraceID.String()
		}
		if tag.Key == influxcommon.AttributeSpanID {
			pSpanID := pcommon.SpanID([]byte(tag.Value))
			spanLink.SetSpanID(pSpanID)
			spanID = pSpanID.String()
		}
		if tag.Key == influxcommon.AttributeLinkedTraceID {
			pLinkedTraceID := pcommon.TraceID(([]byte(tag.Value)))
			linkedTraceID = pLinkedTraceID.String()
		}
		if tag.Key == influxcommon.AttributeLinkedSpanID {
			pLinkedSpanID := pcommon.SpanID([]byte(tag.Value))
			linkedSpanID = pLinkedSpanID.String()
		}
	}

	fields := metric.FieldList()
	for _, field := range fields {
		if field.Key == influxcommon.AttributeTraceState {
			// TODO: convert interface into the correct shiz
			traceStateRaw := field.Value
			traceState, ok := traceStateRaw.(string)
			if !ok {
				return spanLink, fmt.Errorf("invalid type for span trace_state %v", traceStateRaw)
			}
			spanLink.TraceState().FromRaw(traceState)
		}
		if field.Key == influxcommon.AttributeAttributes {
			attributesRaw := field.Value
			attributesRawStr, ok := attributesRaw.(string)
			if !ok {
				return spanLink, fmt.Errorf("invalid type for attributes %v", attributesRaw)
			}
			attributesField := make(map[string]any)
			if err := json.Unmarshal([]byte(attributesRawStr), attributesField); err != nil {
				return spanLink, fmt.Errorf("failed to unmarshal attributes to map %w", err)
			}
			spanLink.Attributes().FromRaw(attributesField)
		}
		if field.Key == influxcommon.AttributeDroppedAttributesCount {
			droppedAttrCountRaw := field.Value
			droppedAttrCount, ok := droppedAttrCountRaw.(uint32)
			if !ok {
				return spanLink, fmt.Errorf("invalid type for dropped attributes count %v", droppedAttrCountRaw)
			}
			spanLink.SetDroppedAttributesCount(droppedAttrCount)
		}
	}

	return spanLink, nil
}
