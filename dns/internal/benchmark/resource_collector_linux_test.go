//go:build linux

package benchmark

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLinuxResourceCollectorReadProcessStat(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		readFile: func(
			path string,
		) ([]byte, error) {
			expected := filepath.Join(
				procRoot,
				"123",
				"stat",
			)

			if path != expected {
				t.Fatalf(
					"unexpected path: got %q, want %q",
					path,
					expected,
				)
			}

			return []byte(
				"123 (homedns-dns worker) " +
					"S 1 2 3 4 5 6 7 8 9 10 " +
					"120 340 0 0 0\n",
			), nil
		},
	}

	result, err := collector.readProcessStat(
		123,
	)
	if err != nil {
		t.Fatalf(
			"read process stat: %v",
			err,
		)
	}

	if result.UserTicks != 120 {
		t.Fatalf(
			"unexpected user ticks: got %d, want 120",
			result.UserTicks,
		)
	}

	if result.SystemTicks != 340 {
		t.Fatalf(
			"unexpected system ticks: got %d, want 340",
			result.SystemTicks,
		)
	}
}

func TestLinuxResourceCollectorReadProcessStatRejectsInvalidContent(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name:          "missing closing parenthesis",
			content:       "123 (homedns-dns S 1 2 3",
			expectedError: "malformed process stat",
		},
		{
			name:          "insufficient fields",
			content:       "123 (homedns-dns) S 1 2",
			expectedError: "process stat has insufficient fields",
		},
		{
			name: "invalid user ticks",
			content: "123 (homedns-dns) " +
				"S 1 2 3 4 5 6 7 8 9 10 nope 340",
			expectedError: "parse user CPU ticks",
		},
		{
			name: "invalid system ticks",
			content: "123 (homedns-dns) " +
				"S 1 2 3 4 5 6 7 8 9 10 120 nope",
			expectedError: "parse system CPU ticks",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				collector :=
					&LinuxResourceCollector{
						readFile: func(
							string,
						) ([]byte, error) {
							return []byte(
								test.content,
							), nil
						},
					}

				_, err :=
					collector.
						readProcessStat(123)

				if err == nil {
					t.Fatal(
						"expected process stat parsing to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.expectedError,
				) {
					t.Fatalf(
						"unexpected error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestLinuxResourceCollectorReadProcessStatReturnsReadError(
	t *testing.T,
) {
	t.Parallel()

	expectedErr := errors.New(
		"read failed",
	)

	collector := &LinuxResourceCollector{
		readFile: func(
			string,
		) ([]byte, error) {
			return nil, expectedErr
		},
	}

	_, err := collector.readProcessStat(
		123,
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"unexpected error: got %v, want %v",
			err,
			expectedErr,
		)
	}
}

func TestLinuxResourceCollectorReadProcessStatus(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		readFile: func(
			path string,
		) ([]byte, error) {
			expected := filepath.Join(
				procRoot,
				"123",
				"status",
			)

			if path != expected {
				t.Fatalf(
					"unexpected path: got %q, want %q",
					path,
					expected,
				)
			}

			return []byte(
				"Name:\thomedns-dns\n" +
					"VmRSS:\t20480 kB\n" +
					"Threads:\t7\n",
			), nil
		},
	}

	result, err :=
		collector.readProcessStatus(
			123,
		)
	if err != nil {
		t.Fatalf(
			"read process status: %v",
			err,
		)
	}

	expectedRSS :=
		uint64(20_480 * 1024)

	if result.RSSBytes != expectedRSS {
		t.Fatalf(
			"unexpected RSS: got %d, want %d",
			result.RSSBytes,
			expectedRSS,
		)
	}

	if result.Threads != 7 {
		t.Fatalf(
			"unexpected thread count: got %d, want 7",
			result.Threads,
		)
	}
}

func TestLinuxResourceCollectorReadProcessStatusRejectsInvalidContent(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name:          "missing RSS",
			content:       "Name:\thomedns-dns\nThreads:\t7\n",
			expectedError: "VmRSS is missing",
		},
		{
			name:          "missing threads",
			content:       "Name:\thomedns-dns\nVmRSS:\t20480 kB\n",
			expectedError: "Threads is missing",
		},
		{
			name:          "malformed RSS",
			content:       "VmRSS:\tinvalid kB\nThreads:\t7\n",
			expectedError: "parse procfs memory value",
		},
		{
			name:          "malformed threads",
			content:       "VmRSS:\t20480 kB\nThreads:\tinvalid\n",
			expectedError: "parse thread count",
		},
		{
			name:          "malformed threads field",
			content:       "VmRSS:\t20480 kB\nThreads:\n",
			expectedError: "malformed Threads field",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				collector :=
					&LinuxResourceCollector{
						readFile: func(
							string,
						) ([]byte, error) {
							return []byte(
								test.content,
							), nil
						},
					}

				_, err :=
					collector.
						readProcessStatus(123)

				if err == nil {
					t.Fatal(
						"expected process status parsing to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.expectedError,
				) {
					t.Fatalf(
						"unexpected error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestLinuxResourceCollectorReadSystemCPUTicks(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		readFile: func(
			path string,
		) ([]byte, error) {
			expected := filepath.Join(
				procRoot,
				"stat",
			)

			if path != expected {
				t.Fatalf(
					"unexpected path: got %q, want %q",
					path,
					expected,
				)
			}

			return []byte(
				"cpu 100 200 300 400 500 600 700 800\n" +
					"cpu0 10 20 30 40\n",
			), nil
		},
	}

	result, err :=
		collector.readSystemCPUTicks()
	if err != nil {
		t.Fatalf(
			"read system CPU ticks: %v",
			err,
		)
	}

	if result != 3600 {
		t.Fatalf(
			"unexpected CPU ticks: got %d, want 3600",
			result,
		)
	}
}

func TestLinuxResourceCollectorReadSystemCPUTicksRejectsInvalidContent(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name:          "malformed line",
			content:       "cpu\n",
			expectedError: "malformed aggregate CPU stat",
		},
		{
			name:          "wrong prefix",
			content:       "cpu0 100 200\n",
			expectedError: "malformed aggregate CPU stat",
		},
		{
			name:          "invalid tick",
			content:       "cpu 100 invalid 300\n",
			expectedError: "parse aggregate CPU ticks",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				collector :=
					&LinuxResourceCollector{
						readFile: func(
							string,
						) ([]byte, error) {
							return []byte(
								test.content,
							), nil
						},
					}

				_, err :=
					collector.
						readSystemCPUTicks()

				if err == nil {
					t.Fatal(
						"expected CPU stat parsing to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.expectedError,
				) {
					t.Fatalf(
						"unexpected error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestLinuxResourceCollectorReadLoadAverage(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		readFile: func(
			path string,
		) ([]byte, error) {
			expected := filepath.Join(
				procRoot,
				"loadavg",
			)

			if path != expected {
				t.Fatalf(
					"unexpected path: got %q, want %q",
					path,
					expected,
				)
			}

			return []byte(
				"0.42 0.33 0.11 1/123 456\n",
			), nil
		},
	}

	result, err :=
		collector.readLoadAverage()
	if err != nil {
		t.Fatalf(
			"read load average: %v",
			err,
		)
	}

	if result != 0.42 {
		t.Fatalf(
			"unexpected load average: got %f, want 0.42",
			result,
		)
	}
}

func TestLinuxResourceCollectorReadLoadAverageRejectsInvalidContent(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name:          "empty",
			content:       "",
			expectedError: "load average is empty",
		},
		{
			name:          "invalid float",
			content:       "invalid 0.33 0.11",
			expectedError: "parse load average",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				collector :=
					&LinuxResourceCollector{
						readFile: func(
							string,
						) ([]byte, error) {
							return []byte(
								test.content,
							), nil
						},
					}

				_, err :=
					collector.
						readLoadAverage()

				if err == nil {
					t.Fatal(
						"expected load average parsing to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.expectedError,
				) {
					t.Fatalf(
						"unexpected error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestLinuxResourceCollectorReadAvailableMemory(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		readFile: func(
			path string,
		) ([]byte, error) {
			expected := filepath.Join(
				procRoot,
				"meminfo",
			)

			if path != expected {
				t.Fatalf(
					"unexpected path: got %q, want %q",
					path,
					expected,
				)
			}

			return []byte(
				"MemTotal:       100000 kB\n" +
					"MemAvailable:    20000 kB\n",
			), nil
		},
	}

	result, err :=
		collector.readAvailableMemory()
	if err != nil {
		t.Fatalf(
			"read available memory: %v",
			err,
		)
	}

	expected :=
		uint64(20_000 * 1024)

	if result != expected {
		t.Fatalf(
			"unexpected available memory: got %d, want %d",
			result,
			expected,
		)
	}
}

func TestLinuxResourceCollectorReadAvailableMemoryRejectsInvalidContent(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name:          "missing field",
			content:       "MemTotal: 100000 kB\n",
			expectedError: "MemAvailable is missing",
		},
		{
			name:          "invalid value",
			content:       "MemAvailable: invalid kB\n",
			expectedError: "parse procfs memory value",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				collector :=
					&LinuxResourceCollector{
						readFile: func(
							string,
						) ([]byte, error) {
							return []byte(
								test.content,
							), nil
						},
					}

				_, err :=
					collector.
						readAvailableMemory()

				if err == nil {
					t.Fatal(
						"expected available memory parsing to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.expectedError,
				) {
					t.Fatalf(
						"unexpected error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestLinuxResourceCollectorReadTemperature(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		readFile: func(
			path string,
		) ([]byte, error) {
			if path != thermalPath {
				t.Fatalf(
					"unexpected path: got %q, want %q",
					path,
					thermalPath,
				)
			}

			return []byte(
				"52341\n",
			), nil
		},
	}

	result, err :=
		collector.readTemperature()
	if err != nil {
		t.Fatalf(
			"read temperature: %v",
			err,
		)
	}

	if result != 52.341 {
		t.Fatalf(
			"unexpected temperature: got %f, want 52.341",
			result,
		)
	}
}

func TestLinuxResourceCollectorReadTemperatureRejectsInvalidContent(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		readFile: func(
			string,
		) ([]byte, error) {
			return []byte(
				"invalid\n",
			), nil
		},
	}

	_, err := collector.readTemperature()
	if err == nil {
		t.Fatal(
			"expected temperature parsing to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"parse thermal value",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestLinuxResourceCollectorReadThrottledState(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		runCommand: func(
			_ context.Context,
			name string,
			args ...string,
		) ([]byte, error) {
			if name != "vcgencmd" {
				t.Fatalf(
					"unexpected command: %q",
					name,
				)
			}

			if len(args) != 1 ||
				args[0] != "get_throttled" {
				t.Fatalf(
					"unexpected arguments: %v",
					args,
				)
			}

			return []byte(
				"throttled=0x0\n",
			), nil
		},
	}

	result, err :=
		collector.readThrottledState(
			context.Background(),
		)
	if err != nil {
		t.Fatalf(
			"read throttled state: %v",
			err,
		)
	}

	if result != "throttled=0x0" {
		t.Fatalf(
			"unexpected throttled state: got %q, want %q",
			result,
			"throttled=0x0",
		)
	}
}

func TestLinuxResourceCollectorReadThrottledStateRejectsEmptyOutput(
	t *testing.T,
) {
	t.Parallel()

	collector := &LinuxResourceCollector{
		runCommand: func(
			context.Context,
			string,
			...string,
		) ([]byte, error) {
			return []byte(
				" \n",
			), nil
		},
	}

	_, err := collector.readThrottledState(
		context.Background(),
	)
	if err == nil {
		t.Fatal(
			"expected empty throttling state to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"empty throttling state",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestLinuxResourceCollectorReadThrottledStateReturnsCommandError(
	t *testing.T,
) {
	t.Parallel()

	expectedErr := errors.New(
		"command failed",
	)

	collector := &LinuxResourceCollector{
		runCommand: func(
			context.Context,
			string,
			...string,
		) ([]byte, error) {
			return nil, expectedErr
		},
	}

	_, err := collector.readThrottledState(
		context.Background(),
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"unexpected error: got %v, want %v",
			err,
			expectedErr,
		)
	}
}

func TestParseProcKilobytes(
	t *testing.T,
) {
	t.Parallel()

	result, err := parseProcKilobytes(
		"VmRSS: 20480 kB",
	)
	if err != nil {
		t.Fatalf(
			"parse proc kilobytes: %v",
			err,
		)
	}

	if result != 20_480 {
		t.Fatalf(
			"unexpected value: got %d, want 20480",
			result,
		)
	}
}

func TestParseProcKilobytesRejectsInvalidContent(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name:          "missing value",
			content:       "VmRSS:",
			expectedError: "malformed procfs memory field",
		},
		{
			name:          "invalid value",
			content:       "VmRSS: invalid kB",
			expectedError: "parse procfs memory value",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				_, err :=
					parseProcKilobytes(
						test.content,
					)

				if err == nil {
					t.Fatal(
						"expected proc value parsing to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.expectedError,
				) {
					t.Fatalf(
						"unexpected error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestCPUPercentFromTicks(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name         string
		processDelta uint64
		systemDelta  uint64
		expected     float64
	}{
		{
			name:         "one percent",
			processDelta: 10,
			systemDelta:  1000,
			expected:     1,
		},
		{
			name:         "five percent",
			processDelta: 25,
			systemDelta:  500,
			expected:     5,
		},
		{
			name:         "zero process activity",
			processDelta: 0,
			systemDelta:  100,
			expected:     0,
		},
		{
			name:         "zero system delta",
			processDelta: 100,
			systemDelta:  0,
			expected:     0,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual :=
					cpuPercentFromTicks(
						test.processDelta,
						test.systemDelta,
					)

				if actual != test.expected {
					t.Fatalf(
						"unexpected CPU percent: got %f, want %f",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestLinuxResourceCollectorCollectSample(
	t *testing.T,
) {
	t.Parallel()

	files := map[string]string{
		filepath.Join(
			procRoot,
			"100",
			"stat",
		): processStatFixture(
			100,
			"benchmark",
			120,
			30,
		),

		filepath.Join(
			procRoot,
			"100",
			"status",
		): "VmRSS: 10000 kB\n" +
			"Threads: 8\n",

		filepath.Join(
			procRoot,
			"200",
			"stat",
		): processStatFixture(
			200,
			"homedns-dns",
			220,
			40,
		),

		filepath.Join(
			procRoot,
			"200",
			"status",
		): "VmRSS: 20000 kB\n" +
			"Threads: 7\n",

		filepath.Join(
			procRoot,
			"stat",
		): "cpu 100 100 100 100 100\n",

		filepath.Join(
			procRoot,
			"loadavg",
		): "0.42 0.33 0.11 1/10 123\n",

		filepath.Join(
			procRoot,
			"meminfo",
		): "MemAvailable: 300000 kB\n",

		thermalPath: "51000\n",
	}

	now := time.Date(
		2026,
		time.August,
		1,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	collector := &LinuxResourceCollector{
		now: func() time.Time {
			return now
		},

		readFile: func(
			path string,
		) ([]byte, error) {
			content, exists := files[path]
			if !exists {
				return nil, fmt.Errorf(
					"unexpected path: %s",
					path,
				)
			}

			return []byte(content), nil
		},

		runCommand: func(
			context.Context,
			string,
			...string,
		) ([]byte, error) {
			return []byte(
				"throttled=0x0\n",
			), nil
		},
	}

	first, snapshot :=
		collector.collectSample(
			context.Background(),
			ResourceCollectionConfig{
				BenchmarkPID: 100,
				TargetPID:    200,
				Interval:     time.Second,
			},
			processTickSnapshot{},
		)

	if first.BenchmarkProcess.PID != 100 {
		t.Fatalf(
			"unexpected benchmark PID: %d",
			first.BenchmarkProcess.PID,
		)
	}

	if first.TargetService.PID != 200 {
		t.Fatalf(
			"unexpected target PID: %d",
			first.TargetService.PID,
		)
	}

	if first.BenchmarkProcess.CPUPercent != nil {
		t.Fatal(
			"first benchmark CPU sample should be nil",
		)
	}

	if first.TargetService.CPUPercent != nil {
		t.Fatal(
			"first target CPU sample should be nil",
		)
	}

	if first.BenchmarkProcess.RSSBytes == nil ||
		*first.BenchmarkProcess.RSSBytes !=
			10_000*1024 {
		t.Fatalf(
			"unexpected benchmark RSS: %v",
			first.BenchmarkProcess.RSSBytes,
		)
	}

	if first.TargetService.RSSBytes == nil ||
		*first.TargetService.RSSBytes !=
			20_000*1024 {
		t.Fatalf(
			"unexpected target RSS: %v",
			first.TargetService.RSSBytes,
		)
	}

	if first.System.LoadAverageOneMinute == nil ||
		*first.System.LoadAverageOneMinute != 0.42 {
		t.Fatalf(
			"unexpected load average: %v",
			first.System.LoadAverageOneMinute,
		)
	}

	if first.System.MemoryAvailableBytes == nil ||
		*first.System.MemoryAvailableBytes !=
			300_000*1024 {
		t.Fatalf(
			"unexpected available memory: %v",
			first.System.MemoryAvailableBytes,
		)
	}

	if first.System.TemperatureCelsius == nil ||
		*first.System.TemperatureCelsius != 51 {
		t.Fatalf(
			"unexpected temperature: %v",
			first.System.TemperatureCelsius,
		)
	}

	if first.System.ThrottledState == nil ||
		*first.System.ThrottledState !=
			"throttled=0x0" {
		t.Fatalf(
			"unexpected throttling state: %v",
			first.System.ThrottledState,
		)
	}

	files[filepath.Join(
		procRoot,
		"100",
		"stat",
	)] = processStatFixture(
		100,
		"benchmark",
		125,
		35,
	)

	files[filepath.Join(
		procRoot,
		"200",
		"stat",
	)] = processStatFixture(
		200,
		"homedns-dns",
		230,
		50,
	)

	files[filepath.Join(
		procRoot,
		"stat",
	)] =
		"cpu 200 200 200 200 200\n"

	second, _ :=
		collector.collectSample(
			context.Background(),
			ResourceCollectionConfig{
				BenchmarkPID: 100,
				TargetPID:    200,
				Interval:     time.Second,
			},
			snapshot,
		)

	if second.BenchmarkProcess.CPUPercent == nil {
		t.Fatal(
			"benchmark CPU percentage is nil",
		)
	}

	if *second.BenchmarkProcess.CPUPercent !=
		2 {
		t.Fatalf(
			"unexpected benchmark CPU: got %f, want 2",
			*second.BenchmarkProcess.CPUPercent,
		)
	}

	if second.TargetService.CPUPercent == nil {
		t.Fatal(
			"target CPU percentage is nil",
		)
	}

	if *second.TargetService.CPUPercent !=
		4 {
		t.Fatalf(
			"unexpected target CPU: got %f, want 4",
			*second.TargetService.CPUPercent,
		)
	}
}

func TestLinuxResourceCollectorCollect(
	t *testing.T,
) {
	t.Parallel()

	var mutex sync.Mutex

	systemTick := uint64(1000)
	benchmarkTick := uint64(100)
	targetTick := uint64(200)
	clockOffset := time.Duration(0)

	baseTime := time.Date(
		2026,
		time.August,
		1,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	collector := &LinuxResourceCollector{
		now: func() time.Time {
			mutex.Lock()
			defer mutex.Unlock()

			clockOffset += time.Millisecond

			return baseTime.Add(
				clockOffset,
			)
		},

		readFile: func(
			path string,
		) ([]byte, error) {
			mutex.Lock()
			defer mutex.Unlock()

			switch path {
			case filepath.Join(
				procRoot,
				"stat",
			):
				systemTick += 100

				return []byte(
					fmt.Sprintf(
						"cpu %d\n",
						systemTick,
					),
				), nil

			case filepath.Join(
				procRoot,
				"100",
				"stat",
			):
				benchmarkTick += 5

				return []byte(
					processStatFixture(
						100,
						"benchmark",
						benchmarkTick,
						0,
					),
				), nil

			case filepath.Join(
				procRoot,
				"100",
				"status",
			):
				return []byte(
					"VmRSS: 10000 kB\n" +
						"Threads: 8\n",
				), nil

			case filepath.Join(
				procRoot,
				"200",
				"stat",
			):
				targetTick += 10

				return []byte(
					processStatFixture(
						200,
						"homedns-dns",
						targetTick,
						0,
					),
				), nil

			case filepath.Join(
				procRoot,
				"200",
				"status",
			):
				return []byte(
					"VmRSS: 20000 kB\n" +
						"Threads: 7\n",
				), nil

			case filepath.Join(
				procRoot,
				"loadavg",
			):
				return []byte(
					"0.42 0.33 0.11 1/10 123\n",
				), nil

			case filepath.Join(
				procRoot,
				"meminfo",
			):
				return []byte(
					"MemAvailable: 300000 kB\n",
				), nil

			case thermalPath:
				return []byte(
					"51000\n",
				), nil

			default:
				return nil, fmt.Errorf(
					"unexpected path: %s",
					path,
				)
			}
		},

		runCommand: func(
			context.Context,
			string,
			...string,
		) ([]byte, error) {
			return []byte(
				"throttled=0x0\n",
			), nil
		},
	}

	result, err := collector.Collect(
		context.Background(),
		ResourceCollectionConfig{
			BenchmarkPID: 100,
			TargetPID:    200,
			Interval:     5 * time.Millisecond,
		},
		func(context.Context) error {
			time.Sleep(
				18 * time.Millisecond,
			)

			return nil
		},
	)
	if err != nil {
		t.Fatalf(
			"collect resources: %v",
			err,
		)
	}

	if result.StartedAt.IsZero() ||
		result.EndedAt.IsZero() {
		t.Fatal(
			"collection boundaries are missing",
		)
	}

	if result.BenchmarkPIDBefore != 100 ||
		result.BenchmarkPIDAfter != 100 {
		t.Fatalf(
			"unexpected benchmark PIDs: before=%d after=%d",
			result.BenchmarkPIDBefore,
			result.BenchmarkPIDAfter,
		)
	}

	if result.TargetPIDBefore != 200 ||
		result.TargetPIDAfter != 200 {
		t.Fatalf(
			"unexpected target PIDs: before=%d after=%d",
			result.TargetPIDBefore,
			result.TargetPIDAfter,
		)
	}

	if result.Before.BenchmarkProcess.RSSBytes == nil {
		t.Fatal(
			"before benchmark RSS is nil",
		)
	}

	if result.After.TargetService.RSSBytes == nil {
		t.Fatal(
			"after target RSS is nil",
		)
	}

	if len(result.Samples) == 0 {
		t.Fatal(
			"expected periodic resource samples",
		)
	}

	for index, sample := range result.Samples {
		if sample.BenchmarkProcess.CPUPercent == nil {
			t.Fatalf(
				"benchmark CPU missing in sample %d",
				index,
			)
		}

		if sample.TargetService.CPUPercent == nil {
			t.Fatalf(
				"target CPU missing in sample %d",
				index,
			)
		}
	}
}

func TestLinuxResourceCollectorCollectReturnsBenchmarkError(
	t *testing.T,
) {
	t.Parallel()

	expectedErr := errors.New(
		"benchmark failed",
	)

	collector :=
		workingLinuxResourceCollectorForTest()

	_, err := collector.Collect(
		context.Background(),
		ResourceCollectionConfig{
			BenchmarkPID: 100,
			TargetPID:    200,
			Interval:     time.Hour,
		},
		func(context.Context) error {
			return expectedErr
		},
	)

	if !errors.Is(
		err,
		expectedErr,
	) {
		t.Fatalf(
			"unexpected error: got %v, want %v",
			err,
			expectedErr,
		)
	}
}

func TestLinuxResourceCollectorCollectValidatesDependencies(
	t *testing.T,
) {
	t.Parallel()

	validConfig := ResourceCollectionConfig{
		BenchmarkPID: 100,
		TargetPID:    200,
		Interval:     time.Second,
	}

	tests := []struct {
		name          string
		collector     *LinuxResourceCollector
		expectedError string
	}{
		{
			name:          "nil collector",
			collector:     nil,
			expectedError: "Linux resource collector is not initialized",
		},
		{
			name: "missing clock",
			collector: &LinuxResourceCollector{
				readFile: func(string) ([]byte, error) {
					return nil, nil
				},

				runCommand: func(
					context.Context,
					string,
					...string,
				) ([]byte, error) {
					return nil, nil
				},
			},
			expectedError: "resource collector clock is not initialized",
		},
		{
			name: "missing file reader",
			collector: &LinuxResourceCollector{
				now: time.Now,

				runCommand: func(
					context.Context,
					string,
					...string,
				) ([]byte, error) {
					return nil, nil
				},
			},
			expectedError: "resource collector file reader is not initialized",
		},
		{
			name: "missing command runner",
			collector: &LinuxResourceCollector{
				now: time.Now,

				readFile: func(string) ([]byte, error) {
					return nil, nil
				},
			},
			expectedError: "resource collector command runner is not initialized",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				_, err :=
					test.collector.Collect(
						context.Background(),
						validConfig,
						func(
							context.Context,
						) error {
							return nil
						},
					)

				if err == nil {
					t.Fatal(
						"expected collection to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.expectedError,
				) {
					t.Fatalf(
						"unexpected error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func processStatFixture(
	pid int,
	name string,
	userTicks uint64,
	systemTicks uint64,
) string {
	return fmt.Sprintf(
		"%d (%s) S 1 2 3 4 5 6 7 8 9 10 %d %d 0 0 0\n",
		pid,
		name,
		userTicks,
		systemTicks,
	)
}

func workingLinuxResourceCollectorForTest() *LinuxResourceCollector {
	return &LinuxResourceCollector{
		now: time.Now,

		readFile: func(
			path string,
		) ([]byte, error) {
			switch path {
			case filepath.Join(
				procRoot,
				"stat",
			):
				return []byte(
					"cpu 100 100 100 100\n",
				), nil

			case filepath.Join(
				procRoot,
				"100",
				"stat",
			):
				return []byte(
					processStatFixture(
						100,
						"benchmark",
						100,
						10,
					),
				), nil

			case filepath.Join(
				procRoot,
				"100",
				"status",
			):
				return []byte(
					"VmRSS: 10000 kB\n" +
						"Threads: 8\n",
				), nil

			case filepath.Join(
				procRoot,
				"200",
				"stat",
			):
				return []byte(
					processStatFixture(
						200,
						"homedns-dns",
						200,
						20,
					),
				), nil

			case filepath.Join(
				procRoot,
				"200",
				"status",
			):
				return []byte(
					"VmRSS: 20000 kB\n" +
						"Threads: 7\n",
				), nil

			case filepath.Join(
				procRoot,
				"loadavg",
			):
				return []byte(
					"0.42 0.33 0.11 1/10 123\n",
				), nil

			case filepath.Join(
				procRoot,
				"meminfo",
			):
				return []byte(
					"MemAvailable: 300000 kB\n",
				), nil

			case thermalPath:
				return []byte(
					"51000\n",
				), nil

			default:
				return nil, fmt.Errorf(
					"unexpected path: %s",
					path,
				)
			}
		},

		runCommand: func(
			context.Context,
			string,
			...string,
		) ([]byte, error) {
			return []byte(
				"throttled=0x0\n",
			), nil
		},
	}
}
