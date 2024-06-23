package oteltrace

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	influxcommon "github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ telegraf.Output = (*OtelTrace)(nil)

//go:embed sample.conf
var sampleConfig string

const (
	defaultServiceAddress = "localhost:4317"
)

type OtelTrace struct {
	Debug          bool   `toml:"debug"`
	ServiceAddress string `toml:"service_address"`
	// TODO: Verify if it works
	// https://github.com/influxdata/telegraf/blob/master/docs/developers/LOGGING.md#plugin-logging
	Log            telegraf.Logger `toml:"-"`
	Exporter       sdktrace.SpanExporter
	tracerProvider sdktrace.TracerProvider
	tracer         trace.Tracer
}

func (o *OtelTrace) Init() error {
	if o.ServiceAddress == "" {
		o.ServiceAddress = defaultServiceAddress
	}

	return nil
}

func (o *OtelTrace) SampleConfig() string {
	return sampleConfig
}

func (o *OtelTrace) Connect() error {
	var traceExporter sdktrace.SpanExporter
	var err error
	// If debug, use the stdouttracer instead of a real tracer
	if o.Debug {
		traceExporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		// traceExporter, err = sdktrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			wrappedErr := fmt.Errorf("failed to create stdout exporter: %w", err)
			o.Log.Error(wrappedErr)
			return wrappedErr
		}
	} else {
		conn, err := grpc.NewClient(
			o.ServiceAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			wrappedErr := fmt.Errorf("failed to create grpc client for %s - err: %w", o.ServiceAddress, err)
			o.Log.Error(wrappedErr)
			return err
		}
		traceExporter, err = otlptracegrpc.New(context.Background(), otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			wrappedErr := fmt.Errorf("failed to create trace exporter: %w", err)
			o.Log.Error(wrappedErr)
			return wrappedErr
		}
	}
	// bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		// TODO: replace with batcher when not debugging
		sdktrace.WithSyncer(traceExporter),
		// sdktrace.WithBatcher(traceExporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		// sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return nil
}

func (o *OtelTrace) Close() error {
	return o.tracerProvider.Shutdown(context.Background())
}

/*
spans end_time_unix_nano="2021-02-19 20:50:25.6893952 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="d5270e78d85f570f",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="4c28227be6a010e1",status_code="STATUS_CODE_OK",trace_id="7d4854815225332c9834e6dbf85b9380" 1613767825689169000
spans end_time_unix_nano="2021-02-19 20:50:25.6893952 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="lets-go",net.peer.ip="1.2.3.4",peer.service="tracegen-server",service.name="tracegen",span.kind="client",span_id="d5270e78d85f570f",status_code="STATUS_CODE_OK",trace_id="7d4854815225332c9834e6dbf85b9380" 1613767825689135000
spans end_time_unix_nano="2021-02-19 20:50:25.6895667 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="b57e98af78c3399b",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="a0643a156d7f9f7f",status_code="STATUS_CODE_OK",trace_id="fd6b8bb5965e726c94978c644962cdc8" 1613767825689388000
spans end_time_unix_nano="2021-02-19 20:50:25.6895667 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="lets-go",net.peer.ip="1.2.3.4",peer.service="tracegen-server",service.name="tracegen",span.kind="client",span_id="b57e98af78c3399b",status_code="STATUS_CODE_OK",trace_id="fd6b8bb5965e726c94978c644962cdc8" 1613767825689303300
spans end_time_unix_nano="2021-02-19 20:50:25.6896741 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="6a8e6a0edcc1c966",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="d68f7f3b41eb8075",status_code="STATUS_CODE_OK",trace_id="651dadde186b7834c52b13a28fc27bea" 1613767825689480300
*/
func (o *OtelTrace) Write(metrics []telegraf.Metric) error {
	// Invert this logic
	// https://github.com/influxdata/influxdb-observability/blob/4be04f3bc56b026c388342a0365a09f9171999a2/otel2influx/traces.go#L78
	traceBatch := map[string]ptrace.Traces{}
	spanBatch := map[string]ptrace.Span{}

	for _, metric := range metrics {
		// The metric names we care about are span, span-links, logs
		switch name := metric.Name(); name {
		case influxcommon.MeasurementSpans:
			// TODO: both span event and span link have to link back to the parent span somehow.
			// The spanevent has the trace and span ID. Likely need some sort of map lookup
			span, err := o.handleSpan(metric)
			if err != nil {
				return err
			}
			traceKey := traceLookupKey(span.TraceID().String(), span.SpanID().String())
			traces, exists := traceBatch[traceKey]
			if !exists {
				traces = ptrace.NewTraces()
				traceBatch[traceKey] = traces
			}
			spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().Spans()
			// TODO
			// spanKey := spanLookupKey(span.TraceID().String(), span.SpanID().String())
			// _, exists := spanBatch[spanKey]
			// // TODO: Not sure if this makes sense or not
			// if exists {
			// 	return fmt.Errorf("already encountered span with key %s", spanKey)
			// }
			// spanBatch[spanKey] = span
		case influxcommon.MeasurementSpanLinks:
			// TODO
			spanLink, err := o.handleSpanLink(metric)
			if err != nil {
				return err
			}
			spanKey := spanLookupKey(spanLink.TraceID().String(), spanLink.SpanID().String())
			span, ok := spanBatch[spanKey]
			if !ok {
				o.Log.Debug("failed to find span with key %s", spanKey)
				return nil
			}
			emptySpanLink := span.Links().AppendEmpty()
			spanLink.CopyTo(emptySpanLink)
		case influxcommon.MeasurementLogs:
			spanEvent, traceID, spanID, err := o.handleSpanEvent(metric)
			if err != nil {
				return err
			}
			spanKey := spanLookupKey(traceID, spanID)
			span, ok := spanBatch[spanKey]
			if !ok {
				o.Log.Debug("failed to find span with key %s", spanKey)
				return nil
			}
			emptySpanEvent := span.Events().AppendEmpty()
			spanEvent.CopyTo(emptySpanEvent)
		}
		// For understanding all of the terminology here
		// https://opentelemetry.io/docs/concepts/signals/traces/
	}

	return nil
}

func spanLookupKey(traceID, spanID string) string {
	return fmt.Sprintf("%s::%s", traceID, spanID)
}

func traceLookupKey(traceID, spanID string) string {
	return fmt.Sprintf("%s::%s", traceID, spanID)
}

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
			if err := json.Unmarshal([]byte(attributesRawStr), attributesField); err != nil {
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

func init() {
	// TODO
	outputs.Add("oteltrace", func() telegraf.Output { return &OtelTrace{} })
}

// TODO Not sure if I need this anymore
func ConvertAnyValue(raw any) *commonpb.AnyValue {
	av := &commonpb.AnyValue{}
	switch v := raw.(type) {
	case string:
		av.Value = &commonpb.AnyValue_StringValue{
			StringValue: v,
		}
	case bool:
		av.Value = &commonpb.AnyValue_BoolValue{
			BoolValue: v,
		}
	case int64:
		av.Value = &commonpb.AnyValue_IntValue{
			IntValue: v,
		}
	case float64:
		av.Value = &commonpb.AnyValue_DoubleValue{
			DoubleValue: v,
		}
	case []byte:
		av.Value = &commonpb.AnyValue_BytesValue{
			BytesValue: v,
		}
	case []any:
		anyValueList := commonpb.ArrayValue{
			Values: make([]*commonpb.AnyValue, 0),
		}
		for _, valRaw := range v {
			anyValueList.Values = append(anyValueList.Values, ConvertAnyValue(valRaw))
		}
		av.Value = &commonpb.AnyValue_ArrayValue{
			ArrayValue: &anyValueList,
		}
	case map[string]any:
		anyValueKv := commonpb.KeyValueList{
			Values: make([]*commonpb.KeyValue, 0),
		}
		for k, valRaw := range v {
			kv := commonpb.KeyValue{
				Key:   k,
				Value: ConvertAnyValue(valRaw),
			}
			anyValueKv.Values = append(anyValueKv.Values, &kv)
		}
		av.Value = &commonpb.AnyValue_KvlistValue{
			KvlistValue: &anyValueKv,
		}
	}
	return av
}

/*
func (o *OtelTrace) handleSpan(metric telegraf.Metric) error {
	// Might just need to deal with the actual raw protos underneath? FUUUUU
	// Can we move to using https://pkg.go.dev/go.opentelemetry.io/collector/consumer/pdata#NewSpan
	span := &tracepb.Span{}

	// traceID/spanID are in tags
	// https://github.com/influxdata/influxdb-observability/blob/4be04f3bc56b026c388342a0365a09f9171999a2/otel2influx/traces.go#L211
	tags := metric.TagList()
	for _, tag := range tags {
		if tag.Key == influxcommon.AttributeTraceID {
			span.TraceId = []byte(tag.Value)
		}
		if tag.Key == influxcommon.AttributeSpanID {
			span.SpanId = []byte(tag.Value)
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
				return fmt.Errorf("invalid type for span trace_state %v", traceStateRaw)
			}
			span.TraceState = traceState
		}
		if field.Key == influxcommon.AttributeParentSpanID {
			parentSpanIDRaw := field.Value
			parentSpanID, ok := parentSpanIDRaw.([]byte)
			if !ok {
				return fmt.Errorf("invalid type for parent_span_id %v", parentSpanIDRaw)
			}
			span.ParentSpanId = parentSpanID
		}
		if field.Key == influxcommon.AttributeSpanName {
			spanNameRaw := field.Value
			spanName, ok := spanNameRaw.(string)
			if !ok {
				return fmt.Errorf("invalid type for span name %v", spanNameRaw)
			}
			span.Name = spanName
		}
		if field.Key == influxcommon.AttributeSpanKind {
			spanKindRaw := field.Value
			spanKindStr, ok := field.Value.(string)
			if !ok {
				return fmt.Errorf("invalid type for span kind %v", spanKindRaw)
			}
			span.Kind = tracepb.Span_SpanKind(tracepb.Span_SpanKind_value[spanKindStr])
		}
		if field.Key == influxcommon.AttributeEndTimeUnixNano {
			endTimeRaw := field.Value
			endTime, ok := field.Value.(uint64)
			if !ok {
				return fmt.Errorf("invalid type for span end_time_unix_nano %v", endTimeRaw)
			}
			span.EndTimeUnixNano = endTime
		}
		span.Status = &tracepb.Status{}
		if field.Key == semconv.OtelStatusCode {
			statusCodeRaw := field.Value
			statusCodeStr, ok := statusCodeRaw.(string)
			if !ok {
				return fmt.Errorf("invalid type for status status_code %v", statusCodeRaw)
			}
			span.Status.Code = tracepb.Status_StatusCode(tracepb.Status_StatusCode_value[statusCodeStr])
		}
		if field.Key == semconv.OtelStatusDescription {
			statusMessageRaw := field.Value
			statusMessage, ok := statusMessageRaw.(string)
			if !ok {
				return fmt.Errorf("invalid type for status message %v", statusMessageRaw)
			}
			span.Status.Message = statusMessage
		}
		if field.Key == influxcommon.AttributeDroppedAttributesCount {
			droppedAttrCountRaw := field.Value
			droppedAttrCount, ok := droppedAttrCountRaw.(uint32)
			if !ok {
				return fmt.Errorf("invalid type for dropped attributes count %v", droppedAttrCountRaw)
			}
			span.DroppedAttributesCount = droppedAttrCount
		}
		if field.Key == influxcommon.AttributeAttributes {
			attributesRaw := field.Value
			attributesRawStr, ok := attributesRaw.(string)
			if !ok {
				return fmt.Errorf("invalid type for attributes %v", attributesRaw)
			}
			attributesField := make(map[string]any)
			if err := json.Unmarshal([]byte(attributesRawStr), attributesField); err != nil {
				return fmt.Errorf("failed to unmarshal attributes to map %w", err)
			}
			attributes := make([]*commonpb.KeyValue, 0)
			for k, v := range attributesField {
				kv := commonpb.KeyValue{
					Key:   k,
					Value: ConvertAnyValue(v),
				}
				attributes = append(attributes, &kv)
			}
			span.Attributes = attributes
		}
	}

	return nil
}
*/
