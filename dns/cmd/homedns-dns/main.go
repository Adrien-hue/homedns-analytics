package main

import (
	"flag"
	"fmt"
	"os"

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
		fmt.Fprintf(os.Stderr, "unexpected arguments: %v\n", flag.Args())
		return 2
	}

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "configuration path is required; use --config <path>")
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load configuration: %v\n", err)
		return 1
	}

	fmt.Printf(
		"HomeDNS DNS configuration loaded: dns=%s upstream=%s health=%s\n",
		cfg.DNSAddress(),
		cfg.Upstream.Address,
		cfg.HealthAddress(),
	)

	return 0
}
