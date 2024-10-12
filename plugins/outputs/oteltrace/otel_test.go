package oteltrace_test

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/catherinetcai/telegraf-execd-otel/plugins/outputs/oteltrace"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
	})
	ot := &oteltrace.OtelTrace{
		Exporter: ptraceotlp.NewGRPCClient(conn),
	}
	defer ot.Close()
	// Handle empty metrics
	assert.NoError(t, ot.Write(testutil.MockMetrics()))
	// Handle non-empty metrics
	assert.NoError(t, ot.Write([]telegraf.Metric{
		generateTraceAsMetric(),
	}))
}
