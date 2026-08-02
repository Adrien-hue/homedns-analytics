package benchmark

import (
	"errors"
	"math"
	"slices"
	"time"
)

// LatencyStatistics contains aggregated latency measurements expressed in
// milliseconds.
type LatencyStatistics struct {
	Minimum float64 `json:"minimum"`
	Mean    float64 `json:"mean"`
	Median  float64 `json:"median"`
	P95     float64 `json:"p95"`
	P99     float64 `json:"p99"`
	Maximum float64 `json:"maximum"`
}

// CalculateLatencyStatistics aggregates a non-empty collection of durations.
//
// Percentiles use linear interpolation between the two nearest ranked samples.
// The input slice is never modified.
func CalculateLatencyStatistics(
	samples []time.Duration,
) (LatencyStatistics, error) {
	if len(samples) == 0 {
		return LatencyStatistics{}, errors.New(
			"at least one latency sample is required",
		)
	}

	values := make([]float64, len(samples))

	for index, sample := range samples {
		if sample < 0 {
			return LatencyStatistics{}, errors.New(
				"latency samples cannot be negative",
			)
		}

		values[index] = durationMilliseconds(sample)
	}

	slices.Sort(values)

	var total float64

	for _, value := range values {
		total += value
	}

	return LatencyStatistics{
		Minimum: round(values[0], 6),
		Mean:    round(total/float64(len(values)), 6),
		Median:  round(percentile(values, 50), 6),
		P95:     round(percentile(values, 95), 6),
		P99:     round(percentile(values, 99), 6),
		Maximum: round(values[len(values)-1], 6),
	}, nil
}

func percentile(
	sortedValues []float64,
	percent float64,
) float64 {
	if len(sortedValues) == 1 {
		return sortedValues[0]
	}

	position := percent / 100 * float64(len(sortedValues)-1)

	lowerIndex := int(math.Floor(position))
	upperIndex := int(math.Ceil(position))

	if lowerIndex == upperIndex {
		return sortedValues[lowerIndex]
	}

	weight := position - float64(lowerIndex)

	return sortedValues[lowerIndex] +
		(sortedValues[upperIndex]-sortedValues[lowerIndex])*weight
}

func durationMilliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func round(value float64, precision int) float64 {
	factor := math.Pow10(precision)

	return math.Round(value*factor) / factor
}
