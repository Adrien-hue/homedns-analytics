package benchmark

import (
	"testing"
	"time"
)

func TestBuildResourceSummary(
	t *testing.T,
) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.August,
		1,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	endedAt := startedAt.Add(
		10 * time.Second,
	)

	result := ResourceCollectionResult{
		StartedAt: startedAt,
		EndedAt:   endedAt,

		BenchmarkPIDBefore: 100,
		BenchmarkPIDAfter:  100,

		TargetPIDBefore: 200,
		TargetPIDAfter:  200,

		Before: ResourceSample{
			BenchmarkProcess: ProcessResourceSample{
				PID:        100,
				RSSBytes:   uint64Pointer(10),
				Threads:    uint64Pointer(2),
				CPUPercent: float64Pointer(1),
			},

			TargetService: ProcessResourceSample{
				PID:        200,
				RSSBytes:   uint64Pointer(20),
				Threads:    uint64Pointer(3),
				CPUPercent: float64Pointer(2),
			},

			System: SystemResourceSample{
				LoadAverageOneMinute: float64Pointer(0.1),

				MemoryAvailableBytes: uint64Pointer(1000),

				TemperatureCelsius: float64Pointer(50),

				ThrottledState: stringPointer(
					"throttled=0x0",
				),
			},
		},

		After: ResourceSample{
			BenchmarkProcess: ProcessResourceSample{
				PID:        100,
				RSSBytes:   uint64Pointer(12),
				Threads:    uint64Pointer(4),
				CPUPercent: float64Pointer(3),
			},

			TargetService: ProcessResourceSample{
				PID:        200,
				RSSBytes:   uint64Pointer(24),
				Threads:    uint64Pointer(5),
				CPUPercent: float64Pointer(4),
			},

			System: SystemResourceSample{
				LoadAverageOneMinute: float64Pointer(0.4),

				MemoryAvailableBytes: uint64Pointer(800),

				TemperatureCelsius: float64Pointer(54),

				ThrottledState: stringPointer(
					"throttled=0x0",
				),
			},
		},

		Samples: []ResourceSample{
			{
				BenchmarkProcess: ProcessResourceSample{
					CPUPercent: float64Pointer(2),
					RSSBytes:   uint64Pointer(11),
					Threads:    uint64Pointer(3),
				},

				TargetService: ProcessResourceSample{
					CPUPercent: float64Pointer(3),
					RSSBytes:   uint64Pointer(22),
					Threads:    uint64Pointer(4),
				},

				System: SystemResourceSample{
					LoadAverageOneMinute: float64Pointer(0.2),

					MemoryAvailableBytes: uint64Pointer(900),

					TemperatureCelsius: float64Pointer(52),

					ThrottledState: stringPointer(
						"throttled=0x0",
					),
				},
			},
		},

		Errors: []string{
			"sample warning",
		},
	}

	summary := BuildResourceSummary(
		result,
		time.Second,
	)

	if summary.Sampling.IntervalMilliseconds !=
		1000 {
		t.Fatalf(
			"unexpected interval: %d",
			summary.Sampling.IntervalMilliseconds,
		)
	}

	if summary.Sampling.SamplesCollected != 1 {
		t.Fatalf(
			"unexpected sample count: %d",
			summary.Sampling.SamplesCollected,
		)
	}

	if summary.BenchmarkProcess.PIDBefore == nil ||
		*summary.BenchmarkProcess.PIDBefore != 100 {
		t.Fatal(
			"unexpected benchmark PID before",
		)
	}

	if summary.TargetService.PIDAfter == nil ||
		*summary.TargetService.PIDAfter != 200 {
		t.Fatal(
			"unexpected target PID after",
		)
	}

	if summary.BenchmarkProcess.CPUPercent.Mean == nil ||
		*summary.BenchmarkProcess.CPUPercent.Mean != 2 {
		t.Fatalf(
			"unexpected benchmark CPU mean: %v",
			summary.BenchmarkProcess.CPUPercent.Mean,
		)
	}

	if summary.TargetService.RSSBytes.Peak == nil ||
		*summary.TargetService.RSSBytes.Peak != 24 {
		t.Fatalf(
			"unexpected target RSS peak: %v",
			summary.TargetService.RSSBytes.Peak,
		)
	}

	if summary.System.MemoryAvailableBytes.Minimum == nil ||
		*summary.System.MemoryAvailableBytes.Minimum != 900 {
		t.Fatalf(
			"unexpected memory minimum: %v",
			summary.System.MemoryAvailableBytes.Minimum,
		)
	}

	if summary.System.TemperatureCelsius.Mean == nil ||
		*summary.System.TemperatureCelsius.Mean != 52 {
		t.Fatalf(
			"unexpected temperature mean: %v",
			summary.System.TemperatureCelsius.Mean,
		)
	}

	if summary.System.Throttling.ObservedDuringRun == nil ||
		*summary.System.Throttling.ObservedDuringRun {
		t.Fatal(
			"unexpected throttling detection",
		)
	}
}

func TestThrottlingDetected(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		value    *string
		expected bool
	}{
		{
			value:    nil,
			expected: false,
		},
		{
			value: stringPointer(
				"throttled=0x0",
			),
			expected: false,
		},
		{
			value: stringPointer(
				"throttled=0x50000",
			),
			expected: true,
		},
	}

	for _, test := range tests {
		actual := throttlingDetected(
			test.value,
		)

		if actual != test.expected {
			t.Fatalf(
				"unexpected throttling result: got %t, want %t",
				actual,
				test.expected,
			)
		}
	}
}

func TestBuildResourceSummaryInitializesEmptyErrors(
	t *testing.T,
) {
	t.Parallel()

	summary := BuildResourceSummary(
		ResourceCollectionResult{},
		time.Second,
	)

	if summary.Sampling.Errors == nil {
		t.Fatal(
			"resource collection errors must be initialized",
		)
	}

	if len(summary.Sampling.Errors) != 0 {
		t.Fatalf(
			"unexpected resource collection errors: %v",
			summary.Sampling.Errors,
		)
	}
}

func TestBuildResourceSummaryCopiesErrors(
	t *testing.T,
) {
	t.Parallel()

	result := ResourceCollectionResult{
		Errors: []string{
			"collection warning",
		},
	}

	summary := BuildResourceSummary(
		result,
		time.Second,
	)

	summary.Sampling.Errors[0] =
		"modified"

	if result.Errors[0] !=
		"collection warning" {
		t.Fatal(
			"resource summary shares error storage with collection result",
		)
	}
}
