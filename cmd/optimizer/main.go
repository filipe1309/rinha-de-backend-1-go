package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

func main() {
	// Load .env file if present (silently ignored if missing)
	_ = godotenv.Load()

	cfg := ParseConfig()
	ctx := context.Background()

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "GOOGLE_API_KEY is required. The Go ADK only supports Gemini models.")
		fmt.Fprintln(os.Stderr, "Get a key at: https://aistudio.google.com/apikey")
		os.Exit(1)
	}

	modelName := cfg.Model
	if modelName == "" {
		modelName = "gemini-2.5-pro"
	}

	model, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{
		APIKey: apiKey,
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
