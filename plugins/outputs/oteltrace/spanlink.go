package oteltrace

import (
	"encoding/json"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	influxcommon "github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/telegraf"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TODO: Returns the spanlink with the linked trace ID and span ID attached, but
// this still has to be appended to the pre-existing span's links
func (o *OtelTrace) handleSpanLink(metric telegraf.Metric) (ptrace.SpanLink, error) {
	o.Log.Debugf("handling span link: %s", metric.Name())
	spanLink := ptrace.NewSpanLink()
	tags := metric.TagList()

	for _, tag := range tags {
		// https://github.com/influxdata/influxdb-observability/blob/main/otel2influx/traces.go#L267
		if tag.Key == influxcommon.AttributeLinkedTraceID {
			o.Log.Debugf("spanlink linked trace ID: %s", tag.Value)
			pLinkedTraceID := pcommon.TraceID(([]byte(tag.Value)))
			spanLink.SetTraceID(pLinkedTraceID)
			// linkedTraceID = pLinkedTraceID.String()
		}
		if tag.Key == influxcommon.AttributeLinkedSpanID {
			o.Log.Debugf("spanlink linked span ID: %s", tag.Value)
			pLinkedSpanID := pcommon.SpanID([]byte(tag.Value))
			spanLink.SetSpanID(pLinkedSpanID)
			// linkedSpanID = pLinkedSpanID.String()
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
			if err := json.Unmarshal([]byte(attributesRawStr), &attributesField); err != nil {
				return spanLink, fmt.Errorf("failed to unmarshal attributes to map %w", err)
			}
			spanLink.Attributes().FromRaw(attributesField)
		}
		if field.Key == influxcommon.AttributeDroppedAttributesCount {
			droppedAttrCountRaw := field.Value
			spew.Dump(droppedAttrCountRaw)
			droppedAttrCount, ok := droppedAttrCountRaw.(uint64)
			if !ok {
				return spanLink, fmt.Errorf("invalid type for dropped attributes count %v", droppedAttrCountRaw)
			}
			// ptrace takes this as uint32, influx takes it as uint64
			spanLink.SetDroppedAttributesCount(uint32(droppedAttrCount))
		}
	}

	return spanLink, nil
}
