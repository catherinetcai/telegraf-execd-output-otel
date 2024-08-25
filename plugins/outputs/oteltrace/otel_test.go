package oteltrace_test

import (
	"context"
	"sync"
	"testing"

	"github.com/catherinetcai/telegraf-execd-otel/plugins/outputs/oteltrace"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

func TestOtelTraceInit(t *testing.T) {
	ot := &oteltrace.OtelTrace{}
	assert.NoError(t, ot.Init())
	assert.NotEmpty(t, ot.ServiceAddress)
}

func TestOtelTrace(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	ptraceotlp.RegisterGRPCServer(s, &fakeTracesServer{t: t})
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, s.Serve(lis))
	}()
	t.Cleanup(func() {
		s.Stop()
		wg.Wait()
	})
	resolver.SetDefaultScheme("passthrough")
	ot := &oteltrace.OtelTrace{
		// https://github.com/open-telemetry/opentelemetry-collector/blob/pdata/v1.13.0/pdata/ptrace/ptraceotlp/grpc_test.go#L41
		ServiceAddress: "bufnet",
	}
	err := ot.Write(testutil.MockMetrics())
	assert.NoError(t, err, "failed to write to OtelTrace")
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

func generateTracesRequest() ptraceotlp.ExportRequest {
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("test_span")
	return ptraceotlp.NewExportRequestFromTraces(td)
}
