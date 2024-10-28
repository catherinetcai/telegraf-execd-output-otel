# Telegraf Execd Otel Trace Plugin (WIP)

This is meant to do the inverse of what the
[otel2influx package](https://github.com/influxdata/influxdb-observability/blob/main/otel2influx/traces.go#L130)
does, except as a Telegraf output execd plugin.

This is still only in a WIP state. OTLP traces do export from Telegraf to a gRPC OTLP collector endpoint, but this hasn't been strenuously tested.

## Usage

- Install binary to a location on your box (e.g. `/usr/local/bin/telegraf-execd-otel`)

- Enable the Telegraf [OpenTelemetry input plugin](https://github.com/influxdata/telegraf/blob/946e4d7d3b0a484456fc336488af609812188520/plugins/inputs/opentelemetry/README.md)

- Create and place Telegraf configuration for the execd plugin at `/etc/telegraf/telegraf.d/telegraf-execd-otel.conf`

```toml
# This is the configuration for Telegraf to invoke your execd plugin. Put this into /etc/telegraf/telegraf.d
[[outputs.execd]]
  command = ["/usr/local/bin/telegraf-execd-otel", "--config", "/etc/telegraf-execd-otel/plugin.conf"]
```

- Create and place the plugin configuration to a known location (e.g. `/etc/telegraf-execd-otel/plugin.conf`). Do note place this config file into `/etc/telegraf/telegraf.d`

```toml
[[outputs.otel]]
  service_address = "localhost:4317" # Set this to a OTLP gRPC collector endpoint
```

- Restart your Telegraf instance to have it pick up the new plugin. You should now be able to start pushing OTLP traces to your Telegraf instance and have them be forwarded to your OTLP collector

## References

These are just references for building out this plugin.

### Otel

- [Otel Span Proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto) -
  Not sure how useful it actually is to use the proto as the direct reference.
  However, the Otel Go library has so many layers of abstraction that it's very
  difficult to figure out how to actually construct and send along a span
  manually.
- [otel2influx](https://github.com/influxdata/influxdb-observability/blob/main/otel2influx/traces.go#L130) -
  This is the otel2influx implementation. I'm doing essentially the inverse of
  this.

### Telegraf

- [Telegraf Metric Godoc](https://pkg.go.dev/github.com/influxdata/telegraf@v1.30.2#Metric) -
  Useful for looking at the structure of `telegraf.Metric`
- [Telegraf Output PlayFab](https://github.com/dgkanatsios/telegraftoplayfab)
- [Telegraf Output Kinesis Plugin](https://github.com/morfien101/telegraf-output-kinesis/tree/master) -
  Example exec'd output plugin I'm basing the structure of this plugin off of.
- [Telegraf Output Plugin README](https://github.com/influxdata/telegraf/blob/master/docs/OUTPUTS.md#output-plugins) -
  General guidance for Telegraf output plugins.
