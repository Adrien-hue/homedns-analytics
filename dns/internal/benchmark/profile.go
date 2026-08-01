package benchmark

import "fmt"

const (
	ProfileQuick      = "quick"
	ProfileValidation = "validation"
	ProfileEndurance  = "endurance"

	DefaultProfile = ProfileQuick
)

// Profile defines a named and reproducible benchmark workload.
type Profile struct {
	Name string

	QueryCount        int
	WarmupQueries     int
	ConcurrentWorkers int
}

// ResolveProfile returns the workload associated with a named benchmark
// profile.
func ResolveProfile(
	name string,
) (Profile, error) {
	switch name {
	case ProfileQuick:
		return Profile{
			Name: ProfileQuick,

			QueryCount:        100,
			WarmupQueries:     10,
			ConcurrentWorkers: 10,
		}, nil

	case ProfileValidation:
		return Profile{
			Name: ProfileValidation,

			QueryCount:        1_000,
			WarmupQueries:     25,
			ConcurrentWorkers: 10,
		}, nil

	case ProfileEndurance:
		return Profile{
			Name: ProfileEndurance,

			QueryCount:        17_000,
			WarmupQueries:     50,
			ConcurrentWorkers: 10,
		}, nil

	default:
		return Profile{}, fmt.Errorf(
			"unsupported benchmark profile %q; supported profiles are %q, %q, and %q",
			name,
			ProfileQuick,
			ProfileValidation,
			ProfileEndurance,
		)
	}
}

// ValidateProfile verifies that a benchmark profile name is supported.
func ValidateProfile(
	name string,
) error {
	_, err := ResolveProfile(name)

	return err
}
