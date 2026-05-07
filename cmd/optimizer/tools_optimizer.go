package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type WriteFileOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func writeFileHandler(projectDir string) func(ctx tool.Context, input WriteFileInput) (WriteFileOutput, error) {
	allowedFiles := map[string]bool{
		"cmd/api/main.go":               true,
		"internal/person/handler.go":    true,
		"internal/person/repository.go": true,
		"internal/person/batcher.go":    true,
		"internal/person/cache.go":      true,
		"internal/person/model.go":      true,
		"internal/server/server.go":     true,
		"docker-compose.yml":            true,
		"nginx.conf":                    true,
		"db/init.sql":                   true,
		"Dockerfile":                    true,
	}

	return func(ctx tool.Context, input WriteFileInput) (WriteFileOutput, error) {
		cleanPath := filepath.Clean(input.Path)
		if strings.Contains(cleanPath, "..") {
			return WriteFileOutput{Error: "path traversal not allowed"}, nil
		}

		if !allowedFiles[cleanPath] {
			return WriteFileOutput{Error: fmt.Sprintf("file %q not in modifiable whitelist", cleanPath)}, nil
		}

		fullPath := filepath.Join(projectDir, cleanPath)
		if err := os.WriteFile(fullPath, []byte(input.Content), 0o644); err != nil {
			return WriteFileOutput{Error: err.Error()}, nil
		}

		return WriteFileOutput{Success: true}, nil
	}
}

type GoBuildInput struct{}

type GoBuildOutput struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

func goBuildHandler(projectDir string) func(ctx tool.Context, input GoBuildInput) (GoBuildOutput, error) {
	return func(ctx tool.Context, input GoBuildInput) (GoBuildOutput, error) {
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = projectDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return GoBuildOutput{Success: false, Output: string(out)}, nil
		}
		return GoBuildOutput{Success: true, Output: "Build successful"}, nil
	}
}

func NewOptimizerTools(projectDir string) ([]tool.Tool, error) {
	writeFile, err := functiontool.New(functiontool.Config{
		Name:        "write_file",
		Description: "Writes content to a project file. Only files in the modifiable whitelist can be written. Use read_file first to understand current content before modifying.",
	}, writeFileHandler(projectDir))
	if err != nil {
		return nil, err
	}

	readFile, err := functiontool.New(functiontool.Config{
		Name:        "read_file",
		Description: "Reads a project file by path to understand current content before modifying.",
	}, readFileHandler(projectDir))
	if err != nil {
		return nil, err
	}

	goBuild, err := functiontool.New(functiontool.Config{
		Name:        "run_go_build",
		Description: "Runs 'go build ./...' to verify Go code compiles. Always call this after modifying Go files.",
	}, goBuildHandler(projectDir))
	if err != nil {
		return nil, err
	}

	return []tool.Tool{readFile, writeFile, goBuild}, nil
}
