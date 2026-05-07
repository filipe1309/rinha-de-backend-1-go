package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileHandler_WritesWhitelistedFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "internal/person/model.go")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create file directory: %v", err)
	}

	output, err := writeFileHandler(projectDir)(nil, WriteFileInput{
		Path:    "internal/person/model.go",
		Content: "package person\n",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !output.Success {
		t.Fatalf("expected success, got %+v", output)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "package person\n" {
		t.Fatalf("unexpected file content %q", string(data))
	}
}

func TestWriteFileHandler_RejectsTraversalAndDisallowedPaths(t *testing.T) {
	projectDir := t.TempDir()
	handler := writeFileHandler(projectDir)

	traversalOutput, err := handler(nil, WriteFileInput{Path: "../go.mod", Content: "ignored"})
	if err != nil {
		t.Fatalf("expected nil error for traversal case, got %v", err)
	}
	if traversalOutput.Error != "path traversal not allowed" {
		t.Fatalf("unexpected traversal error %q", traversalOutput.Error)
	}

	disallowedOutput, err := handler(nil, WriteFileInput{Path: "cmd/optimizer/main.go", Content: "package main"})
	if err != nil {
		t.Fatalf("expected nil error for disallowed case, got %v", err)
	}
	if disallowedOutput.Error == "" {
		t.Fatalf("expected disallowed path error")
	}
}

func TestGoBuildHandler_ReturnsSuccessForBuildableProject(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/test\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	output, err := goBuildHandler(projectDir)(nil, GoBuildInput{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !output.Success {
		t.Fatalf("expected success, got %+v", output)
	}
	if output.Output != "Build successful" {
		t.Fatalf("unexpected output %q", output.Output)
	}
}

func TestGoBuildHandler_ReturnsFailureOutputForInvalidProject(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/test\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "broken.go"), []byte("package main\n\nfunc main() {\n"), 0o644); err != nil {
		t.Fatalf("failed to write broken.go: %v", err)
	}

	output, err := goBuildHandler(projectDir)(nil, GoBuildInput{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.Success {
		t.Fatalf("expected failure, got %+v", output)
	}
	if !strings.Contains(output.Output, "syntax error") {
		t.Fatalf("expected syntax error output, got %q", output.Output)
	}
}

func TestNewOptimizerTools_ReturnsExpectedTools(t *testing.T) {
	tools, err := NewOptimizerTools(t.TempDir())
	if err != nil {
		t.Fatalf("expected tools without error, got %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
}
