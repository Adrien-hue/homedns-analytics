//go:build linux

package app

import (
	"testing"
)

func TestParseProcessCommandLine(
	t *testing.T,
) {
	t.Parallel()

	actual := parseProcessCommandLine(
		[]byte(
			"/opt/homedns/current/homedns-dns\x00" +
				"--config\x00" +
				"/opt/homedns/shared/config/dns.yaml\x00",
		),
	)

	expected := []string{
		"/opt/homedns/current/homedns-dns",
		"--config",
		"/opt/homedns/shared/config/dns.yaml",
	}

	if len(actual) != len(expected) {
		t.Fatalf(
			"unexpected argument count: got %d, want %d",
			len(actual),
			len(expected),
		)
	}

	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf(
				"unexpected argument at index %d: got %q, want %q",
				index,
				actual[index],
				expected[index],
			)
		}
	}
}

func TestParseProcessCommandLineIgnoresEmptyValues(
	t *testing.T,
) {
	t.Parallel()

	actual := parseProcessCommandLine(
		[]byte(
			"homedns-dns\x00\x00--config\x00dns.yaml\x00",
		),
	)

	expected := []string{
		"homedns-dns",
		"--config",
		"dns.yaml",
	}

	if len(actual) != len(expected) {
		t.Fatalf(
			"unexpected argument count: got %d, want %d",
			len(actual),
			len(expected),
		)
	}

	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf(
				"unexpected argument at index %d: got %q, want %q",
				index,
				actual[index],
				expected[index],
			)
		}
	}
}

func TestIsHomeDNSServerProcessWithSeparateConfigArgument(
	t *testing.T,
) {
	t.Parallel()

	arguments := []string{
		"/opt/homedns/current/homedns-dns",
		"--config",
		"/opt/homedns/shared/config/dns.yaml",
	}

	if !isHomeDNSServerProcess(
		arguments,
		"/opt/homedns/shared/config/dns.yaml",
	) {
		t.Fatal(
			"expected HomeDNS server process to match",
		)
	}
}

func TestIsHomeDNSServerProcessWithCombinedConfigArgument(
	t *testing.T,
) {
	t.Parallel()

	arguments := []string{
		"/opt/homedns/current/homedns-dns",
		"--config=/opt/homedns/shared/config/dns.yaml",
	}

	if !isHomeDNSServerProcess(
		arguments,
		"/opt/homedns/shared/config/dns.yaml",
	) {
		t.Fatal(
			"expected HomeDNS server process to match",
		)
	}
}

func TestIsHomeDNSServerProcessRejectsBenchmarkCommand(
	t *testing.T,
) {
	t.Parallel()

	arguments := []string{
		"/opt/homedns/current/homedns-dns",
		"benchmark",
		"--config",
		"/opt/homedns/shared/config/dns.yaml",
	}

	if isHomeDNSServerProcess(
		arguments,
		"/opt/homedns/shared/config/dns.yaml",
	) {
		t.Fatal(
			"benchmark process must not be identified as the server",
		)
	}
}

func TestIsHomeDNSServerProcessRejectsDifferentConfig(
	t *testing.T,
) {
	t.Parallel()

	arguments := []string{
		"/opt/homedns/current/homedns-dns",
		"--config",
		"/tmp/another-config.yaml",
	}

	if isHomeDNSServerProcess(
		arguments,
		"/opt/homedns/shared/config/dns.yaml",
	) {
		t.Fatal(
			"process using another config must not match",
		)
	}
}

func TestIsHomeDNSServerProcessRejectsMissingConfigValue(
	t *testing.T,
) {
	t.Parallel()

	arguments := []string{
		"/opt/homedns/current/homedns-dns",
		"--config",
	}

	if isHomeDNSServerProcess(
		arguments,
		"/opt/homedns/shared/config/dns.yaml",
	) {
		t.Fatal(
			"process with missing config value must not match",
		)
	}
}

func TestIsHomeDNSServerProcessRejectsMissingArguments(
	t *testing.T,
) {
	t.Parallel()

	if isHomeDNSServerProcess(
		nil,
		"/opt/homedns/shared/config/dns.yaml",
	) {
		t.Fatal(
			"empty process arguments must not match",
		)
	}
}

func TestIsHomeDNSServerProcessRejectsMissingConfigFlag(
	t *testing.T,
) {
	t.Parallel()

	arguments := []string{
		"/opt/homedns/current/homedns-dns",
	}

	if isHomeDNSServerProcess(
		arguments,
		"/opt/homedns/shared/config/dns.yaml",
	) {
		t.Fatal(
			"process without config flag must not match",
		)
	}
}

func TestSameConfigurationPath(
	t *testing.T,
) {
	t.Parallel()

	if !sameConfigurationPath(
		"/opt/homedns/shared/config/../config/dns.yaml",
		"/opt/homedns/shared/config/dns.yaml",
	) {
		t.Fatal(
			"equivalent configuration paths should match",
		)
	}
}

func TestSameConfigurationPathRejectsDifferentPaths(
	t *testing.T,
) {
	t.Parallel()

	if sameConfigurationPath(
		"/opt/homedns/shared/config/dns.yaml",
		"/tmp/dns.yaml",
	) {
		t.Fatal(
			"different configuration paths must not match",
		)
	}
}
