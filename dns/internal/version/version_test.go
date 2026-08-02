package version

import "testing"

func TestString(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuildTime := BuildTime

	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuildTime = originalBuildTime
	})

	Version = "v0.2.0"
	Commit = "a3f8d21"
	BuildTime = "2026-07-24T18:00:00Z"

	expected := `HomeDNS DNS
Version: v0.2.0
Commit: a3f8d21
Built: 2026-07-24T18:00:00Z`

	if got := String(); got != expected {
		t.Fatalf("String() = %q, want %q", got, expected)
	}
}

func TestDevelopmentDefaults(t *testing.T) {
	if Version == "" {
		t.Error("Version must not be empty")
	}

	if Commit == "" {
		t.Error("Commit must not be empty")
	}

	if BuildTime == "" {
		t.Error("BuildTime must not be empty")
	}
}
