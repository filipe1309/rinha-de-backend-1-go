package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestReadFileHandler_ReadsAllowedFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "internal/person/model.go")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create file directory: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("package person\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	output, err := readFileHandler(projectDir)(nil, ReadFileInput{Path: "internal/person/model.go"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.Error != "" {
		t.Fatalf("expected empty output error, got %q", output.Error)
	}
	if output.Content != "package person\n" {
		t.Fatalf("unexpected content %q", output.Content)
	}
}

func TestReadFileHandler_RejectsTraversalAndDisallowedPaths(t *testing.T) {
	projectDir := t.TempDir()
	handler := readFileHandler(projectDir)

	traversalOutput, err := handler(nil, ReadFileInput{Path: "../go.mod"})
	if err != nil {
		t.Fatalf("expected nil error for traversal case, got %v", err)
	}
	if traversalOutput.Error != "path traversal not allowed" {
		t.Fatalf("unexpected traversal error %q", traversalOutput.Error)
	}

	disallowedOutput, err := handler(nil, ReadFileInput{Path: "cmd/optimizer/main.go"})
	if err != nil {
		t.Fatalf("expected nil error for disallowed case, got %v", err)
	}
	if disallowedOutput.Error == "" {
		t.Fatalf("expected disallowed path error")
	}
}

func TestGetGatlingReportHandler_ReturnsPerRequestStats(t *testing.T) {
	projectDir := t.TempDir()
	resultsDir := filepath.Join(projectDir, "stress-test/user-files/results/run-1/js")
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		t.Fatalf("failed to create results dir: %v", err)
	}

	statsJSON := `{
		"stats": {
			"numberOfRequests": {"ok": 120, "ko": 3},
			"percentiles1": {"total": 10},
			"percentiles3": {"total": 20},
			"percentiles4": {"total": 30}
		},
		"contents": {
			"req_create": {
				"stats": {
					"name": "POST /pessoas",
					"numberOfRequests": {"total": 50, "ok": 49, "ko": 1},
					"meanResponseTime": {"total": 15},
					"percentiles1": {"total": 11},
					"percentiles3": {"total": 22},
					"percentiles4": {"total": 33}
				}
			}
		}
	}`
	statsPath := filepath.Join(resultsDir, "stats.json")
	if err := os.WriteFile(statsPath, []byte(statsJSON), 0o644); err != nil {
		t.Fatalf("failed to write stats file: %v", err)
	}

	output, err := getGatlingReportHandler(projectDir)(nil, GatlingReportInput{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.Error != "" {
		t.Fatalf("expected empty output error, got %q", output.Error)
	}
	if output.Global.P50 != 10 || output.Global.P95 != 20 || output.Global.P99 != 30 {
		t.Fatalf("unexpected global percentiles: %+v", output.Global)
	}
	if output.Global.OkCount != 120 || output.Global.KoCount != 3 {
		t.Fatalf("unexpected global request counts: %+v", output.Global)
	}
	if len(output.Requests) != 1 {
		t.Fatalf("expected 1 request entry, got %d", len(output.Requests))
	}

	req := output.Requests[0]
	if req.Name != "POST /pessoas" || req.Count != 50 || req.Ok != 49 || req.Ko != 1 || req.P50 != 11 || req.P95 != 22 || req.P99 != 33 || req.Mean != 15 {
		t.Fatalf("unexpected request stats: %+v", req)
	}
}

func TestGetGatlingReportHandler_ReturnsErrorWhenStatsMissing(t *testing.T) {
	output, err := getGatlingReportHandler(t.TempDir())(nil, GatlingReportInput{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.Error == "" {
		t.Fatalf("expected missing stats error")
	}
}

func TestListProjectFilesHandler_ReturnsExistingModifiableFiles(t *testing.T) {
	projectDir := t.TempDir()
	paths := []string{
		"cmd/api/main.go",
		"internal/person/model.go",
		"docker-compose.yml",
	}
	for _, path := range paths {
		fullPath := filepath.Join(projectDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	output, err := listProjectFilesHandler(projectDir)(nil, ListFilesInput{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	sort.Strings(output.Files)
	expected := append([]string(nil), paths...)
	sort.Strings(expected)
	if !reflect.DeepEqual(output.Files, expected) {
		t.Fatalf("expected %v, got %v", expected, output.Files)
	}
}

func TestNewAnalyzerTools_ReturnsExpectedTools(t *testing.T) {
	tools, err := NewAnalyzerTools(t.TempDir())
	if err != nil {
		t.Fatalf("expected tools without error, got %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
}
