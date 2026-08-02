package benchmark

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestResourceCollectionConfigValidate(
	t *testing.T,
) {
	t.Parallel()

	config := ResourceCollectionConfig{
		BenchmarkPID: 100,
		TargetPID:    200,
		Interval:     time.Second,
	}

	if err := config.Validate(); err != nil {
		t.Fatalf(
			"validate resource collection config: %v",
			err,
		)
	}
}

func TestResourceCollectionConfigValidateRejectsInvalidValues(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		mutate        func(*ResourceCollectionConfig)
		expectedError string
	}{
		{
			name: "invalid benchmark PID",
			mutate: func(
				config *ResourceCollectionConfig,
			) {
				config.BenchmarkPID = 0
			},
			expectedError: "benchmark process PID must be greater than zero",
		},
		{
			name: "invalid target PID",
			mutate: func(
				config *ResourceCollectionConfig,
			) {
				config.TargetPID = 0
			},
			expectedError: "target service PID must be greater than zero",
		},
		{
			name: "invalid interval",
			mutate: func(
				config *ResourceCollectionConfig,
			) {
				config.Interval = 0
			},
			expectedError: "resource sampling interval must be greater than zero",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				config := ResourceCollectionConfig{
					BenchmarkPID: 100,
					TargetPID:    200,
					Interval:     time.Second,
				}

				test.mutate(&config)

				err := config.Validate()
				if err == nil {
					t.Fatal(
						"expected validation to fail",
					)
				}

				if err.Error() !=
					test.expectedError {
					t.Fatalf(
						"unexpected error: got %q, want %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestNoopResourceCollectorRunsBenchmark(
	t *testing.T,
) {
	t.Parallel()

	called := false

	collector := NoopResourceCollector{}

	result, err := collector.Collect(
		context.Background(),
		ResourceCollectionConfig{
			BenchmarkPID: 100,
			TargetPID:    200,
			Interval:     time.Second,
		},
		func(context.Context) error {
			called = true

			return nil
		},
	)
	if err != nil {
		t.Fatalf(
			"collect resources: %v",
			err,
		)
	}

	if !called {
		t.Fatal(
			"benchmark function was not called",
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

	if len(result.Samples) != 0 {
		t.Fatalf(
			"unexpected noop sample count: %d",
			len(result.Samples),
		)
	}

	if result.StartedAt.IsZero() ||
		result.EndedAt.IsZero() {
		t.Fatal(
			"noop collector did not record collection boundaries",
		)
	}
}

func TestNoopResourceCollectorReturnsBenchmarkError(
	t *testing.T,
) {
	t.Parallel()

	expectedErr := errors.New(
		"benchmark failed",
	)

	collector := NoopResourceCollector{}

	_, err := collector.Collect(
		context.Background(),
		ResourceCollectionConfig{
			BenchmarkPID: 100,
			TargetPID:    200,
			Interval:     time.Second,
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
			"unexpected benchmark error: got %v, want %v",
			err,
			expectedErr,
		)
	}
}

func TestResourceCollectorFunc(
	t *testing.T,
) {
	t.Parallel()

	called := false

	collector := ResourceCollectorFunc(
		func(
			_ context.Context,
			config ResourceCollectionConfig,
			run func(context.Context) error,
		) (ResourceCollectionResult, error) {
			called = true

			if config.BenchmarkPID != 100 {
				t.Fatalf(
					"unexpected benchmark PID: %d",
					config.BenchmarkPID,
				)
			}

			if err := run(
				context.Background(),
			); err != nil {
				return ResourceCollectionResult{},
					err
			}

			return ResourceCollectionResult{
				Samples: make([]ResourceSample, 0),
				Errors:  make([]string, 0),
			}, nil
		},
	)

	_, err := collector.Collect(
		context.Background(),
		ResourceCollectionConfig{
			BenchmarkPID: 100,
			TargetPID:    200,
			Interval:     time.Second,
		},
		func(context.Context) error {
			return nil
		},
	)
	if err != nil {
		t.Fatalf(
			"collect resources: %v",
			err,
		)
	}

	if !called {
		t.Fatal(
			"resource collector function was not called",
		)
	}
}
