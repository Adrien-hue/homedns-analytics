package benchmark

import (
	"strings"
	"time"
)

// BuildResourceSummary converts raw resource samples into the canonical report
// structure.
func BuildResourceSummary(
	result ResourceCollectionResult,
	interval time.Duration,
) ResourceSummary {
	errors := make(
		[]string,
		len(result.Errors),
	)

	copy(
		errors,
		result.Errors,
	)

	summary := ResourceSummary{
		Sampling: ResourceSampling{
			IntervalMilliseconds: interval.Milliseconds(),

			SamplesCollected: len(result.Samples),

			Errors: errors,
		},
	}

	if !result.StartedAt.IsZero() {
		startedAt := result.StartedAt.UTC()

		summary.Sampling.CollectionStartedAt =
			&startedAt
	}

	if !result.EndedAt.IsZero() {
		endedAt := result.EndedAt.UTC()

		summary.Sampling.CollectionEndedAt =
			&endedAt
	}

	summary.BenchmarkProcess =
		buildProcessResources(
			result.BenchmarkPIDBefore,
			result.BenchmarkPIDAfter,
			result.Before.BenchmarkProcess,
			result.After.BenchmarkProcess,
			result.Samples,
			func(sample ResourceSample) ProcessResourceSample {
				return sample.BenchmarkProcess
			},
		)

	summary.TargetService =
		buildProcessResources(
			result.TargetPIDBefore,
			result.TargetPIDAfter,
			result.Before.TargetService,
			result.After.TargetService,
			result.Samples,
			func(sample ResourceSample) ProcessResourceSample {
				return sample.TargetService
			},
		)

	summary.System =
		buildSystemResources(
			result.Before.System,
			result.After.System,
			result.Samples,
		)

	return summary
}

func buildProcessResources(
	pidBefore int,
	pidAfter int,
	before ProcessResourceSample,
	after ProcessResourceSample,
	samples []ResourceSample,
	selectProcess func(ResourceSample) ProcessResourceSample,
) ProcessResources {
	resources := ProcessResources{}

	if pidBefore > 0 {
		resources.PIDBefore =
			intPointer(pidBefore)
	}

	if pidAfter > 0 {
		resources.PIDAfter =
			intPointer(pidAfter)
	}

	if pidBefore > 0 &&
		pidAfter > 0 {
		restarted :=
			pidBefore != pidAfter

		resources.RestartDetected =
			boolPointer(restarted)
	}

	cpuValues := make(
		[]float64,
		0,
		len(samples)+2,
	)

	rssValues := make(
		[]uint64,
		0,
		len(samples)+2,
	)

	threadValues := make(
		[]uint64,
		0,
		len(samples)+2,
	)

	appendProcessValues(
		&cpuValues,
		&rssValues,
		&threadValues,
		before,
	)

	for _, sample := range samples {
		appendProcessValues(
			&cpuValues,
			&rssValues,
			&threadValues,
			selectProcess(sample),
		)
	}

	appendProcessValues(
		&cpuValues,
		&rssValues,
		&threadValues,
		after,
	)

	resources.CPUPercent =
		buildNumericStatistics(
			cpuValues,
		)

	resources.RSSBytes =
		buildIntegerStatistics(
			rssValues,
		)

	resources.Threads =
		buildIntegerStatistics(
			threadValues,
		)

	return resources
}

func appendProcessValues(
	cpuValues *[]float64,
	rssValues *[]uint64,
	threadValues *[]uint64,
	sample ProcessResourceSample,
) {
	if sample.CPUPercent != nil {
		*cpuValues = append(
			*cpuValues,
			*sample.CPUPercent,
		)
	}

	if sample.RSSBytes != nil {
		*rssValues = append(
			*rssValues,
			*sample.RSSBytes,
		)
	}

	if sample.Threads != nil {
		*threadValues = append(
			*threadValues,
			*sample.Threads,
		)
	}
}

func buildSystemResources(
	before SystemResourceSample,
	after SystemResourceSample,
	samples []ResourceSample,
) SystemResources {
	var resources SystemResources

	if before.LoadAverageOneMinute != nil {
		resources.
			LoadAverageOneMinute.
			Before =
			float64Pointer(
				*before.
					LoadAverageOneMinute,
			)
	}

	if after.LoadAverageOneMinute != nil {
		resources.
			LoadAverageOneMinute.
			After =
			float64Pointer(
				*after.
					LoadAverageOneMinute,
			)
	}

	loadValues := make(
		[]float64,
		0,
		len(samples),
	)

	for _, sample := range samples {
		if sample.System.
			LoadAverageOneMinute != nil {
			loadValues = append(
				loadValues,
				*sample.System.
					LoadAverageOneMinute,
			)
		}
	}

	loadStatistics :=
		buildNumericStatistics(
			loadValues,
		)

	resources.
		LoadAverageOneMinute.
		Mean =
		loadStatistics.Mean

	resources.
		LoadAverageOneMinute.
		Peak =
		loadStatistics.Peak

	if before.MemoryAvailableBytes != nil {
		resources.
			MemoryAvailableBytes.
			Before =
			uint64Pointer(
				*before.
					MemoryAvailableBytes,
			)
	}

	if after.MemoryAvailableBytes != nil {
		resources.
			MemoryAvailableBytes.
			After =
			uint64Pointer(
				*after.
					MemoryAvailableBytes,
			)
	}

	memoryValues := make(
		[]uint64,
		0,
		len(samples),
	)

	for _, sample := range samples {
		if sample.System.
			MemoryAvailableBytes != nil {
			memoryValues = append(
				memoryValues,
				*sample.System.
					MemoryAvailableBytes,
			)
		}
	}

	if len(memoryValues) > 0 {
		minimum := memoryValues[0]

		for _, value := range memoryValues[1:] {
			if value < minimum {
				minimum = value
			}
		}

		resources.
			MemoryAvailableBytes.
			Minimum =
			uint64Pointer(minimum)
	}

	if before.TemperatureCelsius != nil {
		resources.
			TemperatureCelsius.
			Before =
			float64Pointer(
				*before.
					TemperatureCelsius,
			)
	}

	if after.TemperatureCelsius != nil {
		resources.
			TemperatureCelsius.
			After =
			float64Pointer(
				*after.
					TemperatureCelsius,
			)
	}

	temperatureValues := make(
		[]float64,
		0,
		len(samples),
	)

	for _, sample := range samples {
		if sample.System.
			TemperatureCelsius != nil {
			temperatureValues = append(
				temperatureValues,
				*sample.System.
					TemperatureCelsius,
			)
		}
	}

	temperatureStatistics :=
		buildNumericStatistics(
			temperatureValues,
		)

	resources.
		TemperatureCelsius.
		Mean =
		temperatureStatistics.Mean

	resources.
		TemperatureCelsius.
		Peak =
		temperatureStatistics.Peak

	if before.ThrottledState != nil {
		resources.Throttling.Before =
			stringPointer(
				*before.ThrottledState,
			)
	}

	if after.ThrottledState != nil {
		resources.Throttling.After =
			stringPointer(
				*after.ThrottledState,
			)
	}

	throttlingObserved :=
		throttlingDetected(
			before.ThrottledState,
		)

	for _, sample := range samples {
		if throttlingDetected(
			sample.System.
				ThrottledState,
		) {
			throttlingObserved = true
			break
		}
	}

	if throttlingDetected(
		after.ThrottledState,
	) {
		throttlingObserved = true
	}

	if before.ThrottledState != nil ||
		after.ThrottledState != nil ||
		len(samples) > 0 {
		resources.
			Throttling.
			ObservedDuringRun =
			boolPointer(
				throttlingObserved,
			)
	}

	return resources
}

func buildNumericStatistics(
	values []float64,
) NumericStatistics {
	if len(values) == 0 {
		return NumericStatistics{}
	}

	minimum := values[0]
	peak := values[0]

	var total float64

	for _, value := range values {
		total += value

		if value < minimum {
			minimum = value
		}

		if value > peak {
			peak = value
		}
	}

	mean := round(
		total/float64(len(values)),
		6,
	)

	return NumericStatistics{
		Minimum: float64Pointer(minimum),

		Mean: float64Pointer(mean),

		Peak: float64Pointer(peak),
	}
}

func buildIntegerStatistics(
	values []uint64,
) IntegerStatistics {
	if len(values) == 0 {
		return IntegerStatistics{}
	}

	minimum := values[0]
	peak := values[0]

	var total float64

	for _, value := range values {
		total += float64(value)

		if value < minimum {
			minimum = value
		}

		if value > peak {
			peak = value
		}
	}

	mean := round(
		total/float64(len(values)),
		6,
	)

	return IntegerStatistics{
		Minimum: uint64Pointer(minimum),

		Mean: float64Pointer(mean),

		Peak: uint64Pointer(peak),
	}
}

func throttlingDetected(
	value *string,
) bool {
	if value == nil {
		return false
	}

	normalized := strings.TrimSpace(
		strings.ToLower(*value),
	)

	switch normalized {
	case "",
		"0",
		"0x0",
		"throttled=0",
		"throttled=0x0":
		return false

	default:
		return true
	}
}
