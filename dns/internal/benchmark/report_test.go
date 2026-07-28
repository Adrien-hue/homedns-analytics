package benchmark

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewReport(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.July,
		28,
		20,
		30,
		0,
		0,
		time.FixedZone("CEST", 2*60*60),
	)

	report := NewReport(
		"2026-07-28T18-30-00Z",
		startedAt,
	)

	if report.SchemaVersion != SchemaVersion {
		t.Fatalf(
			"unexpected schema version: got %q, want %q",
			report.SchemaVersion,
			SchemaVersion,
		)
	}

	if report.Benchmark.ID != "2026-07-28T18-30-00Z" {
		t.Fatalf(
			"unexpected benchmark ID: %q",
			report.Benchmark.ID,
		)
	}

	if report.Benchmark.StartedAt.Location() != time.UTC {
		t.Fatal("expected benchmark start time in UTC")
	}

	if report.Project.Name != "homedns-analytics" {
		t.Fatalf(
			"unexpected project name: %q",
			report.Project.Name,
		)
	}

	if report.Scenarios == nil {
		t.Fatal("expected scenarios to be initialized")
	}

	if report.Summary.Notes == nil {
		t.Fatal("expected summary notes to be initialized")
	}
}

func TestReportComplete(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.July,
		28,
		18,
		30,
		0,
		0,
		time.UTC,
	)

	report := NewReport("benchmark-id", startedAt)
	report.Complete(
		startedAt.Add(1500 * time.Millisecond),
	)

	if report.Benchmark.EndedAt != startedAt.Add(
		1500*time.Millisecond,
	) {
		t.Fatalf(
			"unexpected end time: %s",
			report.Benchmark.EndedAt,
		)
	}

	if report.Benchmark.DurationMilliseconds != 1500 {
		t.Fatalf(
			"unexpected duration: got %f, want %f",
			report.Benchmark.DurationMilliseconds,
			1500.0,
		)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	report := NewReport(
		"benchmark-id",
		time.Date(
			2026,
			time.July,
			28,
			18,
			30,
			0,
			0,
			time.UTC,
		),
	)

	var output bytes.Buffer

	if err := WriteJSON(&output, report); err != nil {
		t.Fatalf("write JSON report: %v", err)
	}

	if !strings.Contains(
		output.String(),
		`"schema_version": "1.0"`,
	) {
		t.Fatalf(
			"output does not contain schema version:\n%s",
			output.String(),
		)
	}

	var decoded Report

	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode generated report: %v", err)
	}

	if decoded.Benchmark.ID != report.Benchmark.ID {
		t.Fatalf(
			"unexpected decoded benchmark ID: got %q, want %q",
			decoded.Benchmark.ID,
			report.Benchmark.ID,
		)
	}
}

func TestWriteJSONRejectsNilWriter(t *testing.T) {
	t.Parallel()

	err := WriteJSON(nil, Report{})
	if err == nil {
		t.Fatal("expected nil writer to fail")
	}
}

func TestWriteJSONFile(t *testing.T) {
	t.Parallel()

	report := NewReport(
		"benchmark-id",
		time.Date(
			2026,
			time.July,
			28,
			18,
			30,
			0,
			0,
			time.UTC,
		),
	)

	path := filepath.Join(
		t.TempDir(),
		"nested",
		"benchmark.json",
	)

	if err := WriteJSONFile(path, report); err != nil {
		t.Fatalf("write JSON report file: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JSON report file: %v", err)
	}

	var decoded Report

	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("decode JSON report file: %v", err)
	}

	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf(
			"unexpected schema version: got %q, want %q",
			decoded.SchemaVersion,
			SchemaVersion,
		)
	}
}

func TestWriteJSONFileRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	err := WriteJSONFile("", Report{})
	if err == nil {
		t.Fatal("expected empty path to fail")
	}
}
