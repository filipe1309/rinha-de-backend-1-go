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
	Auto          bool
}

func ParseConfig() *Config {
	cfg := &Config{}
	flag.IntVar(&cfg.TargetCount, "target-count", 44936, "Target person count to beat")
	flag.IntVar(&cfg.TargetP99, "target-p99", 17418, "Target p99 latency in ms to beat")
	flag.IntVar(&cfg.MaxIterations, "max-iterations", 5, "Maximum optimization iterations")
	flag.StringVar(&cfg.Model, "model", "", "Gemini model name (default: gemini-2.5-pro)")
	flag.BoolVar(&cfg.Auto, "auto", true, "Run autonomously without interactive prompt")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	cfg.ProjectDir = cwd

	return cfg
}
