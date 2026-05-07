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
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
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
		modelName = os.Getenv("GEMINI_MODEL")
	}
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

	if cfg.Auto {
		runAuto(ctx, rootAgent, cfg)
		return
	}

	// Interactive mode
	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(rootAgent),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, flag.Args()); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}

func runAuto(ctx context.Context, rootAgent agent.Agent, cfg *Config) {
	appName := "optimizer"
	userID := "auto"

	sessionService := session.InMemoryService()
	resp, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          rootAgent,
		SessionService: sessionService,
	})
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

	prompt := fmt.Sprintf(
		"Start optimizing now. Target: person_count > %d AND p99 < %d ms. You have up to %d iterations.",
		cfg.TargetCount, cfg.TargetP99, cfg.MaxIterations,
	)

	fmt.Printf("🚀 Starting autonomous optimization\n")
	fmt.Printf("   Target: >%d people, <%dms p99\n", cfg.TargetCount, cfg.TargetP99)
	fmt.Printf("   Max iterations: %d\n\n", cfg.MaxIterations)

	userMsg := genai.NewContentFromText(prompt, genai.RoleUser)

	for event, err := range r.Run(ctx, userID, resp.Session.ID(), userMsg, agent.RunConfig{}) {
		if err != nil {
			fmt.Printf("\n❌ Error: %v\n", err)
			os.Exit(1)
		}
		if event.LLMResponse.Content == nil {
			continue
		}
		for _, p := range event.LLMResponse.Content.Parts {
			if p.Text != "" {
				fmt.Print(p.Text)
			}
		}
	}
	fmt.Println()
}
