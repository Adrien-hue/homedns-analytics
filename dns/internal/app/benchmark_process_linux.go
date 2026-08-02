//go:build linux

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// resolveBenchmarkTargetPID finds the running HomeDNS server process.
//
// The benchmark command runs from the same binary as the server, so matching
// only the executable name is insufficient. The server process is identified
// by its --config argument and by the absence of the benchmark subcommand.
func resolveBenchmarkTargetPID(
	configPath string,
) (int, error) {
	if strings.TrimSpace(configPath) == "" {
		return 0, errors.New(
			"benchmark target configuration path is required",
		)
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, fmt.Errorf(
			"read procfs: %w",
			err,
		)
	}

	currentPID := os.Getpid()

	var matches []int

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, parseErr := strconv.Atoi(
			entry.Name(),
		)
		if parseErr != nil ||
			pid <= 0 ||
			pid == currentPID {
			continue
		}

		commandLine, readErr := os.ReadFile(
			filepath.Join(
				"/proc",
				entry.Name(),
				"cmdline",
			),
		)
		if readErr != nil {
			continue
		}

		arguments := parseProcessCommandLine(
			commandLine,
		)

		if !isHomeDNSServerProcess(
			arguments,
			configPath,
		) {
			continue
		}

		matches = append(
			matches,
			pid,
		)
	}

	switch len(matches) {
	case 0:
		return 0, fmt.Errorf(
			"running HomeDNS server process using configuration %q was not found",
			configPath,
		)

	case 1:
		return matches[0], nil

	default:
		return 0, fmt.Errorf(
			"multiple HomeDNS server processes use configuration %q: %v",
			configPath,
			matches,
		)
	}
}

func parseProcessCommandLine(
	content []byte,
) []string {
	parts := strings.Split(
		string(content),
		"\x00",
	)

	arguments := make(
		[]string,
		0,
		len(parts),
	)

	for _, part := range parts {
		if part == "" {
			continue
		}

		arguments = append(
			arguments,
			part,
		)
	}

	return arguments
}

func isHomeDNSServerProcess(
	arguments []string,
	configPath string,
) bool {
	if len(arguments) == 0 {
		return false
	}

	for _, argument := range arguments[1:] {
		if argument == "benchmark" {
			return false
		}
	}

	for index := 1; index < len(arguments); index++ {
		argument := arguments[index]

		switch {
		case argument == "--config":
			if index+1 >= len(arguments) {
				return false
			}

			return sameConfigurationPath(
				arguments[index+1],
				configPath,
			)

		case strings.HasPrefix(
			argument,
			"--config=",
		):
			return sameConfigurationPath(
				strings.TrimPrefix(
					argument,
					"--config=",
				),
				configPath,
			)
		}
	}

	return false
}

func sameConfigurationPath(
	actual string,
	expected string,
) bool {
	actualPath, actualErr :=
		filepath.Abs(actual)

	expectedPath, expectedErr :=
		filepath.Abs(expected)

	if actualErr == nil &&
		expectedErr == nil {
		return filepath.Clean(actualPath) ==
			filepath.Clean(expectedPath)
	}

	return filepath.Clean(actual) ==
		filepath.Clean(expected)
}
