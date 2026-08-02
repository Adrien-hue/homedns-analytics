package benchmark

import (
	"reflect"
	"testing"
	"time"
)

func TestCalculateLatencyStatistics(t *testing.T) {
	t.Parallel()

	samples := []time.Duration{
		5 * time.Millisecond,
		1 * time.Millisecond,
		4 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
	}

	original := append([]time.Duration(nil), samples...)

	statistics, err := CalculateLatencyStatistics(samples)
	if err != nil {
		t.Fatalf("calculate latency statistics: %v", err)
	}

	if statistics.Minimum != 1 {
		t.Fatalf(
			"unexpected minimum: got %f, want %f",
			statistics.Minimum,
			1.0,
		)
	}

	if statistics.Mean != 3 {
		t.Fatalf(
			"unexpected mean: got %f, want %f",
			statistics.Mean,
			3.0,
		)
	}

	if statistics.Median != 3 {
		t.Fatalf(
			"unexpected median: got %f, want %f",
			statistics.Median,
			3.0,
		)
	}

	if statistics.P95 != 4.8 {
		t.Fatalf(
			"unexpected p95: got %f, want %f",
			statistics.P95,
			4.8,
		)
	}

	if statistics.P99 != 4.96 {
		t.Fatalf(
			"unexpected p99: got %f, want %f",
			statistics.P99,
			4.96,
		)
	}

	if statistics.Maximum != 5 {
		t.Fatalf(
			"unexpected maximum: got %f, want %f",
			statistics.Maximum,
			5.0,
		)
	}

	if !reflect.DeepEqual(samples, original) {
		t.Fatal("input samples were modified")
	}
}

func TestCalculateLatencyStatisticsWithSingleSample(t *testing.T) {
	t.Parallel()

	statistics, err := CalculateLatencyStatistics(
		[]time.Duration{1500 * time.Microsecond},
	)
	if err != nil {
		t.Fatalf("calculate latency statistics: %v", err)
	}

	const expected = 1.5

	if statistics.Minimum != expected ||
		statistics.Mean != expected ||
		statistics.Median != expected ||
		statistics.P95 != expected ||
		statistics.P99 != expected ||
		statistics.Maximum != expected {
		t.Fatalf(
			"unexpected single-sample statistics: %+v",
			statistics,
		)
	}
}

func TestCalculateLatencyStatisticsRejectsEmptySamples(t *testing.T) {
	t.Parallel()

	_, err := CalculateLatencyStatistics(nil)
	if err == nil {
		t.Fatal("expected empty samples to fail")
	}
}

func TestCalculateLatencyStatisticsRejectsNegativeSamples(
	t *testing.T,
) {
	t.Parallel()

	_, err := CalculateLatencyStatistics(
		[]time.Duration{-time.Millisecond},
	)
	if err == nil {
		t.Fatal("expected negative sample to fail")
	}
}
