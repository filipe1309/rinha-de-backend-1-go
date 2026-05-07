package main

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
)

type stubLLM struct{}

func (stubLLM) Name() string {
	return "stub"
}

func (stubLLM) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(func(*model.LLMResponse, error) bool) {}
}

func TestBuildRootAgent_UsesAgentToolsWithoutTransferSubAgents(t *testing.T) {
	cfg := &Config{
		TargetCount:   44936,
		TargetP99:     17418,
		MaxIterations: 5,
		ProjectDir:    t.TempDir(),
	}

	root := BuildRootAgent(stubLLM{}, cfg)
	if root == nil {
		t.Fatal("expected root agent, got nil")
	}

	if root.Name() != "root_orchestrator" {
		t.Fatalf("expected root agent name root_orchestrator, got %q", root.Name())
	}

	if got := len(root.SubAgents()); got != 0 {
		t.Fatalf("expected root to rely on agenttool wiring without transfer sub-agents, got %d sub-agents", got)
	}
}

func TestSpecialistAgentBuilders_ReturnNamedAgents(t *testing.T) {
	cfg := &Config{ProjectDir: t.TempDir()}

	tests := []struct {
		name  string
		want  string
		build func(model.LLM, *Config) agent.Agent
	}{
		{name: "executor", want: "executor_agent", build: buildExecutorAgent},
		{name: "analyzer", want: "analyzer_agent", build: buildAnalyzerAgent},
		{name: "optimizer", want: "optimizer_agent", build: buildOptimizerAgent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := tc.build(stubLLM{}, cfg)
			if a == nil {
				t.Fatal("expected agent, got nil")
			}
			if a.Name() != tc.want {
				t.Fatalf("expected agent name %q, got %q", tc.want, a.Name())
			}
		})
	}
}
