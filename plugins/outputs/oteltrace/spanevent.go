package oteltrace

import (
	"encoding/json"
	"fmt"

	influxcommon "github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/telegraf"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func (o *OtelTrace) handleSpanEvent(metric telegraf.Metric) (spanEvent ptrace.SpanEvent, traceID string, spanID string, err error) {
	spanEvent = ptrace.NewSpanEvent()
	fields := metric.FieldList()
	for _, field := range fields {
		if field.Key == influxcommon.AttributeDroppedAttributesCount {
			droppedAttrCount, ok := field.Value.(uint32)
			if !ok {
				return spanEvent, "", "", fmt.Errorf("invalid type for dropped attributes count %v", field.Value)
			}
			spanEvent.SetDroppedAttributesCount(uint32(droppedAttrCount))
		}
		if field.Key == influxcommon.AttributeAttributes {
			attributesRaw := field.Value
			attributesRawStr, ok := attributesRaw.(string)
			if !ok {
				return spanEvent, "", "", fmt.Errorf("invalid type for attributes %v", attributesRaw)
			}
			attributesField := make(map[string]any)
			if err := json.Unmarshal([]byte(attributesRawStr), &attributesField); err != nil {
				return spanEvent, "", "", fmt.Errorf("failed to unmarshal attributes to map %w", err)
			}
			spanEvent.Attributes().FromRaw(attributesField)
		}
	}

	tags := metric.TagList()
	for _, tag := range tags {
		if tag.Key == influxcommon.AttributeTraceID {
			pTraceID := pcommon.TraceID(([]byte(tag.Value)))
			traceID = pTraceID.String()
		}
		if tag.Key == influxcommon.AttributeSpanID {
			pSpanID := pcommon.SpanID([]byte(tag.Value))
			spanID = pSpanID.String()
		}
	}
	return
}
