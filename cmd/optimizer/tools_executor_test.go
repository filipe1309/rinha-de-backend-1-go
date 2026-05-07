package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePersonCount_ReturnsParsedValue(t *testing.T) {
	count, err := parsePersonCount([]byte("42\n"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 42 {
		t.Fatalf("expected count 42, got %d", count)
	}
}

func TestParsePersonCount_ReturnsErrorForInvalidOutput(t *testing.T) {
	_, err := parsePersonCount([]byte("not-a-number"))
	if err == nil {
		t.Fatal("expected invalid count output to return an error")
	}
}

func TestOutputTail_TruncatesLongOutput(t *testing.T) {
	tail := outputTail(strings.Repeat("a", 600))
	if len(tail) != 500 {
		t.Fatalf("expected tail length 500, got %d", len(tail))
	}
}

func TestParseStats_PopulatesStressTestResult(t *testing.T) {
	tempDir := t.TempDir()
	statsPath := filepath.Join(tempDir, "stats.json")

	statsJSON := `{
		"stats": {
			"numberOfRequests": {
				"ok": 123,
				"ko": 4
			},
			"percentiles1": {
				"total": 11
			},
			"percentiles3": {
				"total": 22
			},
			"percentiles4": {
				"total": 33
			}
		}
	}`
	if err := os.WriteFile(statsPath, []byte(statsJSON), 0o644); err != nil {
		t.Fatalf("failed to write stats file: %v", err)
	}

	result := StressTestResult{}
	parseStats(statsPath, &result)

	if result.P50 != 11 || result.P95 != 22 || result.P99 != 33 {
		t.Fatalf("unexpected percentiles: %+v", result)
	}
	if result.OkCount != 123 || result.KoCount != 4 {
		t.Fatalf("unexpected request counts: %+v", result)
	}
}

func TestFindLatestStats_ReturnsNewestStatsFile(t *testing.T) {
	resultsDir := t.TempDir()
	olderDir := filepath.Join(resultsDir, "older")
	newerDir := filepath.Join(resultsDir, "newer")
	if err := os.MkdirAll(olderDir, 0o755); err != nil {
		t.Fatalf("failed to create older dir: %v", err)
	}
	if err := os.MkdirAll(newerDir, 0o755); err != nil {
		t.Fatalf("failed to create newer dir: %v", err)
	}

	olderStats := filepath.Join(olderDir, "stats.json")
	newerStats := filepath.Join(newerDir, "stats.json")
	if err := os.WriteFile(olderStats, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to write older stats: %v", err)
	}
	if err := os.WriteFile(newerStats, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to write newer stats: %v", err)
	}

	olderModTime := time.Unix(1, 0)
	newerModTime := time.Unix(2, 0)
	if err := os.Chtimes(olderStats, olderModTime, olderModTime); err != nil {
		t.Fatalf("failed to set older stats mtime: %v", err)
	}
	if err := os.Chtimes(newerStats, newerModTime, newerModTime); err != nil {
		t.Fatalf("failed to set newer stats mtime: %v", err)
	}

	latest := findLatestStats(resultsDir)
	if latest != newerStats {
		t.Fatalf("expected latest stats %q, got %q", newerStats, latest)
	}
}

func TestNewExecutorTools_ReturnsExpectedTools(t *testing.T) {
	tools, err := NewExecutorTools(t.TempDir())
	if err != nil {
		t.Fatalf("expected tools without error, got %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
}
