package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type StressTestResult struct {
	PersonCount int `json:"person_count"`
	P50         int `json:"p50"`
	P95         int `json:"p95"`
	P99         int `json:"p99"`
	OkCount     int `json:"ok_count"`
	KoCount     int `json:"ko_count"`
}

type RunStressTestInput struct{}

func runStressTestHandler(projectDir string) func(ctx tool.Context, input RunStressTestInput) (StressTestResult, error) {
	return func(ctx tool.Context, input RunStressTestInput) (StressTestResult, error) {
		if err := runProjectCommand(projectDir, "make stress", "stress test failed"); err != nil {
			return StressTestResult{}, err
		}

		personCount, err := fetchPersonCount()
		if err != nil {
			return StressTestResult{}, err
		}

		result := StressTestResult{PersonCount: personCount}
		statsPath := findLatestStats(filepath.Join(projectDir, "stress-test/user-files/results"))
		if statsPath != "" {
			parseStats(statsPath, &result)
		}

		return result, nil
	}
}

type RebuildStackInput struct{}

type RebuildStackOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func rebuildStackHandler(projectDir string) func(ctx tool.Context, input RebuildStackInput) (RebuildStackOutput, error) {
	return func(ctx tool.Context, input RebuildStackInput) (RebuildStackOutput, error) {
		if err := runProjectCommand(projectDir, "docker compose build --no-cache && make down && make up", "stack rebuild failed"); err != nil {
			return RebuildStackOutput{Success: false, Message: err.Error()}, nil
		}

		for range 30 {
			checkCmd := exec.Command("curl", "-sf", "http://localhost:9999/contagem-pessoas")
			if err := checkCmd.Run(); err == nil {
				return RebuildStackOutput{Success: true, Message: "Stack rebuilt and healthy"}, nil
			}
			time.Sleep(1 * time.Second)
		}

		return RebuildStackOutput{Success: false, Message: "Stack not healthy after 30s"}, nil
	}
}

type GetResultsInput struct{}

func getResultsHandler(projectDir string) func(ctx tool.Context, input GetResultsInput) (StressTestResult, error) {
	return func(ctx tool.Context, input GetResultsInput) (StressTestResult, error) {
		personCount, err := fetchPersonCount()
		if err != nil {
			return StressTestResult{}, err
		}

		result := StressTestResult{PersonCount: personCount}
		statsPath := findLatestStats(filepath.Join(projectDir, "stress-test/user-files/results"))
		if statsPath != "" {
			parseStats(statsPath, &result)
		}
		return result, nil
	}
}

func runProjectCommand(projectDir, shellCommand, action string) error {
	cmd := exec.Command("bash", "-c", shellCommand)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\noutput tail:\n%s", action, err, outputTail(string(output)))
	}
	return nil
}

func fetchPersonCount() (int, error) {
	var lastErr error
	for range 5 {
		countCmd := exec.Command("curl", "-sf", "http://localhost:9999/contagem-pessoas")
		countOut, err := countCmd.Output()
		if err == nil {
			return parsePersonCount(countOut)
		}
		lastErr = err
		time.Sleep(1 * time.Second)
	}
	return 0, fmt.Errorf("failed to get person count: %w", lastErr)
}

func parsePersonCount(output []byte) (int, error) {
	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, fmt.Errorf("invalid person count response %q: %w", strings.TrimSpace(string(output)), err)
	}
	return count, nil
}

func outputTail(output string) string {
	const maxLen = 500
	if len(output) <= maxLen {
		return output
	}
	return output[len(output)-maxLen:]
}

func findLatestStats(resultsDir string) string {
	var latest string
	var latestTime time.Time

	_ = filepath.Walk(resultsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Name() == "stats.json" && info.ModTime().After(latestTime) {
			latest = path
			latestTime = info.ModTime()
		}
		return nil
	})

	return latest
}

func parseStats(path string, result *StressTestResult) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var stats struct {
		Stats struct {
			NumberOfRequests struct {
				Ok int `json:"ok"`
				Ko int `json:"ko"`
			} `json:"numberOfRequests"`
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
	if err := json.Unmarshal(data, &stats); err != nil {
		return
	}

	result.P50 = stats.Stats.Percentiles1.Total
	result.P95 = stats.Stats.Percentiles3.Total
	result.P99 = stats.Stats.Percentiles4.Total
	result.OkCount = stats.Stats.NumberOfRequests.Ok
	result.KoCount = stats.Stats.NumberOfRequests.Ko
}

func NewExecutorTools(projectDir string) ([]tool.Tool, error) {
	runTest, err := functiontool.New(functiontool.Config{
		Name:        "run_stress_test",
		Description: "Runs the full Gatling stress test (~3 min) and returns results including person_count, p50, p95, p99, ok_count, ko_count",
	}, runStressTestHandler(projectDir))
	if err != nil {
		return nil, err
	}

	rebuild, err := functiontool.New(functiontool.Config{
		Name:        "rebuild_stack",
		Description: "Rebuilds Docker images (no cache), stops, and restarts the full stack. Waits until healthy.",
	}, rebuildStackHandler(projectDir))
	if err != nil {
		return nil, err
	}

	getResults, err := functiontool.New(functiontool.Config{
		Name:        "get_current_results",
		Description: "Gets the latest stress test results from stats.json and current person count without re-running the test",
	}, getResultsHandler(projectDir))
	if err != nil {
		return nil, err
	}

	return []tool.Tool{runTest, rebuild, getResults}, nil
}
