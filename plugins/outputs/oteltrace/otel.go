package oteltrace

import (
	"context"
	_ "embed"
	"fmt"

	influxcommon "github.com/influxdata/influx-observability/common"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
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

func (o *OtelTrace) handleSpan(metric []telegraf.Metric) error {
	// Might just need to deal with the actual raw protos underneath? FUUUUU
	span := &tracepb.Span{}

	// traceID/spanID are in tags
	// https://github.com/influxdata/influxdb-observability/blob/4be04f3bc56b026c388342a0365a09f9171999a2/otel2influx/traces.go#L211
	tags := metric.TagList()
	for _, tag := range tags {
		if tag.Key == influxcommon.AttributeTraceID {
			span.TraceId = tag.Value
		}
		if tag.Key == influxcommon.AttributeSpanID {
			span.SpanId = tag.Value
		}
		// TODO: There are other things that can be configured to be span dimensions, but whatever``
	}

	fields := metric.FieldList()
	for _, field := range fields {
		if field.Key == influxcommon.AttributeTraceState {
			// TODO: convert interface into the correct shiz
			traceStateRaw := field.Value
		}
		if field.Key == influxcommon.AttributeParentSpanID {
			parentSpanIDRaw := field.Value
		}
		if field.Key == influxcommon.AttributeSpanName {
			attributeSpanNameRaw := field.Value
		}
		if field.Key == influxcommon.AttributeSpanKind {
			attributeSpanKindRaw := field.Value
		}
		if field.Key == influxcommon.AttributeEndTimeUnixNano {
		}
		if field.Key == influxcommon.AttributeDurationNano {
		}
		if field.Key == semconv.OtelStatusCode {
		}
		if field.Key == semconv.OtelStatusDescription {
		}
		if field.Key == influxcommon.AttributeDroppedAttributesCount {
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
