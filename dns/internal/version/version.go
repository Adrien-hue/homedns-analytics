package version

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// String returns the complete human-readable build information.
func String() string {
	return fmt.Sprintf(
		"HomeDNS DNS\nVersion: %s\nCommit: %s\nBuilt: %s",
		Version,
		Commit,
		BuildTime,
	)
}
