# ADK Performance Optimizer Agent — Design Spec

## Overview

A multi-agent AI system built with Google ADK (Go) that autonomously optimizes the Rinha de Backend implementation by running stress tests, analyzing results, modifying code/config, and iterating until performance targets are met.

**Target:** Beat first place — person count > 44,936 AND p99 < 17,418ms.

## Architecture

```
┌─────────────────────────────────────────────────┐
│            Root Orchestrator Agent               │
│  (drives the loop: test → analyze → optimize)   │
└────────┬──────────────┬──────────────┬──────────┘
         │              │              │
    ┌────▼────┐   ┌─────▼─────┐  ┌────▼─────┐
    │ Executor│   │  Analyzer  │  │ Optimizer│
    │  Agent  │   │   Agent    │  │  Agent   │
    └─────────┘   └───────────┘  └──────────┘
```

### Root Orchestrator Agent

- Manages the iteration loop (max 5 iterations)
- Delegates to sub-agents via `agenttool`
- Tracks iteration count and results history
- Decides when target is met or when to stop
- Instruction: "You are a performance optimization orchestrator. Run stress tests, analyze results, apply optimizations, and repeat until the target is met or max iterations reached."

### Executor Agent

- Runs infrastructure commands (build, deploy, test)
- Returns structured results
- Tools:
  - `run_stress_test()` → Runs full Gatling simulation, returns `{person_count, p50, p95, p99, ok_count, ko_count}`
  - `rebuild_stack()` → `docker compose build --no-cache && make down && make up`, waits for health
  - `get_current_results()` → Parses latest `stats.json` without re-running

### Analyzer Agent

- Reads code and test results to identify bottlenecks
- Produces structured analysis with specific recommendations
- Tools:
  - `read_file(path)` → Reads any project file
  - `get_gatling_report()` → Per-request-type breakdown (creation p99, search p99, etc.)
  - `list_project_files()` → Lists modifiable files

### Optimizer Agent

- Receives analysis and applies code/config changes
- Verifies changes compile
- Tools:
  - `read_file(path)` → Read current file content
  - `write_file(path, content)` → Overwrite file with new content
  - `run_go_build()` → Verify Go code compiles, returns success/error

## Data Flow Per Iteration

1. Root calls Executor → "run stress test"
2. Root checks results against target (person_count > 44,936 AND p99 < 17,418)
3. If target met → stop, print success
4. If not met → Root calls Analyzer → "analyze these results and the code, identify bottlenecks"
5. Root calls Optimizer → "apply these improvements: [analysis output]"
6. Root calls Executor → "rebuild stack"
7. Loop back to step 1

## Model Configuration

Multi-model support via environment variables (checked in order):

1. `GOOGLE_API_KEY` → Gemini 2.5 Flash (native ADK)
2. `OPENAI_API_KEY` → OpenAI GPT-4o via adapter
3. `GH_TOKEN` → GitHub Models via Azure inference endpoint (`https://models.inference.ai.azure.com/chat/completions`)

A `--model` CLI flag overrides auto-detection.

For GitHub Models, the agent makes HTTP calls to the Azure endpoint with `Authorization: Bearer $GH_TOKEN`.

## CLI Interface

```bash
# Run with default settings
go run ./cmd/optimizer

# Custom target and iterations
go run ./cmd/optimizer --target-count=44936 --target-p99=17418 --max-iterations=3

# Specify model
go run ./cmd/optimizer --model=gemini-2.5-flash
```

**Default flags:**
- `--target-count=44936`
- `--target-p99=17418`
- `--max-iterations=5`
- `--model=""` (auto-detect from env vars)

## Output Format

Each iteration prints a structured summary:

```
╔═══ Iteration 1/5 ═══════════════════════╗
║ Contagem de Pessoas: 41,455             ║
║ p99 geral: 37,790 ms                   ║
║ Status: BELOW TARGET                    ║
╚═════════════════════════════════════════╝
→ Analyzer: "Search queries saturating DB..."
→ Optimizer: Modified docker-compose.yml, nginx.conf
→ Rebuilding stack...
```

Final output:
```
╔═══ FINAL RESULTS ═══════════════════════╗
║ Target: 44,936 pessoas / 17,418ms p99   ║
║ Achieved: 45,200 pessoas / 12,300ms p99 ║
║ Status: ✅ TARGET MET (iteration 3/5)   ║
╚═════════════════════════════════════════╝
```

## Safety Mechanisms

1. **Git stash** before starting — preserves current state
2. **Go build check** after every file modification — if build fails, revert and try different approach
3. **Hard limit** of 5 iterations — prevents runaway API costs
4. **Git restore** on failure — reverts all changes if agent cannot improve
5. **Modifiable files whitelist:**
   - `cmd/api/main.go`
   - `internal/person/*.go`
   - `internal/server/server.go`
   - `docker-compose.yml`
   - `nginx.conf`
   - `db/init.sql`
   - `Dockerfile`

## Project Structure

```
cmd/optimizer/
├── main.go              # Entry point, CLI flags, model selection
├── agents.go            # Agent definitions (root, executor, analyzer, optimizer)
├── tools_executor.go    # Executor tools (run_stress_test, rebuild_stack, etc.)
├── tools_analyzer.go    # Analyzer tools (read_file, get_gatling_report, etc.)
└── tools_optimizer.go   # Optimizer tools (write_file, run_go_build)
```

## Dependencies

- `google.golang.org/adk` — Google ADK framework
- `google.golang.org/genai` — Gemini model client
- Standard library for shell execution (`os/exec`), file I/O, JSON parsing

## Agent Instructions

### Root Orchestrator Instruction

```
You are a performance optimization agent for a Go backend API (Rinha de Backend 2023).

Your goal: achieve person_count > {target_count} AND p99 < {target_p99}ms.

Process:
1. Ask executor to run the stress test
2. Check if target is met. If yes, stop.
3. Ask analyzer to identify bottlenecks in the code and configuration
4. Ask optimizer to apply the recommended changes
5. Ask executor to rebuild the stack
6. Repeat (max {max_iterations} times)

Key files you can modify:
- docker-compose.yml (PostgreSQL tuning, resource limits)
- nginx.conf (connection handling, buffering)
- cmd/api/main.go (pool size, batcher settings)
- internal/person/handler.go (caching, response handling)
- internal/person/repository.go (queries, connection handling)
- internal/person/batcher.go (batch size, flush interval)
- db/init.sql (indexes, schema)

Common optimization areas:
- PostgreSQL memory settings (shared_buffers, work_mem must fit in 2.2GB container)
- Connection pool sizing (too many = contention, too few = starvation)
- Batch insert size and flush interval
- Search query caching
- Nginx upstream keepalive and buffering
- Index strategies for search_field
```

### Analyzer Instruction

```
You are a performance analyst. Given stress test results and source code, identify the top 3 bottlenecks causing high p99 latency or low person count.

For each bottleneck:
1. Identify the root cause (with file and line reference)
2. Explain why it causes the problem
3. Propose a specific fix with exact values

Consider: DB memory (must fit 2.2GB), connection pool contention, query performance, caching opportunities, batch sizing, network configuration.
```

### Optimizer Instruction

```
You are a code optimizer. Apply the recommended changes to source files.

Rules:
- Make minimal, targeted changes
- Always verify the build compiles after changes
- Do not change test files
- Do not change the API interface or behavior
- Only modify files in the allowed list
```
