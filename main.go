// Most of this file is from https://github.com/influxdata/telegraf/blob/71b58ddaf5dba73031f9207849d8eba0ee791b77/plugins/common/shim/example/cmd/main.go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/influxdata/telegraf/plugins/common/shim"
	flag "github.com/spf13/pflag"
)

var (
	version = "v0.0.0-dev"

	args cliArgs
)

type cliArgs struct {
	configFile           string
	pollInterval         time.Duration
	pollIntervalDisabled bool
	otlpExporterEndpoint string
	version              bool
}

func main() {
	flag.StringVar(&args.configFile, "config", "", "Path to config file for this plugin")
	flag.StringVar(&args.otlpExporterEndpoint, "endpoint", "0.0.0.0:4317", "OTLP exporter endpoint to send metrics to.")
	flag.DurationVar(&args.pollInterval, "poll_interval", 1*time.Second, "How often to send metrics.")
	flag.BoolVar(&args.pollIntervalDisabled, "poll_interval_disabled", false, "Set to true to disable polling.")
	flag.BoolVarP(&args.version, "version", "v", false, "Show version of plugin.")
	flag.Parse()

	if args.pollIntervalDisabled {
		args.pollInterval = shim.PollIntervalDisabled
	}

	// create the shim. This is what will run your plugins.
	shimLayer := shim.New()

	if err := shimLayer.LoadConfig(args.configFile); err != nil {
		fmt.Fprintf(os.Stderr, "Err loading input: %s\n", err)
		os.Exit(1)
	}

	// run a single plugin until stdin closes, or we receive a termination signal
	if err := shimLayer.Run(args.pollInterval); err != nil {
		fmt.Fprintf(os.Stderr, "Err: %s\n", err)
		os.Exit(1)
	}

	// scanner := bufio.NewScanner(os.Stdin)
	// for {
	// 	if scanner.Scan() {
	// 		line := scanner.Text()
	// 	}
	// }
}

// https://docs.influxdata.com/telegraf/v1/data_formats/output/json/#batch-format
