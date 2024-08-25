package oteltrace_test

import (
	"testing"

	"github.com/catherinetcai/telegraf-execd-otel/plugins/outputs/oteltrace"
	"github.com/stretchr/testify/assert"
)

func TestOtelTraceInit(t *testing.T) {
	ot := &oteltrace.OtelTrace{}
	assert.NoError(t, ot.Init())
	assert.NotEmpty(t, ot.ServiceAddress)
}

func TestOtelTraceConnect(t *testing.T) {
}
