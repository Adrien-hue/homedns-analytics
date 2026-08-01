package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/miekg/dns"

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
	if len(args) > 0 &&
		args[0] == "benchmark" {
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
		fmt.Fprintln(
			stdout,
			version.String(),
		)

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

	signalContext, stopSignals :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
	defer stopSignals()

	if err := application.Run(
		signalContext,
	); err != nil {
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

	profileName := flags.String(
		"profile",
		app.DefaultBenchmarkProfile,
		"benchmark profile: quick, validation, or endurance",
	)

	outputDirectory := flags.String(
		"output",
		app.DefaultBenchmarkOutputDirectory,
		"directory in which benchmark reports are written",
	)

	queryCount := flags.Int(
		"queries",
		0,
		"number of measured queries per path",
	)

	warmupQueries := flags.Int(
		"warmup",
		0,
		"number of warmup queries per path",
	)

	concurrentWorkers := flags.Int(
		"concurrency",
		0,
		"number of workers used by concurrent scenarios",
	)

	timeout := flags.Duration(
		"timeout",
		0,
		"timeout for each DNS query, for example 3s",
	)

	queryTypeName := flags.String(
		"query-type",
		"",
		"DNS query type: A or AAAA",
	)

	domains := flags.String(
		"domains",
		"",
		"comma-separated domain names to query",
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

	profile, err := app.ResolveBenchmarkProfile(
		*profileName,
	)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"invalid benchmark profile: %v\n",
			err,
		)

		return exitInvalidArgs
	}

	options := app.DefaultBenchmarkOptions()

	options.Profile = profile.Name
	options.OutputDirectory =
		*outputDirectory
	options.QueryCount =
		profile.QueryCount
	options.WarmupQueries =
		profile.WarmupQueries
	options.ConcurrentWorkers =
		profile.ConcurrentWorkers

	explicitFlags := make(
		map[string]bool,
	)

	flags.Visit(
		func(currentFlag *flag.Flag) {
			explicitFlags[currentFlag.Name] = true
		},
	)

	if explicitFlags["queries"] {
		options.QueryCount =
			*queryCount

		options.ProfileCustomized =
			true
	}

	if explicitFlags["warmup"] {
		options.WarmupQueries =
			*warmupQueries

		options.ProfileCustomized =
			true
	}

	if explicitFlags["concurrency"] {
		options.ConcurrentWorkers =
			*concurrentWorkers

		options.ProfileCustomized =
			true
	}

	if explicitFlags["timeout"] {
		if *timeout <= 0 {
			fmt.Fprintln(
				stderr,
				"benchmark timeout must be greater than zero",
			)

			return exitInvalidArgs
		}

		options.Timeout = *timeout
		options.ProfileCustomized =
			true
	}

	if explicitFlags["query-type"] {
		queryType, err :=
			parseBenchmarkQueryType(
				*queryTypeName,
			)
		if err != nil {
			fmt.Fprintf(
				stderr,
				"invalid benchmark query type: %v\n",
				err,
			)

			return exitInvalidArgs
		}

		options.QueryType =
			queryType

		options.ProfileCustomized =
			true
	}

	if explicitFlags["domains"] {
		queryNames, err :=
			parseBenchmarkDomains(
				*domains,
			)
		if err != nil {
			fmt.Fprintf(
				stderr,
				"invalid benchmark domains: %v\n",
				err,
			)

			return exitInvalidArgs
		}

		options.QueryNames =
			queryNames

		options.ProfileCustomized =
			true
	}

	if options.QueryCount <= 0 {
		fmt.Fprintln(
			stderr,
			"benchmark query count must be greater than zero",
		)

		return exitInvalidArgs
	}

	if options.WarmupQueries < 0 {
		fmt.Fprintln(
			stderr,
			"benchmark warmup query count cannot be negative",
		)

		return exitInvalidArgs
	}

	if options.ConcurrentWorkers <= 0 {
		fmt.Fprintln(
			stderr,
			"benchmark concurrency must be greater than zero",
		)

		return exitInvalidArgs
	}

	if options.ConcurrentWorkers >
		options.QueryCount {
		fmt.Fprintf(
			stderr,
			"benchmark concurrency %d cannot exceed query count %d\n",
			options.ConcurrentWorkers,
			options.QueryCount,
		)

		return exitInvalidArgs
	}

	cfg, err := config.Load(
		*configPath,
	)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"load configuration: %v\n",
			err,
		)

		return exitFailure
	}

	signalContext, stopSignals :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
	defer stopSignals()

	effectiveTimeout :=
		options.Timeout

	if effectiveTimeout == 0 {
		effectiveTimeout =
			cfg.Upstream.Timeout
	}

	fmt.Fprintln(
		stdout,
		"Running DNS benchmark...",
	)
	fmt.Fprintln(stdout)

	fmt.Fprintf(
		stdout,
		"Profile: %s\n",
		options.Profile,
	)

	fmt.Fprintf(
		stdout,
		"Customized: %t\n",
		options.ProfileCustomized,
	)

	fmt.Fprintf(
		stdout,
		"Queries per path: %d\n",
		options.QueryCount,
	)

	fmt.Fprintf(
		stdout,
		"Warmup queries: %d\n",
		options.WarmupQueries,
	)

	fmt.Fprintf(
		stdout,
		"Concurrent workers: %d\n",
		options.ConcurrentWorkers,
	)

	fmt.Fprintf(
		stdout,
		"Query type: %s\n",
		dns.TypeToString[options.QueryType],
	)

	fmt.Fprintf(
		stdout,
		"Domains: %s\n",
		strings.Join(
			options.QueryNames,
			", ",
		),
	)

	fmt.Fprintf(
		stdout,
		"Timeout: %s\n",
		effectiveTimeout,
	)

	fmt.Fprintf(
		stdout,
		"Output: %s\n",
		options.OutputDirectory,
	)

	fmt.Fprintln(stdout)

	options.ProgressReporter =
		newTerminalProgressReporter(
			stdout,
			options.QueryCount,
		)

	result, err := app.RunBenchmark(
		signalContext,
		cfg,
		options,
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
	fmt.Fprintln(
		stdout,
		"Benchmark completed",
	)
	fmt.Fprintln(stdout)

	fmt.Fprintf(
		stdout,
		"Status: %s\n",
		result.Report.Summary.Status,
	)

	fmt.Fprintf(
		stdout,
		"Requests: %d\n",
		result.Report.
			Summary.
			Requests.
			Attempted,
	)

	fmt.Fprintf(
		stdout,
		"Successful: %d\n",
		result.Report.
			Summary.
			Requests.
			Successful,
	)

	fmt.Fprintf(
		stdout,
		"Failed: %d\n",
		result.Report.
			Summary.
			Requests.
			Failed,
	)

	fmt.Fprintf(
		stdout,
		"Timeouts: %d\n",
		result.Report.
			Summary.
			Requests.
			Timeouts,
	)

	fmt.Fprintln(stdout)

	fmt.Fprintf(
		stdout,
		"Attempted QPS: %.3f\n",
		result.Report.
			Summary.
			Throughput.
			AttemptedQueriesPerSecond,
	)

	fmt.Fprintf(
		stdout,
		"Successful QPS: %.3f\n",
		result.Report.
			Summary.
			Throughput.
			SuccessfulQueriesPerSecond,
	)

	fmt.Fprintln(stdout)

	fmt.Fprintf(
		stdout,
		"Report: %s\n",
		result.Path,
	)

	return exitSuccess
}

func parseBenchmarkQueryType(
	value string,
) (uint16, error) {
	switch strings.ToUpper(
		strings.TrimSpace(value),
	) {
	case "A":
		return dns.TypeA, nil

	case "AAAA":
		return dns.TypeAAAA, nil

	default:
		return 0, fmt.Errorf(
			"unsupported query type %q; supported types are A and AAAA",
			value,
		)
	}
}

func parseBenchmarkDomains(
	value string,
) ([]string, error) {
	parts := strings.Split(
		value,
		",",
	)

	queryNames := make(
		[]string,
		0,
		len(parts),
	)

	for index, part := range parts {
		queryName :=
			strings.TrimSpace(part)

		if queryName == "" {
			return nil, fmt.Errorf(
				"domain at index %d is empty",
				index,
			)
		}

		queryNames = append(
			queryNames,
			queryName,
		)
	}

	if len(queryNames) == 0 {
		return nil, fmt.Errorf(
			"at least one domain is required",
		)
	}

	return queryNames, nil
}
