package otel

import (
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/outputs"
)

type otelPlugin struct{}

func (o *otelPlugin) SampleConfig() string                  { return "" }
func (o *otelPlugin) Connect() error                        { return nil }
func (o *otelPlugin) Close() error                          { return nil }
func (o *otelPlugin) Write(metrics []telegraf.Metric) error { return nil }

func init() {
	// TODO
	outputs.Add("oteltrace", nil)
}
