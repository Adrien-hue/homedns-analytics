package benchmark

import (
	"strings"
	"testing"
)

func TestResolveProfile(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name     string
		profile  string
		expected Profile
	}{
		{
			name:    "quick",
			profile: ProfileQuick,
			expected: Profile{
				Name:              ProfileQuick,
				QueryCount:        100,
				WarmupQueries:     10,
				ConcurrentWorkers: 10,
			},
		},
		{
			name:    "validation",
			profile: ProfileValidation,
			expected: Profile{
				Name:              ProfileValidation,
				QueryCount:        1_000,
				WarmupQueries:     25,
				ConcurrentWorkers: 10,
			},
		},
		{
			name:    "endurance",
			profile: ProfileEndurance,
			expected: Profile{
				Name:              ProfileEndurance,
				QueryCount:        17_000,
				WarmupQueries:     50,
				ConcurrentWorkers: 10,
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual, err := ResolveProfile(
					test.profile,
				)
				if err != nil {
					t.Fatalf(
						"resolve benchmark profile: %v",
						err,
					)
				}

				if actual != test.expected {
					t.Fatalf(
						"unexpected profile: got %+v, want %+v",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestResolveProfileRejectsUnsupportedProfile(
	t *testing.T,
) {
	t.Parallel()

	_, err := ResolveProfile(
		"unknown",
	)
	if err == nil {
		t.Fatal(
			"expected unsupported profile to fail",
		)
	}

	for _, expected := range []string{
		`unsupported benchmark profile "unknown"`,
		ProfileQuick,
		ProfileValidation,
		ProfileEndurance,
	} {
		if !strings.Contains(
			err.Error(),
			expected,
		) {
			t.Fatalf(
				"error does not contain %q: %v",
				expected,
				err,
			)
		}
	}
}

func TestValidateProfile(
	t *testing.T,
) {
	t.Parallel()

	for _, profile := range []string{
		ProfileQuick,
		ProfileValidation,
		ProfileEndurance,
	} {
		profile := profile

		t.Run(
			profile,
			func(t *testing.T) {
				t.Parallel()

				if err := ValidateProfile(
					profile,
				); err != nil {
					t.Fatalf(
						"validate benchmark profile: %v",
						err,
					)
				}
			},
		)
	}
}

func TestValidateProfileRejectsUnsupportedProfile(
	t *testing.T,
) {
	t.Parallel()

	err := ValidateProfile(
		"unknown",
	)
	if err == nil {
		t.Fatal(
			"expected unsupported profile to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"unsupported benchmark profile",
	) {
		t.Fatalf(
			"unexpected validation error: %v",
			err,
		)
	}
}

func TestDefaultProfile(
	t *testing.T,
) {
	t.Parallel()

	if DefaultProfile != ProfileQuick {
		t.Fatalf(
			"unexpected default profile: got %q, want %q",
			DefaultProfile,
			ProfileQuick,
		)
	}

	profile, err := ResolveProfile(
		DefaultProfile,
	)
	if err != nil {
		t.Fatalf(
			"resolve default profile: %v",
			err,
		)
	}

	if profile.Name != ProfileQuick {
		t.Fatalf(
			"unexpected resolved default profile: got %q, want %q",
			profile.Name,
			ProfileQuick,
		)
	}
}

func TestProfileWorkloadsAreValid(
	t *testing.T,
) {
	t.Parallel()

	for _, name := range []string{
		ProfileQuick,
		ProfileValidation,
		ProfileEndurance,
	} {
		name := name

		t.Run(
			name,
			func(t *testing.T) {
				t.Parallel()

				profile, err :=
					ResolveProfile(name)
				if err != nil {
					t.Fatalf(
						"resolve benchmark profile: %v",
						err,
					)
				}

				if profile.QueryCount <= 0 {
					t.Fatalf(
						"query count must be positive: %d",
						profile.QueryCount,
					)
				}

				if profile.WarmupQueries < 0 {
					t.Fatalf(
						"warmup count cannot be negative: %d",
						profile.WarmupQueries,
					)
				}

				if profile.ConcurrentWorkers <= 0 {
					t.Fatalf(
						"concurrency must be positive: %d",
						profile.ConcurrentWorkers,
					)
				}

				if profile.ConcurrentWorkers >
					profile.QueryCount {
					t.Fatalf(
						"concurrency %d exceeds query count %d",
						profile.ConcurrentWorkers,
						profile.QueryCount,
					)
				}
			},
		)
	}
}

func TestProfileWorkloadsIncreaseByScope(
	t *testing.T,
) {
	t.Parallel()

	quick, err :=
		ResolveProfile(ProfileQuick)
	if err != nil {
		t.Fatalf(
			"resolve quick profile: %v",
			err,
		)
	}

	validation, err :=
		ResolveProfile(ProfileValidation)
	if err != nil {
		t.Fatalf(
			"resolve validation profile: %v",
			err,
		)
	}

	endurance, err :=
		ResolveProfile(ProfileEndurance)
	if err != nil {
		t.Fatalf(
			"resolve endurance profile: %v",
			err,
		)
	}

	if quick.QueryCount >=
		validation.QueryCount {
		t.Fatalf(
			"quick query count %d must be below validation query count %d",
			quick.QueryCount,
			validation.QueryCount,
		)
	}

	if validation.QueryCount >=
		endurance.QueryCount {
		t.Fatalf(
			"validation query count %d must be below endurance query count %d",
			validation.QueryCount,
			endurance.QueryCount,
		)
	}

	if quick.WarmupQueries >
		validation.WarmupQueries {
		t.Fatalf(
			"quick warmup count %d exceeds validation warmup count %d",
			quick.WarmupQueries,
			validation.WarmupQueries,
		)
	}

	if validation.WarmupQueries >
		endurance.WarmupQueries {
		t.Fatalf(
			"validation warmup count %d exceeds endurance warmup count %d",
			validation.WarmupQueries,
			endurance.WarmupQueries,
		)
	}
}
