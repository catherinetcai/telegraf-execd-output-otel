package oteltrace

import (
	_ "embed"
	"fmt"

	influxcommon "github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

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
	Log      telegraf.Logger `toml:"-"`
	Exporter ptraceotlp.GRPCClient

	// Private store to allow for teardown
	clientConn *grpc.ClientConn
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
	var err error
	conn, err := grpc.NewClient(
		o.ServiceAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to create grpc client for %s - err: %w", o.ServiceAddress, err)
		o.Log.Error(wrappedErr)
		return err
	}
	traceExporter := ptraceotlp.NewGRPCClient(conn)
	o.Exporter = traceExporter
	return nil
}

func (o *OtelTrace) Close() error {
	if o.clientConn != nil {
		return o.clientConn.Close()
	}
	return nil
}

/*
spans end_time_unix_nano="2021-02-19 20:50:25.6893952 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="d5270e78d85f570f",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="4c28227be6a010e1",status_code="STATUS_CODE_OK",trace_id="7d4854815225332c9834e6dbf85b9380" 1613767825689169000
spans end_time_unix_nano="2021-02-19 20:50:25.6893952 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="lets-go",net.peer.ip="1.2.3.4",peer.service="tracegen-server",service.name="tracegen",span.kind="client",span_id="d5270e78d85f570f",status_code="STATUS_CODE_OK",trace_id="7d4854815225332c9834e6dbf85b9380" 1613767825689135000
spans end_time_unix_nano="2021-02-19 20:50:25.6895667 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="b57e98af78c3399b",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="a0643a156d7f9f7f",status_code="STATUS_CODE_OK",trace_id="fd6b8bb5965e726c94978c644962cdc8" 1613767825689388000
spans end_time_unix_nano="2021-02-19 20:50:25.6895667 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="lets-go",net.peer.ip="1.2.3.4",peer.service="tracegen-server",service.name="tracegen",span.kind="client",span_id="b57e98af78c3399b",status_code="STATUS_CODE_OK",trace_id="fd6b8bb5965e726c94978c644962cdc8" 1613767825689303300
spans end_time_unix_nano="2021-02-19 20:50:25.6896741 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="6a8e6a0edcc1c966",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="d68f7f3b41eb8075",status_code="STATUS_CODE_OK",trace_id="651dadde186b7834c52b13a28fc27bea" 1613767825689480300
*/
// https://opentelemetry.io/docs/collector/building/receiver/#representing-operations-with-spans
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
				rs := traces.ResourceSpans().AppendEmpty()
				// TODO: Only add the relevant resource attributes, not all of them from span
				if err := rs.Resource().Attributes().FromRaw(span.Attributes().AsRaw()); err != nil {
					return fmt.Errorf("unable to add attributes from span to resource %w", err)
				}
			}
			// TODO: Is this the right thing to do in terms of scope spans?
			scopeSpans := traces.ResourceSpans().At(0).ScopeSpans().At(0)
			newSpan := scopeSpans.Spans().AppendEmpty()
			span.CopyTo(newSpan)
		case influxcommon.MeasurementSpanLinks:
			// TODO
			spanLink, err := o.handleSpanLink(metric)
			if err != nil {
				return err
			}
			traceKey := traceLookupKey(spanLink.TraceID().String(), spanLink.SpanID().String())
			traces, exists := traceBatch[traceKey]
			if !exists {
				traces = ptrace.NewTraces()
				traceBatch[traceKey] = traces
				rs := traces.ResourceSpans().AppendEmpty()
				// TODO
				if err := rs.Resource().Attributes().FromRaw(spanLink.Attributes().AsRaw()); err != nil {
					return fmt.Errorf("unable to add attributes from spanlink to resource %w", err)
				}
			}
			scopeSpans := traces.ResourceSpans().At(0).ScopeSpans().At(0)
			spanCount := scopeSpans.Spans().Len()
			var span ptrace.Span
			for i := 0; i < spanCount; i++ {
				currentSpan := scopeSpans.Spans().At(i)
				if spanLink.SpanID().String() == currentSpan.SpanID().String() {
					span = &currentSpan
				}
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
