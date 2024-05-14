# Telegraf Execd Otel Trace Plugin (WIP)

This is meant to do the inverse of what the
[otel2influx package](https://github.com/influxdata/influxdb-observability/blob/main/otel2influx/traces.go#L130)
does, except as a Telegraf output exec plugin.

### References

#### Otel

- [Otel Span Proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto) -
  Not sure how useful it actually is to use the proto as the direct reference.
  However, the Otel Go library has so many layers of abstraction that it's very
  difficult to figure out how to actually construct and send along a span
  manually.
- [otel2influx](https://github.com/influxdata/influxdb-observability/blob/main/otel2influx/traces.go#L130) -
  This is the otel2influx implementation. I'm doing essentially the inverse of
  this.

#### Telegraf

- [Telegraf Metric Godoc](https://pkg.go.dev/github.com/influxdata/telegraf@v1.30.2#Metric) -
  Useful for looking at the structure of `telegraf.Metric`
- [Telegraf Output PlayFab](https://github.com/dgkanatsios/telegraftoplayfab)
- [Telegraf Output Kinesis Plugin](https://github.com/morfien101/telegraf-output-kinesis/tree/master) -
  Example exec'd output plugin I'm basing the structure of this plugin off of.
- [Telegraf Output Plugin README](https://github.com/influxdata/telegraf/blob/master/docs/OUTPUTS.md#output-plugins) -
  General guidance for Telegraf output plugins.
