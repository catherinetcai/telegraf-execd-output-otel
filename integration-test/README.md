# Integration Tests

## Docker Compose

The docker-compose sets up a test environment for manually testing pushing traces through Telegraf to Grafana Tempo. The Grafana instance is just for easy viewing of the traces that are pushed through the system.

```shell
docker-compose up -d
```

The following containers will come up:

- Telegraf
  - <http://localhost:44317> (gRPC OTLP)
  - <http://localhost:44318> (HTTP OTLP)
- Tempo - <http://localhost:3200>
- Grafana - <http://localhost:3000>

## Otel

The Otel subfolder is just to run a Go test that parses the sample-span.json and pushes that as a trace to localhost:14317.

```shell
# From the otel subfolder
go run main.go
```
