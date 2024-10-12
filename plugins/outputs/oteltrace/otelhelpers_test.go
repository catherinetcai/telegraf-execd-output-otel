package oteltrace_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"

	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	influxcommon "github.com/influxdata/influxdb-observability/common"
)

var _ sdktrace.IDGenerator = (*testIDGenerator)(nil)

var testIDGen = testIDGenerator{}

type testIDGenerator struct {
	traceID int
	spanID  int
}

func (gen *testIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	traceIDHex := fmt.Sprintf("%032x", gen.traceID)
	traceID, _ := trace.TraceIDFromHex(traceIDHex)
	gen.traceID++

	spanID := gen.NewSpanID(ctx, traceID)
	return traceID, spanID
}

func (gen *testIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	spanIDHex := fmt.Sprintf("%016x", gen.spanID)
	spanID, _ := trace.SpanIDFromHex(spanIDHex)
	gen.spanID++
	return spanID
}

// https://github.com/open-telemetry/opentelemetry-collector/blob/pdata/v1.13.0/pdata/ptrace/ptraceotlp/grpc_test.go#L93
type fakeTracesServer struct {
	ptraceotlp.UnimplementedGRPCServer
	t   *testing.T
	err error
}

func (f fakeTracesServer) Export(_ context.Context, request ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	assert.Equal(f.t, generateTracesRequest(), request)
	return ptraceotlp.NewExportResponse(), f.err
}

func generateTraces() ptrace.Traces {
	gen := &testIDGenerator{
		traceID: 10,
		spanID:  1,
	}
	traceID, spanID := gen.NewIDs(context.Background())
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("fakespan")
	ptraceID := pcommon.TraceID([16]byte(traceID))
	pspanID := pcommon.SpanID([8]byte(spanID))
	span.SetTraceID(ptraceID)
	span.SetSpanID(pspanID)
	span.SetKind(ptrace.SpanKindServer)
	return td
}

func generateTracesRequest() ptraceotlp.ExportRequest {
	return ptraceotlp.NewExportRequestFromTraces(generateTraces())
}

// TODO: Make this less awful?
func generateTraceAsMetric() telegraf.Metric {
	gen := &testIDGenerator{
		traceID: 10,
		spanID:  1,
	}
	traceID, spanID := gen.NewIDs(context.Background())
	tags := map[string]string{
		influxcommon.AttributeTraceID: traceID.String(),
		influxcommon.AttributeSpanID:  spanID.String(),
	}
	fields := map[string]interface{}{
		influxcommon.AttributeDroppedAttributesCount: uint64(0),
		influxcommon.AttributeDroppedEventsCount:     uint64(0),
		influxcommon.AttributeSpanKind:               ptrace.SpanKindServer.String(),
		influxcommon.AttributeSpanName:               "fakespan",
		// semconv.OtelStatusCode:                       codes.OK.String(),
	}
	mtrace := metric.New(
		influxcommon.MeasurementSpans,
		tags,
		fields,
		time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	)
	return mtrace
}
