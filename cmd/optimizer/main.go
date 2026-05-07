package main

import (
	"context"
	"flag"
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
		case "openai", "github":
			modelName = "gpt-4o"
		}
	}

	if provider != "" && provider != "gemini" && key != "" {
		log.Fatalf("detected %s credentials, but the current optimizer scaffold only supports Gemini-backed ADK models", provider)
	}

	_ = key

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
	if err = l.Execute(ctx, config, flag.Args()); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
