package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/app"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/config"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/version"
)

const (
	exitSuccess     = 0
	exitFailure     = 1
	exitInvalidArgs = 2
)

func main() {
	os.Exit(
		run(
			os.Args[1:],
			os.Stdout,
			os.Stderr,
		),
	)
}

func run(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	if len(args) > 0 && args[0] == "benchmark" {
		return runBenchmark(
			args[1:],
			stdout,
			stderr,
		)
	}

	return runServer(
		args,
		stdout,
		stderr,
	)
}

func runServer(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet(
		"homedns-dns",
		flag.ContinueOnError,
	)
	flags.SetOutput(stderr)

	configPath := flags.String(
		"config",
		"",
		"path to the YAML configuration file",
	)

	showVersion := flags.Bool(
		"version",
		false,
		"print version information and exit",
	)

	if err := flags.Parse(args); err != nil {
		return exitInvalidArgs
	}

	if *showVersion {
		fmt.Fprintln(stdout, version.String())
		return exitSuccess
	}

	if flags.NArg() > 0 {
		fmt.Fprintf(
			stderr,
			"unexpected arguments: %v\n",
			flags.Args(),
		)

		return exitInvalidArgs
	}

	if *configPath == "" {
		fmt.Fprintln(
			stderr,
			"configuration path is required; use --config <path>",
		)

		return exitInvalidArgs
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"load configuration: %v\n",
			err,
		)

		return exitFailure
	}

	application, err := app.New(cfg)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"create application: %v\n",
			err,
		)

		return exitFailure
	}

	signalContext, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	if err := application.Run(signalContext); err != nil {
		fmt.Fprintf(
			stderr,
			"run application: %v\n",
			err,
		)

		return exitFailure
	}

	return exitSuccess
}

func runBenchmark(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet(
		"homedns-dns benchmark",
		flag.ContinueOnError,
	)
	flags.SetOutput(stderr)

	configPath := flags.String(
		"config",
		"",
		"path to the YAML configuration file",
	)

	outputDirectory := flags.String(
		"output",
		"benchmarks/runs",
		"directory in which benchmark reports are written",
	)

	if err := flags.Parse(args); err != nil {
		return exitInvalidArgs
	}

	if flags.NArg() > 0 {
		fmt.Fprintf(
			stderr,
			"unexpected benchmark arguments: %v\n",
			flags.Args(),
		)

		return exitInvalidArgs
	}

	if *configPath == "" {
		fmt.Fprintln(
			stderr,
			"configuration path is required; use benchmark --config <path>",
		)

		return exitInvalidArgs
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"load configuration: %v\n",
			err,
		)

		return exitFailure
	}

	signalContext, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	fmt.Fprintln(stdout, "Running DNS benchmark...")

	result, err := app.RunBenchmark(
		signalContext,
		cfg,
		*outputDirectory,
	)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"benchmark failed: %v\n",
			err,
		)

		return exitFailure
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Benchmark completed")
	fmt.Fprintln(stdout)
	fmt.Fprintf(
		stdout,
		"Status: %s\n",
		result.Report.Summary.Status,
	)
	fmt.Fprintf(
		stdout,
		"Requests: %d\n",
		result.Report.Summary.Requests.Attempted,
	)
	fmt.Fprintf(
		stdout,
		"Successful: %d\n",
		result.Report.Summary.Requests.Successful,
	)
	fmt.Fprintf(
		stdout,
		"Failed: %d\n",
		result.Report.Summary.Requests.Failed,
	)
	fmt.Fprintf(
		stdout,
		"Timeouts: %d\n",
		result.Report.Summary.Requests.Timeouts,
	)
	fmt.Fprintln(stdout)
	fmt.Fprintf(
		stdout,
		"Report: %s\n",
		result.Path,
	)

	return exitSuccess
}
