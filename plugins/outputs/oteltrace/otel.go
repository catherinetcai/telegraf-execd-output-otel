package oteltrace

import (
	"context"
	_ "embed"
	"fmt"

	influxcommon "github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// Ensure the plugin conforms to the correct plugin interfaces
	_ telegraf.Initializer = (*OtelTrace)(nil)
	_ telegraf.Output      = (*OtelTrace)(nil)
)

//go:embed sample.conf
var sampleConfig string

const (
	defaultServiceAddress = "localhost:4317"
)

type OtelTrace struct {
	Debug          bool   `toml:"debug"`
	ServiceAddress string `toml:"service_address"`
	Exporter       ptraceotlp.GRPCClient

	clientConn *grpc.ClientConn

	Log telegraf.Logger `toml:"-"`
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
	o.Log.Debugf("connecting to trace exporter at: %s", o.ServiceAddress)
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
	o.clientConn = conn
	o.Exporter = traceExporter
	return nil
}

func (o *OtelTrace) Close() error {
	if o.clientConn != nil {
		o.Log.Debug("closing Otel client connection")
		return o.clientConn.Close()
	}
	return nil
}

/*
spans end_time_unix_nano="2021-02-19 20:50:25.6896741 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="6a8e6a0edcc1c966",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="d68f7f3b41eb8075",status_code="STATUS_CODE_OK",trace_id="651dadde186b7834c52b13a28fc27bea" 1613767825689480300
*/
// https://opentelemetry.io/docs/collector/building/receiver/#representing-operations-with-spans
func (o *OtelTrace) Write(metrics []telegraf.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Inversion of this logic:
	// https://github.com/influxdata/influxdb-observability/blob/4be04f3bc56b026c388342a0365a09f9171999a2/otel2influx/traces.go#L78
	traceBatch := map[string]ptrace.Traces{}
	spanBatch := map[string]ptrace.Span{}

	for _, metric := range metrics {
		o.Log.Debugf("converting otel metric: %s", metric.Name())
		// The metric names we care about are span, span-links, logs
		switch name := metric.Name(); name {
		case influxcommon.MeasurementSpans:
			span, err := o.handleSpan(metric)
			if err != nil {
				o.Log.Error(err)
				return err
			}
			traceKey := traceLookupKey(span.TraceID().String(), span.SpanID().String())
			traces, exists := traceBatch[traceKey]
			if !exists {
				traces = ptrace.NewTraces()
				traceBatch[traceKey] = traces
				rs := traces.ResourceSpans().AppendEmpty()
				if err := rs.Resource().Attributes().FromRaw(span.Attributes().AsRaw()); err != nil {
					wrappedErr := fmt.Errorf("unable to add attributes from span to resource %w", err)
					o.Log.Error(wrappedErr)
					return wrappedErr
				}
			}
			rSpan := traces.ResourceSpans().At(0)
			if rSpan.ScopeSpans().Len() == 0 {
				rSpan.ScopeSpans().AppendEmpty()
			}
			newSpan := rSpan.ScopeSpans().At(0).Spans().AppendEmpty()
			span.CopyTo(newSpan)
		case influxcommon.MeasurementSpanLinks:
			spanLink, err := o.handleSpanLink(metric)
			if err != nil {
				o.Log.Error(err)
				return err
			}
			traceKey := traceLookupKey(spanLink.TraceID().String(), spanLink.SpanID().String())
			traces, exists := traceBatch[traceKey]
			if !exists {
				traces = ptrace.NewTraces()
				traceBatch[traceKey] = traces
				rs := traces.ResourceSpans().AppendEmpty()
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
					span = currentSpan
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
	}

	for traceName, trace := range traceBatch {
		o.Log.Debugf("sending trace: %s:\n%#v", traceName, trace)
		_, err := o.Exporter.Export(context.TODO(), ptraceotlp.NewExportRequestFromTraces((trace)))
		if err != nil {
			o.Log.Errorf("failed to export traces %s: %s", trace, err)
			return err
		}
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
	outputs.Add("oteltrace", func() telegraf.Output { return &OtelTrace{} })
}
