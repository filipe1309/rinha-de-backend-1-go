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
