package oteltrace

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	influxcommon "github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
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
		traceExporter, err = sdktrace.New(stdouttrace.WithPrettyPrint())
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
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
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
	for _, metric := range metrics {
		name := metric.Name()
		// The metric names we care about are span, span-links, logs
		switch name := metric.Name(); name {
		case influxcommon.MeasurementSpans:
			// TODO
			o.handleSpan(metric)
		case influxcommon.MeasurementSpanLinks:
			// TODO
			o.handleSpanLink()
		case influxcommon.MeasurementLogs:
			// TODO
			o.handleSpanEvent()
		}

		// For understanding all of the terminology here
		// https://opentelemetry.io/docs/concepts/signals/traces/

	}
	return nil
}

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

func (o *OtelTrace) handleSpanEvent(metric []telegraf.Metric) error {
	return nil
}

func (o *OtelTrace) handleSpanLink(metric []telegraf.Metric) error {
	return nil
}

func init() {
	// TODO
	outputs.Add("oteltrace", func() telegraf.Output { return &OtelTrace{} })
}

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
