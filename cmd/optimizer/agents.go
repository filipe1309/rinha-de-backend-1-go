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

func BuildRootAgent(m model.LLM, cfg *Config) agent.Agent {
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
6. Go HTTP server timeouts`,
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

func buildExecutorAgent(m model.LLM, cfg *Config) agent.Agent {
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

func buildAnalyzerAgent(m model.LLM, cfg *Config) agent.Agent {
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
- Why it causes the problem (with data from the report)
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

func buildOptimizerAgent(m model.LLM, cfg *Config) agent.Agent {
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
