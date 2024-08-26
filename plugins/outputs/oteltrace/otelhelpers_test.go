package oteltrace_test

import (
	"context"
	"fmt"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"

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
		semconv.OtelStatusCode:                       codes.OK.String(),
	}
	mtrace := metric.New(
		influxcommon.MeasurementSpans,
		tags,
		fields,
		time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	)
	return mtrace
}
