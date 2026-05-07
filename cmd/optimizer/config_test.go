package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfig_DefaultsAndProjectDir(t *testing.T) {
	cfg := runParseConfig(t)

	if cfg.TargetCount != 44936 {
		t.Fatalf("expected default target count 44936, got %d", cfg.TargetCount)
	}
	if cfg.TargetP99 != 17418 {
		t.Fatalf("expected default target p99 17418, got %d", cfg.TargetP99)
	}
	if cfg.MaxIterations != 5 {
		t.Fatalf("expected default max iterations 5, got %d", cfg.MaxIterations)
	}
	if cfg.Model != "" {
		t.Fatalf("expected default model to be empty, got %q", cfg.Model)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if cfg.ProjectDir != cwd {
		t.Fatalf("expected project dir %q, got %q", cwd, cfg.ProjectDir)
	}
}

func TestParseConfig_OverridesFromFlags(t *testing.T) {
	cfg := runParseConfig(t,
		"-target-count", "123",
		"-target-p99", "456",
		"-max-iterations", "7",
		"-model", "gemini-2.5-pro",
	)

	if cfg.TargetCount != 123 {
		t.Fatalf("expected target count 123, got %d", cfg.TargetCount)
	}
	if cfg.TargetP99 != 456 {
		t.Fatalf("expected target p99 456, got %d", cfg.TargetP99)
	}
	if cfg.MaxIterations != 7 {
		t.Fatalf("expected max iterations 7, got %d", cfg.MaxIterations)
	}
	if cfg.Model != "gemini-2.5-pro" {
		t.Fatalf("expected model override to be applied, got %q", cfg.Model)
	}
}

func runParseConfig(t *testing.T, args ...string) *Config {
	t.Helper()

	oldArgs := os.Args
	oldCommandLine := flag.CommandLine

	flag.CommandLine = flag.NewFlagSet(filepath.Base(oldArgs[0]), flag.ContinueOnError)
	os.Args = append([]string{oldArgs[0]}, args...)

	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})

	return ParseConfig()
}
