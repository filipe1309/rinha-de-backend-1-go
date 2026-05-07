# ADK Performance Optimizer Agent — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a multi-agent AI system using Google ADK (Go) that autonomously runs stress tests, analyzes results, modifies code/config, and iterates until the Rinha de Backend implementation beats first place (>44,936 people / <17,418ms p99).

**Architecture:** Three sub-agents (Executor, Analyzer, Optimizer) orchestrated by a Root agent. Custom `functiontool` tools expose shell operations, file I/O, and build commands. Multi-model support (Gemini, OpenAI, GitHub Models) via env vars.

**Tech Stack:** Go, Google ADK (`google.golang.org/adk`), Gemini (`google.golang.org/genai`), `os/exec` for shell commands.

---

## File Structure

```
cmd/optimizer/
├── main.go              # Entry point, CLI flags, model selection, launcher
├── agents.go            # All agent definitions (root, executor, analyzer, optimizer)
├── tools_executor.go    # run_stress_test, rebuild_stack, get_current_results
├── tools_analyzer.go    # read_file, get_gatling_report, list_project_files
├── tools_optimizer.go   # write_file, run_go_build
└── config.go            # Config struct, model factory, CLI parsing
```

---

### Task 1: Project Setup and Dependencies

**Files:**
- Create: `cmd/optimizer/main.go`
- Create: `cmd/optimizer/config.go`

- [ ] **Step 1: Create config.go with CLI flag parsing**

```go
package main

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	TargetCount   int
	TargetP99     int
	MaxIterations int
	Model         string
	ProjectDir    string
}

func ParseConfig() *Config {
	cfg := &Config{}
	flag.IntVar(&cfg.TargetCount, "target-count", 44936, "Target person count to beat")
	flag.IntVar(&cfg.TargetP99, "target-p99", 17418, "Target p99 latency in ms to beat")
	flag.IntVar(&cfg.MaxIterations, "max-iterations", 5, "Maximum optimization iterations")
	flag.StringVar(&cfg.Model, "model", "", "Model name override (auto-detect if empty)")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	cfg.ProjectDir = cwd

	return cfg
}

func DetectModelProvider() (provider string, key string) {
	if k := os.Getenv("GOOGLE_API_KEY"); k != "" {
		return "gemini", k
	}
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		return "openai", k
	}
	if k := os.Getenv("GH_TOKEN"); k != "" {
		return "github", k
	}
	return "", ""
}
```

- [ ] **Step 2: Create main.go with basic ADK launcher**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
)

func main() {
	cfg := ParseConfig()
	ctx := context.Background()

	provider, key := DetectModelProvider()
	if cfg.Model == "" && provider == "" {
		fmt.Fprintln(os.Stderr, "No model API key found. Set GOOGLE_API_KEY, OPENAI_API_KEY, or GH_TOKEN.")
		os.Exit(1)
	}

	modelName := cfg.Model
	if modelName == "" {
		switch provider {
		case "gemini":
			modelName = "gemini-2.5-flash"
		case "openai":
			modelName = "gpt-4o"
		case "github":
			modelName = "gpt-4o"
		}
	}

	_ = key // used later when creating model

	model, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	rootAgent := BuildRootAgent(model, cfg)

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(rootAgent),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
```

- [ ] **Step 3: Install ADK dependency**

Run: `cd /Users/filipe1309/Projects/Personal/rinha-de-backend/rinha-de-backend-1-2023 && go get google.golang.org/adk google.golang.org/genai`

- [ ] **Step 4: Verify module resolves**

Run: `go mod tidy`
Expected: no errors, go.sum updated

- [ ] **Step 5: Commit**

```bash
git add cmd/optimizer/ go.mod go.sum
git commit -m "feat(optimizer): scaffold ADK agent with config and model setup"
```

---

### Task 2: Executor Agent Tools

**Files:**
- Create: `cmd/optimizer/tools_executor.go`

- [ ] **Step 1: Implement run_stress_test tool**

```go
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
		// Run the stress test
		cmd := exec.Command("bash", "-c", "make stress")
		cmd.Dir = projectDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return StressTestResult{}, fmt.Errorf("stress test failed: %w", err)
		}

		// Wait for async flushes
		time.Sleep(5 * time.Second)

		// Get person count
		countCmd := exec.Command("curl", "-sf", "http://localhost:9999/contagem-pessoas")
		countOut, err := countCmd.Output()
		if err != nil {
			return StressTestResult{}, fmt.Errorf("failed to get person count: %w", err)
		}
		personCount, _ := strconv.Atoi(strings.TrimSpace(string(countOut)))

		// Parse stats.json
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
		// Build and restart
		cmd := exec.Command("bash", "-c", "docker compose build --no-cache && make down && make up")
		cmd.Dir = projectDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return RebuildStackOutput{Success: false, Message: err.Error()}, nil
		}

		// Wait for stack to be healthy
		for i := 0; i < 30; i++ {
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
		countCmd := exec.Command("curl", "-sf", "http://localhost:9999/contagem-pessoas")
		countOut, err := countCmd.Output()
		if err != nil {
			return StressTestResult{}, fmt.Errorf("stack not running: %w", err)
		}
		personCount, _ := strconv.Atoi(strings.TrimSpace(string(countOut)))

		result := StressTestResult{PersonCount: personCount}
		statsPath := findLatestStats(filepath.Join(projectDir, "stress-test/user-files/results"))
		if statsPath != "" {
			parseStats(statsPath, &result)
		}
		return result, nil
	}
}

// findLatestStats finds the most recent stats.json in the results directory.
func findLatestStats(resultsDir string) string {
	var latest string
	var latestTime time.Time

	filepath.Walk(resultsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
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

// parseStats reads a Gatling stats.json and populates the result.
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
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./cmd/optimizer/...`
Expected: May fail (agents.go not yet created). Create placeholder if needed.

- [ ] **Step 3: Commit**

```bash
git add cmd/optimizer/tools_executor.go
git commit -m "feat(optimizer): add executor agent tools (stress test, rebuild, results)"
```

---

### Task 3: Analyzer Agent Tools

**Files:**
- Create: `cmd/optimizer/tools_analyzer.go`

- [ ] **Step 1: Implement analyzer tools**

```go
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
		// Security: only allow reading project files
		cleanPath := filepath.Clean(input.Path)
		if strings.Contains(cleanPath, "..") {
			return ReadFileOutput{Error: "path traversal not allowed"}, nil
		}

		allowed := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(cleanPath, prefix) {
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

		// Parse per-request stats from "contents" field
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
```

- [ ] **Step 2: Commit**

```bash
git add cmd/optimizer/tools_analyzer.go
git commit -m "feat(optimizer): add analyzer agent tools (read_file, gatling_report, list_files)"
```

---

### Task 4: Optimizer Agent Tools

**Files:**
- Create: `cmd/optimizer/tools_optimizer.go`

- [ ] **Step 1: Implement optimizer tools**

```go
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
		"cmd/api/main.go":              true,
		"internal/person/handler.go":   true,
		"internal/person/repository.go": true,
		"internal/person/batcher.go":   true,
		"internal/person/cache.go":     true,
		"internal/person/model.go":     true,
		"internal/server/server.go":    true,
		"docker-compose.yml":           true,
		"nginx.conf":                   true,
		"db/init.sql":                  true,
		"Dockerfile":                   true,
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
		if err := os.WriteFile(fullPath, []byte(input.Content), 0644); err != nil {
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

	// Reuse read_file from analyzer
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
```

- [ ] **Step 2: Commit**

```bash
git add cmd/optimizer/tools_optimizer.go
git commit -m "feat(optimizer): add optimizer agent tools (write_file, read_file, go_build)"
```

---

### Task 5: Agent Definitions and Wiring

**Files:**
- Create: `cmd/optimizer/agents.go`

- [ ] **Step 1: Implement all agent definitions**

```go
package main

import (
	"fmt"
	"log"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/agenttool"
)

func BuildRootAgent(m model.Model, cfg *Config) agent.Agent {
	executorAgent := buildExecutorAgent(m, cfg)
	analyzerAgent := buildAnalyzerAgent(m, cfg)
	optimizerAgent := buildOptimizerAgent(m, cfg)

	rootInstruction := fmt.Sprintf(`You are a performance optimization orchestrator for a Go backend API (Rinha de Backend 2023 Q3).

Your goal: achieve person_count > %d AND p99 < %d ms.

You have access to three specialist sub-agents:
1. executor_agent - runs stress tests, rebuilds the stack, gets results
2. analyzer_agent - reads source code and Gatling reports, identifies bottlenecks
3. optimizer_agent - modifies source files to apply performance improvements

Process (repeat up to %d times):
1. Ask executor_agent to run the stress test
2. Check results: if person_count > %d AND p99 < %d, declare SUCCESS and stop
3. If target not met, ask analyzer_agent to analyze the Gatling report and read relevant source files to identify the top 3 bottlenecks
4. Ask optimizer_agent to apply the recommended fixes. It must call run_go_build after modifying Go files.
5. Ask executor_agent to rebuild the stack
6. Go back to step 1

IMPORTANT CONSTRAINTS:
- PostgreSQL runs in a 2.2GB RAM container. shared_buffers + work_mem*connections must stay under 1.5GB.
- Total resources: 1.5 CPU / 3GB RAM across all containers.
- The stack uses 2 Go API instances behind Nginx, each with 0.2 CPU / 0.3GB.
- Do NOT change test files or the API interface.
- If a change causes a build failure, ask optimizer to revert and try something else.
- After each iteration, summarize what was changed and the results.

Common optimization strategies (try in order of impact):
1. PostgreSQL tuning (shared_buffers, work_mem, WAL settings) - must fit in 2.2GB
2. Connection pool sizing (balance between contention and starvation)
3. Batch insert size and flush interval
4. Search query caching (in-memory with TTL)
5. Nginx keepalive and upstream tuning
6. Go HTTP server timeouts

When you achieve the target or exhaust iterations, output a final summary.`,
		cfg.TargetCount, cfg.TargetP99,
		cfg.MaxIterations,
		cfg.TargetCount, cfg.TargetP99)

	a, err := llmagent.New(llmagent.Config{
		Name:        "root_orchestrator",
		Model:       m,
		Description: "Performance optimization orchestrator that drives test-analyze-optimize loops",
		Instruction: rootInstruction,
		Tools: []tool.Tool{
			agenttool.New(executorAgent, nil),
			agenttool.New(analyzerAgent, nil),
			agenttool.New(optimizerAgent, nil),
		},
	})
	if err != nil {
		log.Fatalf("Failed to create root agent: %v", err)
	}
	return a
}

func buildExecutorAgent(m model.Model, cfg *Config) agent.Agent {
	tools, err := NewExecutorTools(cfg.ProjectDir)
	if err != nil {
		log.Fatalf("Failed to create executor tools: %v", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "executor_agent",
		Model:       m,
		Description: "Runs infrastructure commands: stress tests, stack rebuilds, and result retrieval",
		Instruction: `You are an executor agent. You run stress tests, rebuild the Docker stack, and retrieve results.

When asked to run a stress test: call run_stress_test and report all metrics.
When asked to rebuild: call rebuild_stack and confirm success/failure.
When asked for current results: call get_current_results.

Always report results in a structured format with person_count, p50, p95, p99, ok_count, ko_count.`,
		Tools: tools,
	})
	if err != nil {
		log.Fatalf("Failed to create executor agent: %v", err)
	}
	return a
}

func buildAnalyzerAgent(m model.Model, cfg *Config) agent.Agent {
	tools, err := NewAnalyzerTools(cfg.ProjectDir)
	if err != nil {
		log.Fatalf("Failed to create analyzer tools: %v", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "analyzer_agent",
		Model:       m,
		Description: "Reads source code and Gatling reports to identify performance bottlenecks and recommend specific fixes",
		Instruction: `You are a performance analyst for a Go backend API.

When asked to analyze, you should:
1. Call get_gatling_report to see per-request-type performance breakdown
2. Call list_project_files to see what can be modified
3. Read the most relevant files based on the bottleneck (e.g., if search is slow, read repository.go and handler.go)
4. Identify the top 3 bottlenecks with specific root causes

For each bottleneck, provide:
- Which file and what code causes it
- Why it's a problem (with data from the report)
- A specific fix with exact values/code

CONSTRAINTS:
- PostgreSQL container has 2.2GB RAM. shared_buffers + (work_mem * max_connections) must be < 1.5GB
- Total system: 1.5 CPU / 3GB RAM
- Each Go API instance: 0.2 CPU / 0.3GB RAM`,
		Tools: tools,
	})
	if err != nil {
		log.Fatalf("Failed to create analyzer agent: %v", err)
	}
	return a
}

func buildOptimizerAgent(m model.Model, cfg *Config) agent.Agent {
	tools, err := NewOptimizerTools(cfg.ProjectDir)
	if err != nil {
		log.Fatalf("Failed to create optimizer tools: %v", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "optimizer_agent",
		Model:       m,
		Description: "Applies code and configuration changes to optimize performance",
		Instruction: `You are a code optimizer. Apply recommended performance changes to source files.

Process:
1. Read the current content of files you need to modify (use read_file)
2. Apply the recommended changes by writing the full new content (use write_file)
3. After modifying any Go file, call run_go_build to verify compilation
4. If build fails, read the error, fix the issue, and try again

Rules:
- Make minimal, targeted changes - don't rewrite entire files unnecessarily
- Never modify test files
- Never change the API interface or behavior (same endpoints, same response format)
- Only modify files in the allowed whitelist
- Always call run_go_build after Go file changes
- If build fails after 2 attempts, report failure and suggest reverting`,
		Tools: tools,
	})
	if err != nil {
		log.Fatalf("Failed to create optimizer agent: %v", err)
	}
	return a
}
```

- [ ] **Step 2: Verify full compilation**

Run: `go build ./cmd/optimizer/...`
Expected: SUCCESS (all files compile together)

- [ ] **Step 3: Commit**

```bash
git add cmd/optimizer/agents.go
git commit -m "feat(optimizer): wire up multi-agent system with root orchestrator"
```

---

### Task 6: Integration Test and Final Verification

**Files:**
- Modify: `cmd/optimizer/main.go` (if needed for compilation fixes)

- [ ] **Step 1: Run go vet for issues**

Run: `go vet ./cmd/optimizer/...`
Expected: No issues

- [ ] **Step 2: Test CLI help output**

Run: `go run ./cmd/optimizer --help`
Expected: Shows flag descriptions (target-count, target-p99, max-iterations, model)

- [ ] **Step 3: Test without API key (should print error)**

Run: `GOOGLE_API_KEY="" OPENAI_API_KEY="" GH_TOKEN="" go run ./cmd/optimizer 2>&1 | head -5`
Expected: "No model API key found" error message

- [ ] **Step 4: Final commit with all fixes**

```bash
git add -A
git commit -m "feat(optimizer): finalize ADK multi-agent performance optimizer

Multi-agent system using Google ADK (Go):
- Root orchestrator drives test→analyze→optimize loop
- Executor agent runs Gatling stress tests and manages Docker
- Analyzer agent reads code/config and identifies bottlenecks
- Optimizer agent applies targeted changes and verifies builds
- Multi-model support (Gemini, OpenAI, GitHub Models)
- Safety: file whitelist, build verification, max iterations"
```

---

### Task 7: Update README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add optimizer section to README**

Add after the Stress Test section:

```markdown
## AI Performance Optimizer

An autonomous AI agent (built with [Google ADK](https://google.github.io/adk-docs/)) that iteratively optimizes the codebase to beat the challenge's first place.

### Prerequisites

- Go 1.25+
- One of: `GOOGLE_API_KEY` (Gemini), `OPENAI_API_KEY`, or `GH_TOKEN` (GitHub Models)
- Docker running with the stack deployed

### Run

\```bash
# Start the stack first
make up

# Run the optimizer (uses first available API key)
go run ./cmd/optimizer

# Custom targets
go run ./cmd/optimizer --target-count=44936 --target-p99=17418 --max-iterations=3
\```

The agent will:
1. Run the Gatling stress test
2. Analyze results and source code for bottlenecks
3. Apply targeted optimizations
4. Rebuild and re-test
5. Repeat until target is met or max iterations reached
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add AI optimizer section to README"
```
