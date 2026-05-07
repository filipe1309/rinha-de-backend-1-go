package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/filipe1309/rinha-de-backend-1-2023/internal/person"
	"github.com/filipe1309/rinha-de-backend-1-2023/internal/server"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	initSQL, err := os.ReadFile(filepath.Join("..", "db", "init.sql"))
	if err != nil {
		t.Fatalf("failed to read init.sql: %v", err)
	}

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("rinha"),
		postgres.WithUsername("rinha"),
		postgres.WithPassword("rinha"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	if _, err := pool.Exec(ctx, string(initSQL)); err != nil {
		t.Fatalf("failed to run init.sql: %v", err)
	}

	cleanup := func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	}
	return pool, cleanup
}

func setupTestServer(t *testing.T, pool *pgxpool.Pool) *httptest.Server {
	t.Helper()

	cache := person.NewCache()
	repo := person.NewPgRepository(pool)
	batcher := person.NewBatcher(repo, 5000, 500, 5*time.Millisecond)
	handler := person.NewHandlerWithBatcher(cache, repo, batcher)

	ctx, cancel := context.WithCancel(context.Background())
	go batcher.Run(ctx)

	srv := server.NewServer(":0", handler)
	ts := httptest.NewServer(srv.Handler)

	t.Cleanup(func() {
		ts.Close()
		cancel()
		batcher.Wait()
	})

	return ts
}

func getCount(t *testing.T, baseURL string) string {
	t.Helper()

	resp, err := http.Get(baseURL + "/contagem-pessoas")
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("failed to read count response: %v", err)
	}
	return strings.TrimSpace(string(body))
}

func waitForCount(t *testing.T, baseURL string, want int, timeout time.Duration) {
	t.Helper()

	expected := fmt.Sprintf("%d", want)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if getCount(t, baseURL) == expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("count did not reach %d within %s", want, timeout)
}

func TestIntegration_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	ts := setupTestServer(t, pool)

	body := `{"apelido":"josé","nome":"José Roberto","nascimento":"2000-10-01","stack":["C#","Node","Oracle"]}`
	resp, err := http.Post(ts.URL+"/pessoas", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
	}
	location := resp.Header.Get("Location")
	if location == "" {
		resp.Body.Close()
		t.Fatal("expected Location header")
	}
	resp.Body.Close()

	resp, err = http.Get(ts.URL + location)
	if err != nil {
		t.Fatalf("GET by ID failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var p person.PersonResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		resp.Body.Close()
		t.Fatalf("failed to decode person response: %v", err)
	}
	resp.Body.Close()

	if p.Apelido != "josé" {
		t.Errorf("expected apelido 'josé', got '%s'", p.Apelido)
	}
	if p.Nome != "José Roberto" {
		t.Errorf("expected nome 'José Roberto', got '%s'", p.Nome)
	}

	waitForCount(t, ts.URL, 1, 5*time.Second)

	resp, err = http.Get(ts.URL + "/pessoas?t=node")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected 200 from search, got %d", resp.StatusCode)
	}
	var results []person.PersonResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		resp.Body.Close()
		t.Fatalf("failed to decode search response: %v", err)
	}
	resp.Body.Close()

	if len(results) != 1 {
		t.Errorf("expected 1 search result, got %d", len(results))
	}

	if got := getCount(t, ts.URL); got != "1" {
		t.Errorf("expected count '1', got '%s'", got)
	}
}

func TestIntegration_DuplicateNickname(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	ts := setupTestServer(t, pool)

	body := `{"apelido":"unique","nome":"Test","nascimento":"2000-01-01","stack":null}`

	resp, err := http.Post(ts.URL+"/pessoas", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		t.Fatalf("first create: expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	waitForCount(t, ts.URL, 1, 5*time.Second)

	resp, err = http.Post(ts.URL+"/pessoas", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("duplicate create failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		resp.Body.Close()
		t.Fatalf("duplicate: expected 422, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	if got := getCount(t, ts.URL); got != "1" {
		t.Errorf("expected count '1' after duplicate rejection, got '%s'", got)
	}
}

func TestIntegration_ConcurrentCreates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	ts := setupTestServer(t, pool)

	const n = 50
	results := make(chan int, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			body := fmt.Sprintf(`{"apelido":"user%d","nome":"User %d","nascimento":"2000-01-01","stack":["Go"]}`, i, i)
			resp, err := http.Post(ts.URL+"/pessoas", "application/json", bytes.NewBufferString(body))
			if err != nil {
				results <- 0
				return
			}
			statusCode := resp.StatusCode
			resp.Body.Close()
			results <- statusCode
		}(i)
	}

	created := 0
	for i := 0; i < n; i++ {
		code := <-results
		if code == http.StatusCreated {
			created++
		}
	}

	if created != n {
		t.Errorf("expected %d created, got %d", n, created)
	}

	waitForCount(t, ts.URL, n, 5*time.Second)

	expected := fmt.Sprintf("%d", n)
	if got := getCount(t, ts.URL); got != expected {
		t.Errorf("expected count %s, got '%s'", expected, got)
	}
}
