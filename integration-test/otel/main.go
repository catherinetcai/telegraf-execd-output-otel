package main

// This is basically a quick test to make sure you can even use the exporter to fire off spans
// Example of how to do it with curl + HTTP:
// https://grafana.com/docs/tempo/latest/api_docs/pushing-spans-with-http/#push-spans-with-otlp as well

import (
	"context"
	_ "embed"
	"fmt"
	"log"

	"github.com/davecgh/go-spew/spew"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var exporterGRPCServer = "localhost:4317"

func main() {
	conn, err := grpc.NewClient(
		exporterGRPCServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	log.Printf("constructing client conn to %s\n", exporterGRPCServer)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to create grpc client for %s - err: %w", exporterGRPCServer, err)
		panic(wrappedErr)
	}
	log.Println("constructing exporter grpc client")
	traceExporter := ptraceotlp.NewGRPCClient(conn)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to create trace exporter: %w", err)
		panic(wrappedErr)
	}

	log.Println("constructing new trace")
	traces := ptrace.NewTraces()
	rSpan := traces.ResourceSpans().AppendEmpty()
	resourceAttrs := rSpan.Resource().Attributes()
	resourceAttrs.PutStr("service.name", "test-with-ptrace")
	sSpan := rSpan.ScopeSpans().AppendEmpty()
	sSpan.Scope().SetName("manual-test-ptrace")
	span := sSpan.Spans().AppendEmpty()
	span.SetName("spanitron")
	span.SetKind(2)
	span.SetTraceID(pcommon.TraceID([]byte("71699b6fe85982c7c8995ea3d9c95df2")))
	span.SetSpanID(pcommon.SpanID([]byte("3c191d03fa8be065")))
	span.Status().SetCode(ptrace.StatusCode(1))

	log.Println("calling export with manual trace")
	response, err := traceExporter.Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(traces))
	if err != nil {
		panic(fmt.Errorf("failed to export spans - %w", err))
	}
	spew.Dump(response)
}
