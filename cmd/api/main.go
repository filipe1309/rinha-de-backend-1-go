package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/filipe1309/rinha-de-backend-1-2023/internal/person"
	"github.com/filipe1309/rinha-de-backend-1-2023/internal/server"
)

func main() {
	port := envOrDefault("PORT", "8080")
	dbURL := envOrDefault("DATABASE_URL", "postgres://rinha:rinha@localhost:5432/rinha?sslmode=disable")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("failed to parse database URL: %v", err)
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 15 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		log.Printf("waiting for database... (%d/30)", i+1)
		time.Sleep(1 * time.Second)
	}
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database not ready after 30s: %v", err)
	}
	log.Println("connected to database")

	cache := person.NewCache()
	repo := person.NewPgRepository(pool)
	batcher := person.NewBatcher(repo, 10000, 1000, 10*time.Millisecond)
	handler := person.NewHandlerWithBatcher(cache, repo, batcher)

	batchCtx, batchCancel := context.WithCancel(context.Background())
	go batcher.Run(batchCtx)

	srv := server.NewServer(fmt.Sprintf(":%s", port), handler)

	go func() {
		log.Printf("server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	batchCancel()
	batcher.Wait()

	log.Println("shutdown complete")
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
