//go:build !linux

package app

// resolveBenchmarkTargetPID returns zero on non-Linux systems.
//
// Resource collection is disabled by RunBenchmark when no target PID can be
// discovered, allowing local macOS benchmark runs to remain supported.
func resolveBenchmarkTargetPID(
	string,
) (int, error) {
	return 0, nil
}
