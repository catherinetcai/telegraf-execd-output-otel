package config

type PluginConfig struct{}

type ExporterConfig struct {
	Endpoint string
	TLS      bool
}
