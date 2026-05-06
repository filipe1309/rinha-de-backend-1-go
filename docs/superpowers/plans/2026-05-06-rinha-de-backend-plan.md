# Rinha de Backend 2023 Q3 — Go Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a high-performance people API in Go with async write-behind caching, PostgreSQL persistence, and Docker Compose deployment for the Rinha de Backend 2023 Q3 challenge.

**Architecture:** Two Go API instances behind Nginx (round-robin) with in-memory `sync.Map` caching and a channel-based batcher that batch-inserts into PostgreSQL using COPY protocol. Validation uses `json.RawMessage` for precise type/null detection.

**Tech Stack:** Go 1.22 (stdlib `net/http`), PostgreSQL 16, pgx/v5, Nginx, Docker, testcontainers-go.

---

## File Structure

| File | Responsibility |
|------|---------------|
| `cmd/api/main.go` | Entry point: parse config from env vars, create dependencies, wire everything, start server with graceful shutdown |
| `internal/person/model.go` | `Person` struct, `CreatePersonRequest` with `json.RawMessage` fields, `Validate()` method returning typed errors |
| `internal/person/cache.go` | `Cache` struct wrapping two `sync.Map` instances (`byID`, `byNickname`), methods: `Set`, `GetByID`, `HasNickname` |
| `internal/person/repository.go` | `Repository` interface + `PgRepository` implementation with `GetByID`, `Search`, `Count`, `InsertBatch` methods |
| `internal/person/batcher.go` | `Batcher` struct with buffered channel, `Run()` goroutine, `Add()` method, `Shutdown()` for graceful drain |
| `internal/person/handler.go` | `Handler` struct with `Create`, `GetByID`, `Search`, `Count` methods, each is an `http.HandlerFunc` |
| `internal/server/server.go` | `NewServer()` that registers routes on `http.ServeMux`, returns configured `http.Server` |
| `db/init.sql` | PostgreSQL schema: `pg_trgm` extension, `pessoas` table, GIN index |
| `Dockerfile` | Multi-stage build: golang:1.22-alpine → alpine:3.19 |
| `docker-compose.yml` | 4 services with resource limits summing to 1.5 CPU / 3.0GB |
| `nginx.conf` | Round-robin between api1:8080 and api2:8080, listen on 9999 |
| `internal/person/handler_test.go` | Unit tests for all HTTP handlers with mocked cache/repo |
| `internal/person/model_test.go` | Unit tests for validation logic |
| `internal/person/cache_test.go` | Unit tests for cache operations |
| `internal/person/batcher_test.go` | Unit tests for batcher with mock repository |
| `tests/integration_test.go` | Integration tests with testcontainers-go (real Postgres) |

---

## Task 1: Project Initialization

**Files:**
- Create: `go.mod`
- Create: `db/init.sql`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/filipe1309/Projects/Personal/rinha-de-backend/rinha-de-backend-1-2023
go mod init github.com/filipe1309/rinha-de-backend-1-2023
```

- [ ] **Step 2: Install dependencies**

```bash
go get github.com/jackc/pgx/v5
go get github.com/google/uuid
```

- [ ] **Step 3: Create database schema**

Create `db/init.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS pessoas (
    id           UUID PRIMARY KEY,
    apelido      VARCHAR(32) UNIQUE NOT NULL,
    nome         VARCHAR(100) NOT NULL,
    nascimento   DATE NOT NULL,
    stack        TEXT,
    search_field TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pessoas_search ON pessoas USING GIN (search_field gin_trgm_ops);
```

- [ ] **Step 4: Create directory structure**

```bash
mkdir -p cmd/api internal/person internal/server tests
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: initialize Go module and database schema"
```

---

## Task 2: Person Model & Validation

**Files:**
- Create: `internal/person/model.go`
- Create: `internal/person/model_test.go`

- [ ] **Step 1: Write failing tests for validation**

Create `internal/person/model_test.go`:

```go
package person

import (
	"testing"
)

func TestValidateCreateRequest_ValidComplete(t *testing.T) {
	body := []byte(`{
		"apelido": "josé",
		"nome": "José Roberto",
		"nascimento": "2000-10-01",
		"stack": ["C#", "Node", "Oracle"]
	}`)
	p, err := ParseAndValidate(body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Apelido != "josé" {
		t.Errorf("expected apelido 'josé', got '%s'", p.Apelido)
	}
	if p.Nome != "José Roberto" {
		t.Errorf("expected nome 'José Roberto', got '%s'", p.Nome)
	}
	if p.Nascimento != "2000-10-01" {
		t.Errorf("expected nascimento '2000-10-01', got '%s'", p.Nascimento)
	}
	if len(p.Stack) != 3 || p.Stack[0] != "C#" {
		t.Errorf("expected stack [C#, Node, Oracle], got %v", p.Stack)
	}
}

func TestValidateCreateRequest_ValidNullStack(t *testing.T) {
	body := []byte(`{
		"apelido": "ana",
		"nome": "Ana Barbosa",
		"nascimento": "1985-09-23",
		"stack": null
	}`)
	p, err := ParseAndValidate(body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Stack != nil {
		t.Errorf("expected nil stack, got %v", p.Stack)
	}
}

func TestValidateCreateRequest_MissingApelido(t *testing.T) {
	body := []byte(`{
		"nome": "Ana Barbosa",
		"nascimento": "1985-09-23",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for missing apelido")
	}
	var ve *ValidationError
	if !isValidationError(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.StatusCode != 422 {
		t.Errorf("expected 422, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_NullNome(t *testing.T) {
	body := []byte(`{
		"apelido": "ana",
		"nome": null,
		"nascimento": "1985-09-23",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for null nome")
	}
	var ve *ValidationError
	if !isValidationError(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.StatusCode != 422 {
		t.Errorf("expected 422, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_NomeIsNumber(t *testing.T) {
	body := []byte(`{
		"apelido": "apelido",
		"nome": 1,
		"nascimento": "1985-01-01",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for numeric nome")
	}
	var ve *ValidationError
	if !isValidationError(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.StatusCode != 400 {
		t.Errorf("expected 400, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_StackWithNumber(t *testing.T) {
	body := []byte(`{
		"apelido": "apelido",
		"nome": "nome",
		"nascimento": "1985-01-01",
		"stack": [1, "PHP"]
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for non-string stack element")
	}
	var ve *ValidationError
	if !isValidationError(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.StatusCode != 400 {
		t.Errorf("expected 400, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_ApelidoTooLong(t *testing.T) {
	longApelido := make([]byte, 33)
	for i := range longApelido {
		longApelido[i] = 'a'
	}
	body := []byte(`{
		"apelido": "` + string(longApelido) + `",
		"nome": "Nome",
		"nascimento": "1985-01-01",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for apelido too long")
	}
	var ve *ValidationError
	if !isValidationError(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.StatusCode != 422 {
		t.Errorf("expected 422, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_InvalidDateFormat(t *testing.T) {
	body := []byte(`{
		"apelido": "apelido",
		"nome": "Nome",
		"nascimento": "01-01-1985",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}
}

func TestBuildSearchField(t *testing.T) {
	p := &Person{
		Apelido:    "José",
		Nome:       "José Roberto",
		Stack:      []string{"C#", "Node"},
	}
	got := p.BuildSearchField()
	if got != "josé josé roberto c# node" {
		t.Errorf("expected 'josé josé roberto c# node', got '%s'", got)
	}
}

func TestBuildSearchField_NilStack(t *testing.T) {
	p := &Person{
		Apelido: "ana",
		Nome:    "Ana Barbosa",
		Stack:   nil,
	}
	got := p.BuildSearchField()
	if got != "ana ana barbosa" {
		t.Errorf("expected 'ana ana barbosa', got '%s'", got)
	}
}

// helper to check if error is *ValidationError using errors.As
func isValidationError(err error, target **ValidationError) bool {
	if ve, ok := err.(*ValidationError); ok {
		*target = ve
		return true
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/filipe1309/Projects/Personal/rinha-de-backend/rinha-de-backend-1-2023
go test ./internal/person/ -v -run TestValidate
```

Expected: compilation error — `ParseAndValidate`, `ValidationError`, `Person`, `BuildSearchField` not defined.

- [ ] **Step 3: Implement the model**

Create `internal/person/model.go`:

```go
package person

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Person struct {
	ID         uuid.UUID `json:"id"`
	Apelido    string    `json:"apelido"`
	Nome       string    `json:"nome"`
	Nascimento string    `json:"nascimento"`
	Stack      []string  `json:"stack"`
}

type ValidationError struct {
	StatusCode int
	Message    string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func NewBadRequestError(msg string) *ValidationError {
	return &ValidationError{StatusCode: 400, Message: msg}
}

func NewUnprocessableError(msg string) *ValidationError {
	return &ValidationError{StatusCode: 422, Message: msg}
}

// ParseAndValidate parses JSON body and validates all fields.
// Returns 400 for type errors, 422 for missing/null required fields or constraint violations.
func ParseAndValidate(body []byte) (*Person, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, NewBadRequestError("invalid JSON")
	}

	// --- apelido ---
	apelidoRaw, ok := raw["apelido"]
	if !ok || string(apelidoRaw) == "null" {
		return nil, NewUnprocessableError("apelido is required")
	}
	var apelido string
	if err := json.Unmarshal(apelidoRaw, &apelido); err != nil {
		return nil, NewBadRequestError("apelido must be a string")
	}
	if len(apelido) > 32 {
		return nil, NewUnprocessableError("apelido must be at most 32 characters")
	}

	// --- nome ---
	nomeRaw, ok := raw["nome"]
	if !ok || string(nomeRaw) == "null" {
		return nil, NewUnprocessableError("nome is required")
	}
	var nome string
	if err := json.Unmarshal(nomeRaw, &nome); err != nil {
		return nil, NewBadRequestError("nome must be a string")
	}
	if len(nome) > 100 {
		return nil, NewUnprocessableError("nome must be at most 100 characters")
	}

	// --- nascimento ---
	nascimentoRaw, ok := raw["nascimento"]
	if !ok || string(nascimentoRaw) == "null" {
		return nil, NewUnprocessableError("nascimento is required")
	}
	var nascimento string
	if err := json.Unmarshal(nascimentoRaw, &nascimento); err != nil {
		return nil, NewBadRequestError("nascimento must be a string")
	}
	if _, err := time.Parse("2006-01-02", nascimento); err != nil {
		return nil, NewUnprocessableError("nascimento must be in YYYY-MM-DD format")
	}

	// --- stack ---
	var stack []string
	stackRaw, ok := raw["stack"]
	if ok && string(stackRaw) != "null" {
		var rawStack []json.RawMessage
		if err := json.Unmarshal(stackRaw, &rawStack); err != nil {
			return nil, NewBadRequestError("stack must be an array of strings")
		}
		stack = make([]string, 0, len(rawStack))
		for _, item := range rawStack {
			var s string
			if err := json.Unmarshal(item, &s); err != nil {
				return nil, NewBadRequestError("each stack element must be a string")
			}
			if len(s) > 32 {
				return nil, NewUnprocessableError("each stack element must be at most 32 characters")
			}
			stack = append(stack, s)
		}
	}

	p := &Person{
		ID:         uuid.New(),
		Apelido:    apelido,
		Nome:       nome,
		Nascimento: nascimento,
		Stack:      stack,
	}

	return p, nil
}

// BuildSearchField builds the lowercase searchable text for pg_trgm index.
func (p *Person) BuildSearchField() string {
	parts := []string{p.Apelido, p.Nome}
	parts = append(parts, p.Stack...)
	return strings.ToLower(strings.Join(parts, " "))
}

// StackAsText returns stack as comma-separated string for DB storage.
func (p *Person) StackAsText() *string {
	if p.Stack == nil {
		return nil
	}
	s := strings.Join(p.Stack, ",")
	return &s
}

// StackFromText parses comma-separated string back to []string.
func StackFromText(s *string) []string {
	if s == nil || *s == "" {
		return nil
	}
	return strings.Split(*s, ",")
}

// MarshalJSON returns the person JSON with stack serialized correctly.
func (p Person) MarshalJSON() ([]byte, error) {
	type Alias Person
	return json.Marshal(&struct {
		ID string `json:"id"`
		Alias
	}{
		ID:    p.ID.String(),
		Alias: Alias(p),
	})
}

// FormatStackJSON returns stack as JSON-friendly value.
func FormatStackJSON(stack []string) interface{} {
	if stack == nil {
		return nil
	}
	return stack
}

// PersonResponse is used for JSON serialization to match the API spec.
type PersonResponse struct {
	ID         string   `json:"id"`
	Apelido    string   `json:"apelido"`
	Nome       string   `json:"nome"`
	Nascimento string   `json:"nascimento"`
	Stack      []string `json:"stack"`
}

// ToResponse converts a Person to a PersonResponse.
func (p *Person) ToResponse() PersonResponse {
	return PersonResponse{
		ID:         p.ID.String(),
		Apelido:    p.Apelido,
		Nome:       p.Nome,
		Nascimento: p.Nascimento,
		Stack:      p.Stack,
	}
}

func NewValidationError(statusCode int, msg string, args ...interface{}) *ValidationError {
	return &ValidationError{StatusCode: statusCode, Message: fmt.Sprintf(msg, args...)}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/person/ -v -run "TestValidate|TestBuildSearchField"
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add Person model with validation and search field builder"
```

---

## Task 3: In-Memory Cache

**Files:**
- Create: `internal/person/cache.go`
- Create: `internal/person/cache_test.go`

- [ ] **Step 1: Write failing tests for cache**

Create `internal/person/cache_test.go`:

```go
package person

import (
	"testing"

	"github.com/google/uuid"
)

func TestCache_SetAndGetByID(t *testing.T) {
	c := NewCache()
	p := &Person{
		ID:      uuid.New(),
		Apelido: "josé",
		Nome:    "José Roberto",
	}
	c.Set(p)

	got, ok := c.GetByID(p.ID)
	if !ok {
		t.Fatal("expected to find person in cache")
	}
	if got.Apelido != "josé" {
		t.Errorf("expected apelido 'josé', got '%s'", got.Apelido)
	}
}

func TestCache_GetByID_NotFound(t *testing.T) {
	c := NewCache()
	_, ok := c.GetByID(uuid.New())
	if ok {
		t.Fatal("expected not to find person in cache")
	}
}

func TestCache_HasNickname(t *testing.T) {
	c := NewCache()
	p := &Person{
		ID:      uuid.New(),
		Apelido: "josé",
		Nome:    "José Roberto",
	}
	c.Set(p)

	if !c.HasNickname("josé") {
		t.Fatal("expected nickname 'josé' to exist")
	}
	if c.HasNickname("ana") {
		t.Fatal("expected nickname 'ana' to not exist")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache()
	done := make(chan struct{})

	for i := 0; i < 100; i++ {
		go func(i int) {
			p := &Person{
				ID:      uuid.New(),
				Apelido: uuid.New().String()[:8],
				Nome:    "Test",
			}
			c.Set(p)
			c.GetByID(p.ID)
			c.HasNickname(p.Apelido)
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/person/ -v -run TestCache -race
```

Expected: compilation error — `NewCache`, `Cache` not defined.

- [ ] **Step 3: Implement the cache**

Create `internal/person/cache.go`:

```go
package person

import (
	"sync"

	"github.com/google/uuid"
)

// Cache provides thread-safe in-memory storage for Person records.
type Cache struct {
	byID       sync.Map // map[uuid.UUID]*Person
	byNickname sync.Map // map[string]struct{}
}

// NewCache creates a new empty Cache.
func NewCache() *Cache {
	return &Cache{}
}

// Set stores a person in both ID and nickname maps.
// Returns false if the nickname already exists (duplicate).
func (c *Cache) Set(p *Person) bool {
	if _, loaded := c.byNickname.LoadOrStore(p.Apelido, struct{}{}); loaded {
		return false
	}
	c.byID.Store(p.ID, p)
	return true
}

// GetByID retrieves a person by UUID. Returns nil, false if not found.
func (c *Cache) GetByID(id uuid.UUID) (*Person, bool) {
	val, ok := c.byID.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Person), true
}

// HasNickname checks if a nickname is already taken.
func (c *Cache) HasNickname(nickname string) bool {
	_, ok := c.byNickname.Load(nickname)
	return ok
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/person/ -v -run TestCache -race
```

Expected: all tests PASS, no race conditions.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add in-memory cache with sync.Map for persons"
```

---

## Task 4: PostgreSQL Repository

**Files:**
- Create: `internal/person/repository.go`

- [ ] **Step 1: Create the repository interface and implementation**

Create `internal/person/repository.go`:

```go
package person

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the interface for person persistence.
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Person, error)
	Search(ctx context.Context, term string) ([]Person, error)
	Count(ctx context.Context) (int, error)
	InsertBatch(ctx context.Context, persons []*Person) error
}

// PgRepository implements Repository using PostgreSQL.
type PgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository creates a new PostgreSQL repository.
func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

// GetByID retrieves a person by UUID from PostgreSQL.
func (r *PgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Person, error) {
	row := r.pool.QueryRow(ctx,
		"SELECT id, apelido, nome, nascimento, stack FROM pessoas WHERE id = $1", id)

	var p Person
	var nascimento string
	var stack *string
	err := row.Scan(&p.ID, &p.Apelido, &p.Nome, &nascimento, &stack)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying person by id: %w", err)
	}
	p.Nascimento = nascimento
	p.Stack = StackFromText(stack)
	return &p, nil
}

// Search finds people whose search_field contains the given term (case-insensitive).
func (r *PgRepository) Search(ctx context.Context, term string) ([]Person, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT id, apelido, nome, nascimento, stack FROM pessoas WHERE search_field ILIKE '%' || $1 || '%' LIMIT 50",
		strings.ToLower(term))
	if err != nil {
		return nil, fmt.Errorf("searching people: %w", err)
	}
	defer rows.Close()

	var people []Person
	for rows.Next() {
		var p Person
		var nascimento string
		var stack *string
		if err := rows.Scan(&p.ID, &p.Apelido, &p.Nome, &nascimento, &stack); err != nil {
			return nil, fmt.Errorf("scanning person row: %w", err)
		}
		p.Nascimento = nascimento
		p.Stack = StackFromText(stack)
		people = append(people, p)
	}

	if people == nil {
		people = []Person{}
	}
	return people, nil
}

// Count returns the total number of people in the database.
func (r *PgRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM pessoas").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting people: %w", err)
	}
	return count, nil
}

// InsertBatch inserts multiple people using PostgreSQL COPY protocol.
func (r *PgRepository) InsertBatch(ctx context.Context, persons []*Person) error {
	if len(persons) == 0 {
		return nil
	}

	rows := make([][]interface{}, 0, len(persons))
	for _, p := range persons {
		rows = append(rows, []interface{}{
			p.ID,
			p.Apelido,
			p.Nome,
			p.Nascimento,
			p.StackAsText(),
			p.BuildSearchField(),
		})
	}

	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"pessoas"},
		[]string{"id", "apelido", "nome", "nascimento", "stack", "search_field"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return r.fallbackInsert(ctx, persons)
	}
	return nil
}

// fallbackInsert inserts each person individually with ON CONFLICT DO NOTHING.
func (r *PgRepository) fallbackInsert(ctx context.Context, persons []*Person) error {
	for _, p := range persons {
		_, err := r.pool.Exec(ctx,
			`INSERT INTO pessoas (id, apelido, nome, nascimento, stack, search_field)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT DO NOTHING`,
			p.ID, p.Apelido, p.Nome, p.Nascimento, p.StackAsText(), p.BuildSearchField())
		if err != nil {
			return fmt.Errorf("fallback insert for %s: %w", p.Apelido, err)
		}
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/person/
```

Expected: compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add PostgreSQL repository with COPY batch insert"
```

---

## Task 5: Channel-Based Batcher

**Files:**
- Create: `internal/person/batcher.go`
- Create: `internal/person/batcher_test.go`

- [ ] **Step 1: Write failing tests for batcher**

Create `internal/person/batcher_test.go`:

```go
package person

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mockRepository records all InsertBatch calls for testing.
type mockRepository struct {
	mu       sync.Mutex
	batches  [][]*Person
	errToReturn error
}

func (m *mockRepository) GetByID(ctx context.Context, id uuid.UUID) (*Person, error) {
	return nil, nil
}

func (m *mockRepository) Search(ctx context.Context, term string) ([]Person, error) {
	return nil, nil
}

func (m *mockRepository) Count(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockRepository) InsertBatch(ctx context.Context, persons []*Person) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batches = append(m.batches, persons)
	return m.errToReturn
}

func (m *mockRepository) totalInserted() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, b := range m.batches {
		total += len(b)
	}
	return total
}

func TestBatcher_FlushesOnShutdown(t *testing.T) {
	repo := &mockRepository{}
	b := NewBatcher(repo, 5000, 500, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go b.Run(ctx)

	for i := 0; i < 10; i++ {
		b.Add(&Person{
			ID:      uuid.New(),
			Apelido: uuid.New().String()[:8],
			Nome:    "Test",
			Nascimento: "2000-01-01",
		})
	}

	cancel()
	b.Wait()

	if repo.totalInserted() != 10 {
		t.Errorf("expected 10 inserted, got %d", repo.totalInserted())
	}
}

func TestBatcher_FlushesOnTick(t *testing.T) {
	repo := &mockRepository{}
	b := NewBatcher(repo, 5000, 500, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	b.Add(&Person{
		ID:      uuid.New(),
		Apelido: "tick-test",
		Nome:    "Test",
		Nascimento: "2000-01-01",
	})

	// Wait for ticker to fire
	time.Sleep(50 * time.Millisecond)

	if repo.totalInserted() < 1 {
		t.Error("expected at least 1 inserted after tick")
	}

	cancel()
	b.Wait()
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/person/ -v -run TestBatcher -race
```

Expected: compilation error — `NewBatcher`, `Batcher` not defined.

- [ ] **Step 3: Implement the batcher**

Create `internal/person/batcher.go`:

```go
package person

import (
	"context"
	"log"
	"sync"
	"time"
)

// Batcher accumulates Person records and batch-inserts them into the database.
type Batcher struct {
	repo      Repository
	ch        chan *Person
	maxBatch  int
	flushInterval time.Duration
	wg        sync.WaitGroup
}

// NewBatcher creates a new Batcher.
// chanSize: buffered channel capacity.
// maxBatch: flush when this many items are accumulated.
// flushInterval: flush at least this often.
func NewBatcher(repo Repository, chanSize, maxBatch int, flushInterval time.Duration) *Batcher {
	return &Batcher{
		repo:          repo,
		ch:            make(chan *Person, chanSize),
		maxBatch:      maxBatch,
		flushInterval: flushInterval,
	}
}

// Add sends a person to the batcher channel for later insertion.
func (b *Batcher) Add(p *Person) {
	b.ch <- p
}

// Run starts the batcher loop. It blocks until ctx is cancelled.
// Call Wait() after Run returns to ensure all items are flushed.
func (b *Batcher) Run(ctx context.Context) {
	b.wg.Add(1)
	defer b.wg.Done()

	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	buf := make([]*Person, 0, b.maxBatch)

	flush := func() {
		if len(buf) == 0 {
			return
		}
		batch := make([]*Person, len(buf))
		copy(batch, buf)
		buf = buf[:0]

		if err := b.repo.InsertBatch(context.Background(), batch); err != nil {
			log.Printf("batcher: insert error: %v", err)
		}
	}

	for {
		select {
		case p := <-b.ch:
			buf = append(buf, p)
			if len(buf) >= b.maxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			// Drain remaining items from channel
			close := false
			for !close {
				select {
				case p := <-b.ch:
					buf = append(buf, p)
				default:
					close = true
				}
			}
			flush()
			return
		}
	}
}

// Wait blocks until the batcher goroutine has finished.
func (b *Batcher) Wait() {
	b.wg.Wait()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/person/ -v -run TestBatcher -race
```

Expected: all tests PASS, no race conditions.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add channel-based batcher with periodic flush"
```

---

## Task 6: HTTP Handlers

**Files:**
- Create: `internal/person/handler.go`
- Create: `internal/person/handler_test.go`

- [ ] **Step 1: Write failing tests for handlers**

Create `internal/person/handler_test.go`:

```go
package person

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// testHandler creates a Handler with a fresh cache and mock repo.
func testHandler() (*Handler, *mockRepository) {
	cache := NewCache()
	repo := &mockRepository{}
	h := NewHandler(cache, repo)
	return h, repo
}

func TestHandler_Create_Valid(t *testing.T) {
	h, _ := testHandler()

	body := `{"apelido":"josé","nome":"José Roberto","nascimento":"2000-10-01","stack":["C#","Node"]}`
	req := httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Error("expected Location header")
	}
	if len(loc) < len("/pessoas/") {
		t.Errorf("Location header too short: %s", loc)
	}
}

func TestHandler_Create_DuplicateNickname(t *testing.T) {
	h, _ := testHandler()

	body := `{"apelido":"josé","nome":"José Roberto","nascimento":"2000-10-01","stack":null}`

	// First create
	req := httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create failed: %d", w.Code)
	}

	// Second create — same nickname
	req = httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestHandler_Create_BadRequest_NomeIsNumber(t *testing.T) {
	h, _ := testHandler()

	body := `{"apelido":"test","nome":1,"nascimento":"2000-01-01","stack":null}`
	req := httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Create_Unprocessable_MissingNome(t *testing.T) {
	h, _ := testHandler()

	body := `{"apelido":"test","nascimento":"2000-01-01","stack":null}`
	req := httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetByID_Found(t *testing.T) {
	h, _ := testHandler()

	// Create first
	body := `{"apelido":"ana","nome":"Ana Barbosa","nascimento":"1985-09-23","stack":null}`
	req := httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.Create(w, req)

	loc := w.Header().Get("Location")
	id := loc[len("/pessoas/"):]

	// Get
	req = httptest.NewRequest(http.MethodGet, "/pessoas/"+id, nil)
	w = httptest.NewRecorder()
	h.GetByID(w, req, id)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp PersonResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Apelido != "ana" {
		t.Errorf("expected apelido 'ana', got '%s'", resp.Apelido)
	}
}

func TestHandler_GetByID_NotFound(t *testing.T) {
	h, _ := testHandler()

	id := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/pessoas/"+id, nil)
	w := httptest.NewRecorder()
	h.GetByID(w, req, id)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_Search_MissingTerm(t *testing.T) {
	h, _ := testHandler()

	req := httptest.NewRequest(http.MethodGet, "/pessoas", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_Search_WithTerm(t *testing.T) {
	h, repo := testHandler()

	// Mock search returns empty
	_ = repo
	req := httptest.NewRequest(http.MethodGet, "/pessoas?t=node", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var results []PersonResponse
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	// Mock returns empty, so expect empty array
	if results == nil {
		t.Error("expected empty array, got nil")
	}
}

func TestHandler_Count(t *testing.T) {
	h, _ := testHandler()

	req := httptest.NewRequest(http.MethodGet, "/contagem-pessoas", nil)
	w := httptest.NewRecorder()
	h.Count(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "0" {
		t.Errorf("expected '0', got '%s'", w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/person/ -v -run TestHandler -race
```

Expected: compilation error — `Handler`, `NewHandler` not defined.

- [ ] **Step 3: Implement the handlers**

Create `internal/person/handler.go`:

```go
package person

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	cache   *Cache
	repo    Repository
	batcher *Batcher
}

// NewHandler creates a Handler for unit testing (no batcher).
func NewHandler(cache *Cache, repo Repository) *Handler {
	return &Handler{
		cache: cache,
		repo:  repo,
	}
}

// NewHandlerWithBatcher creates a Handler with all dependencies.
func NewHandlerWithBatcher(cache *Cache, repo Repository, batcher *Batcher) *Handler {
	return &Handler{
		cache:   cache,
		repo:    repo,
		batcher: batcher,
	}
}

// Create handles POST /pessoas.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	p, err := ParseAndValidate(body)
	if err != nil {
		if ve, ok := err.(*ValidationError); ok {
			writeError(w, ve.StatusCode, ve.Message)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !h.cache.Set(p) {
		writeError(w, http.StatusUnprocessableEntity, "apelido already exists")
		return
	}

	if h.batcher != nil {
		h.batcher.Add(p)
	}

	w.Header().Set("Location", fmt.Sprintf("/pessoas/%s", p.ID.String()))
	w.WriteHeader(http.StatusCreated)
}

// GetByID handles GET /pessoas/:id.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Try cache first
	if p, ok := h.cache.GetByID(id); ok {
		writeJSON(w, http.StatusOK, p.ToResponse())
		return
	}

	// Fallback to database
	p, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if p == nil {
		http.NotFound(w, r)
		return
	}

	// Store in cache for next time
	h.cache.Set(p)
	writeJSON(w, http.StatusOK, p.ToResponse())
}

// Search handles GET /pessoas?t=:term.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("t")
	if term == "" {
		writeError(w, http.StatusBadRequest, "query parameter 't' is required")
		return
	}

	people, err := h.repo.Search(r.Context(), term)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	responses := make([]PersonResponse, 0, len(people))
	for i := range people {
		responses = append(responses, people[i].ToResponse())
	}
	writeJSON(w, http.StatusOK, responses)
}

// Count handles GET /contagem-pessoas.
func (h *Handler) Count(w http.ResponseWriter, r *http.Request) {
	count, err := h.repo.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, count)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/person/ -v -run TestHandler -race
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add HTTP handlers for all API endpoints"
```

---

## Task 7: Server & Routing

**Files:**
- Create: `internal/server/server.go`

- [ ] **Step 1: Implement the server with route registration**

Create `internal/server/server.go`:

```go
package server

import (
	"net/http"
	"strings"

	"github.com/filipe1309/rinha-de-backend-1-2023/internal/person"
)

// NewServer creates an HTTP server with all routes registered.
func NewServer(addr string, handler *person.Handler) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /pessoas", handler.Create)

	mux.HandleFunc("GET /pessoas/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		handler.GetByID(w, r, id)
	})

	mux.HandleFunc("GET /pessoas", func(w http.ResponseWriter, r *http.Request) {
		// Distinguish search from list — search requires ?t= parameter
		if !strings.Contains(r.URL.RawQuery, "t=") {
			handler.Search(w, r) // Will return 400 since t is missing
			return
		}
		handler.Search(w, r)
	})

	mux.HandleFunc("GET /contagem-pessoas", handler.Count)

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/server/
```

Expected: compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add HTTP server with route registration"
```

---

## Task 8: Application Entry Point

**Files:**
- Create: `cmd/api/main.go`

- [ ] **Step 1: Implement main.go with graceful shutdown**

Create `cmd/api/main.go`:

```go
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

	// Connect to PostgreSQL
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("failed to parse database URL: %v", err)
	}
	poolConfig.MaxConns = 30
	poolConfig.MinConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Wait for database to be ready
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

	// Create dependencies
	cache := person.NewCache()
	repo := person.NewPgRepository(pool)
	batcher := person.NewBatcher(repo, 5000, 500, 5*time.Millisecond)
	handler := person.NewHandlerWithBatcher(cache, repo, batcher)

	// Start batcher
	batchCtx, batchCancel := context.WithCancel(context.Background())
	go batcher.Run(batchCtx)

	// Start HTTP server
	srv := server.NewServer(fmt.Sprintf(":%s", port), handler)

	go func() {
		log.Printf("server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")

	// Stop HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	// Stop batcher and wait for drain
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
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./cmd/api/
```

Expected: compiles without errors, produces `api` binary.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add application entry point with graceful shutdown"
```

---

## Task 9: Docker & Infrastructure

**Files:**
- Create: `Dockerfile`
- Create: `nginx.conf`
- Create: `docker-compose.yml`
- Create: `.dockerignore`

- [ ] **Step 1: Create Dockerfile**

Create `Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api/

# Runtime stage
FROM alpine:3.19

COPY --from=builder /api /api

EXPOSE 8080

CMD ["/api"]
```

- [ ] **Step 2: Create .dockerignore**

Create `.dockerignore`:

```
.git
docs
tests
*.md
```

- [ ] **Step 3: Create nginx.conf**

Create `nginx.conf`:

```nginx
events {
    worker_connections 1024;
}

http {
    upstream api {
        server api1:8080;
        server api2:8080;
    }

    server {
        listen 9999;

        location / {
            proxy_pass http://api;
        }
    }
}
```

- [ ] **Step 4: Create docker-compose.yml**

Create `docker-compose.yml`:

```yaml
version: '3.5'

services:
  api1:
    build: .
    hostname: api1
    depends_on:
      db:
        condition: service_healthy
    environment:
      - PORT=8080
      - DATABASE_URL=postgres://rinha:rinha@db:5432/rinha?sslmode=disable
    expose:
      - "8080"
    deploy:
      resources:
        limits:
          cpus: '0.20'
          memory: '0.3GB'

  api2:
    build: .
    hostname: api2
    depends_on:
      db:
        condition: service_healthy
    environment:
      - PORT=8080
      - DATABASE_URL=postgres://rinha:rinha@db:5432/rinha?sslmode=disable
    expose:
      - "8080"
    deploy:
      resources:
        limits:
          cpus: '0.20'
          memory: '0.3GB'

  nginx:
    image: nginx:latest
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - api1
      - api2
    ports:
      - "9999:9999"
    deploy:
      resources:
        limits:
          cpus: '0.10'
          memory: '0.2GB'

  db:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=rinha
      - POSTGRES_PASSWORD=rinha
      - POSTGRES_DB=rinha
    volumes:
      - ./db/init.sql:/docker-entrypoint-initdb.d/init.sql
    command: >
      postgres
        -c shared_buffers=512MB
        -c work_mem=64MB
        -c max_connections=200
        -c synchronous_commit=off
        -c checkpoint_completion_target=0.9
        -c effective_cache_size=1GB
    expose:
      - "5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U rinha"]
      interval: 2s
      timeout: 5s
      retries: 15
    deploy:
      resources:
        limits:
          cpus: '1.00'
          memory: '2.2GB'
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add Docker and infrastructure configuration"
```

---

## Task 10: Build & Smoke Test

**Files:**
- No new files.

- [ ] **Step 1: Build Docker images**

```bash
cd /Users/filipe1309/Projects/Personal/rinha-de-backend/rinha-de-backend-1-2023
docker compose build
```

Expected: successful build.

- [ ] **Step 2: Start the stack**

```bash
docker compose up -d
```

Expected: all 4 containers start.

- [ ] **Step 3: Smoke test — create a person**

```bash
curl -v -X POST http://localhost:9999/pessoas \
  -H "Content-Type: application/json" \
  -d '{"apelido":"josé","nome":"José Roberto","nascimento":"2000-10-01","stack":["C#","Node","Oracle"]}'
```

Expected: 201 Created with Location header.

- [ ] **Step 4: Smoke test — get person by ID**

Use the UUID from the Location header:

```bash
curl -v http://localhost:9999/pessoas/<uuid-from-step-3>
```

Expected: 200 OK with person JSON.

- [ ] **Step 5: Smoke test — search**

```bash
curl -v "http://localhost:9999/pessoas?t=node"
```

Expected: 200 OK with array containing the person.

- [ ] **Step 6: Smoke test — count**

```bash
curl -v http://localhost:9999/contagem-pessoas
```

Expected: 200 OK with "1".

- [ ] **Step 7: Smoke test — validation errors**

```bash
# 422 — null nome
curl -v -X POST http://localhost:9999/pessoas \
  -H "Content-Type: application/json" \
  -d '{"apelido":"test","nome":null,"nascimento":"2000-01-01","stack":null}'

# 400 — nome is number
curl -v -X POST http://localhost:9999/pessoas \
  -H "Content-Type: application/json" \
  -d '{"apelido":"test","nome":1,"nascimento":"2000-01-01","stack":null}'

# 422 — duplicate apelido
curl -v -X POST http://localhost:9999/pessoas \
  -H "Content-Type: application/json" \
  -d '{"apelido":"josé","nome":"Outro","nascimento":"2000-01-01","stack":null}'
```

Expected: 422, 400, 422 respectively.

- [ ] **Step 8: Stop the stack**

```bash
docker compose down
```

- [ ] **Step 9: Commit (if any fixes were needed)**

```bash
git add -A
git commit -m "fix: address issues found during smoke testing"
```

---

## Task 11: Integration Tests

**Files:**
- Create: `tests/integration_test.go`

- [ ] **Step 1: Install test dependencies**

```bash
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

- [ ] **Step 2: Write integration tests**

Create `tests/integration_test.go`:

```go
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
		postgres.WithInitScripts(), // We'll run init manually
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

	// Run init SQL
	if _, err := pool.Exec(ctx, string(initSQL)); err != nil {
		t.Fatalf("failed to run init.sql: %v", err)
	}

	cleanup := func() {
		pool.Close()
		pgContainer.Terminate(ctx)
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

func TestIntegration_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ts := setupTestServer(t, pool)

	// 1. Create a person
	body := `{"apelido":"josé","nome":"José Roberto","nascimento":"2000-10-01","stack":["C#","Node","Oracle"]}`
	resp, err := http.Post(ts.URL+"/pessoas", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
	}
	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}
	resp.Body.Close()

	// 2. Get by ID (from cache)
	resp, err = http.Get(ts.URL + location)
	if err != nil {
		t.Fatalf("GET by ID failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var p person.PersonResponse
	json.NewDecoder(resp.Body).Decode(&p)
	resp.Body.Close()

	if p.Apelido != "josé" {
		t.Errorf("expected apelido 'josé', got '%s'", p.Apelido)
	}
	if p.Nome != "José Roberto" {
		t.Errorf("expected nome 'José Roberto', got '%s'", p.Nome)
	}

	// 3. Wait for batcher to flush
	time.Sleep(50 * time.Millisecond)

	// 4. Search
	resp, err = http.Get(ts.URL + "/pessoas?t=node")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from search, got %d", resp.StatusCode)
	}
	var results []person.PersonResponse
	json.NewDecoder(resp.Body).Decode(&results)
	resp.Body.Close()

	if len(results) != 1 {
		t.Errorf("expected 1 search result, got %d", len(results))
	}

	// 5. Count
	resp, err = http.Get(ts.URL + "/contagem-pessoas")
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(b) != "1\n" && string(b) != "1" {
		t.Errorf("expected count '1', got '%s'", string(b))
	}
}

func TestIntegration_DuplicateNickname(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ts := setupTestServer(t, pool)

	body := `{"apelido":"unique","nome":"Test","nascimento":"2000-01-01","stack":null}`

	// First create
	resp, _ := http.Post(ts.URL+"/pessoas", "application/json", bytes.NewBufferString(body))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Duplicate
	resp, _ = http.Post(ts.URL+"/pessoas", "application/json", bytes.NewBufferString(body))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate: expected 422, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestIntegration_ConcurrentCreates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pool, cleanup := setupTestDB(t)
	defer cleanup()

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
			results <- resp.StatusCode
			resp.Body.Close()
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

	// Wait for batcher flush
	time.Sleep(100 * time.Millisecond)

	// Verify count in DB
	resp, _ := http.Get(ts.URL + "/contagem-pessoas")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	expected := fmt.Sprintf("%d", n)
	got := string(b)
	// Trim newline if present
	if len(got) > 0 && got[len(got)-1] == '\n' {
		got = got[:len(got)-1]
	}
	if got != expected {
		t.Errorf("expected count %s, got '%s'", expected, got)
	}
}
```

- [ ] **Step 3: Run integration tests**

```bash
go test ./tests/ -v -race -count=1
```

Expected: all tests PASS (requires Docker running).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test: add integration tests with testcontainers"
```

---

## Task 12: Run All Tests & Final Verification

**Files:**
- No new files.

- [ ] **Step 1: Run all unit tests**

```bash
go test ./internal/... -v -race
```

Expected: all tests PASS.

- [ ] **Step 2: Run integration tests**

```bash
go test ./tests/ -v -race -count=1
```

Expected: all tests PASS.

- [ ] **Step 3: Run full Docker Compose stack and verify**

```bash
docker compose up -d --build
sleep 5
# Create
curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:9999/pessoas \
  -H "Content-Type: application/json" \
  -d '{"apelido":"final","nome":"Final Test","nascimento":"2000-01-01","stack":["Go"]}'
# Should print: 201

# Count
curl -s http://localhost:9999/contagem-pessoas
# Should print: 1

docker compose down
```

- [ ] **Step 4: Final commit and tag**

```bash
git add -A
git commit -m "chore: final verification complete"
```
