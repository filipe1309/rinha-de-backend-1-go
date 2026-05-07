package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type ReadFileInput struct {
	Path string `json:"path"`
}

type ReadFileOutput struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func readFileHandler(projectDir string) func(ctx tool.Context, input ReadFileInput) (ReadFileOutput, error) {
	allowedPrefixes := []string{
		"cmd/api/",
		"internal/",
		"docker-compose.yml",
		"nginx.conf",
		"db/init.sql",
		"Dockerfile",
		"Makefile",
		"stress-test/",
	}

	return func(ctx tool.Context, input ReadFileInput) (ReadFileOutput, error) {
		cleanPath := filepath.Clean(input.Path)
		if strings.Contains(cleanPath, "..") {
			return ReadFileOutput{Error: "path traversal not allowed"}, nil
		}

		allowed := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(cleanPath, prefix) || cleanPath == prefix {
				allowed = true
				break
			}
		}
		if !allowed {
			return ReadFileOutput{Error: fmt.Sprintf("path %q not in allowed list", cleanPath)}, nil
		}

		fullPath := filepath.Join(projectDir, cleanPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return ReadFileOutput{Error: err.Error()}, nil
		}
		return ReadFileOutput{Content: string(data)}, nil
	}
}

type GatlingReportInput struct{}

type RequestTypeStats struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Ok    int    `json:"ok"`
	Ko    int    `json:"ko"`
	P50   int    `json:"p50"`
	P95   int    `json:"p95"`
	P99   int    `json:"p99"`
	Mean  int    `json:"mean"`
}

type GatlingReportOutput struct {
	Global   StressTestResult   `json:"global"`
	Requests []RequestTypeStats `json:"requests"`
	Error    string             `json:"error,omitempty"`
}

func getGatlingReportHandler(projectDir string) func(ctx tool.Context, input GatlingReportInput) (GatlingReportOutput, error) {
	return func(ctx tool.Context, input GatlingReportInput) (GatlingReportOutput, error) {
		statsPath := findLatestStats(filepath.Join(projectDir, "stress-test/user-files/results"))
		if statsPath == "" {
			return GatlingReportOutput{Error: "no stats.json found, run stress test first"}, nil
		}

		data, err := os.ReadFile(statsPath)
		if err != nil {
			return GatlingReportOutput{Error: err.Error()}, nil
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return GatlingReportOutput{Error: err.Error()}, nil
		}

		output := GatlingReportOutput{}
		parseStats(statsPath, &output.Global)

		if contents, ok := raw["contents"]; ok {
			var reqMap map[string]json.RawMessage
			if err := json.Unmarshal(contents, &reqMap); err == nil {
				for _, v := range reqMap {
					var req struct {
						Stats struct {
							Name             string `json:"name"`
							NumberOfRequests struct {
								Total int `json:"total"`
								Ok    int `json:"ok"`
								Ko    int `json:"ko"`
							} `json:"numberOfRequests"`
							MeanResponseTime struct {
								Total int `json:"total"`
							} `json:"meanResponseTime"`
							Percentiles1 struct {
								Total int `json:"total"`
							} `json:"percentiles1"`
							Percentiles3 struct {
								Total int `json:"total"`
							} `json:"percentiles3"`
							Percentiles4 struct {
								Total int `json:"total"`
							} `json:"percentiles4"`
						} `json:"stats"`
					}
					if err := json.Unmarshal(v, &req); err == nil && req.Stats.Name != "" {
						output.Requests = append(output.Requests, RequestTypeStats{
							Name:  req.Stats.Name,
							Count: req.Stats.NumberOfRequests.Total,
							Ok:    req.Stats.NumberOfRequests.Ok,
							Ko:    req.Stats.NumberOfRequests.Ko,
							P50:   req.Stats.Percentiles1.Total,
							P95:   req.Stats.Percentiles3.Total,
							P99:   req.Stats.Percentiles4.Total,
							Mean:  req.Stats.MeanResponseTime.Total,
						})
					}
				}
			}
		}

		return output, nil
	}
}

type ListFilesInput struct{}

type ListFilesOutput struct {
	Files []string `json:"files"`
}

func listProjectFilesHandler(projectDir string) func(ctx tool.Context, input ListFilesInput) (ListFilesOutput, error) {
	return func(ctx tool.Context, input ListFilesInput) (ListFilesOutput, error) {
		modifiable := []string{
			"cmd/api/main.go",
			"internal/person/handler.go",
			"internal/person/repository.go",
			"internal/person/batcher.go",
			"internal/person/cache.go",
			"internal/person/model.go",
			"internal/server/server.go",
			"docker-compose.yml",
			"nginx.conf",
			"db/init.sql",
			"Dockerfile",
		}

		existing := []string{}
		for _, f := range modifiable {
			if _, err := os.Stat(filepath.Join(projectDir, f)); err == nil {
				existing = append(existing, f)
			}
		}
		return ListFilesOutput{Files: existing}, nil
	}
}

func NewAnalyzerTools(projectDir string) ([]tool.Tool, error) {
	readFile, err := functiontool.New(functiontool.Config{
		Name:        "read_file",
		Description: "Reads a project file by path. Only files in the allowed list can be read (cmd/api/, internal/, configs, db/).",
	}, readFileHandler(projectDir))
	if err != nil {
		return nil, err
	}

	report, err := functiontool.New(functiontool.Config{
		Name:        "get_gatling_report",
		Description: "Gets detailed Gatling report with per-request-type breakdown (creation, search, etc.) showing p50/p95/p99 for each.",
	}, getGatlingReportHandler(projectDir))
	if err != nil {
		return nil, err
	}

	listFiles, err := functiontool.New(functiontool.Config{
		Name:        "list_project_files",
		Description: "Lists all modifiable project files that can be optimized.",
	}, listProjectFilesHandler(projectDir))
	if err != nil {
		return nil, err
	}

	return []tool.Tool{readFile, report, listFiles}, nil
}
