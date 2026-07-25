package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/version"
)

func main() {
	showVersion := flag.Bool(
		"version",
		false,
		"print version information and exit",
	)

	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected arguments: %v\n", flag.Args())
		os.Exit(2)
	}

	fmt.Println("HomeDNS DNS service")
}
