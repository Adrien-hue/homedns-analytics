package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/app"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/config"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/version"
)

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String(
		"config",
		"",
		"path to the YAML configuration file",
	)

	showVersion := flag.Bool(
		"version",
		false,
		"print version information and exit",
	)

	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return 0
	}

	if flag.NArg() > 0 {
		fmt.Fprintf(
			os.Stderr,
			"unexpected arguments: %v\n",
			flag.Args(),
		)

		return 2
	}

	if *configPath == "" {
		fmt.Fprintln(
			os.Stderr,
			"configuration path is required; use --config <path>",
		)

		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"load configuration: %v\n",
			err,
		)

		return 1
	}

	application, err := app.New(cfg)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"create application: %v\n",
			err,
		)

		return 1
	}

	signalContext, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	if err := application.Run(signalContext); err != nil {
		return 1
	}

	return 0
}
