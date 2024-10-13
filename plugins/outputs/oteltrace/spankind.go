package oteltrace

import (
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// Reversal of:
// https://github.com/open-telemetry/opentelemetry-go/blob/bc2fe88756962b76eb43ea2fd92ed3f5b6491cc0/trace/span.go#L162-L177
func SpanKindFromString(sk string) trace.SpanKind {
	switch strings.ToLower(sk) {
	case "server":
		return trace.SpanKindServer
	case "client":
		return trace.SpanKindClient
	case "producer":
		return trace.SpanKindProducer
	case "consumer":
		return trace.SpanKindConsumer
	default:
		return trace.SpanKindInternal
	}
}
