package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/filipe1309/rinha-de-backend-1-2023/internal/person"
)

type stubRepository struct {
	count int
}

func (s *stubRepository) GetByID(ctx context.Context, id uuid.UUID) (*person.Person, error) {
	return nil, nil
}

func (s *stubRepository) Search(ctx context.Context, term string) ([]person.Person, error) {
	return []person.Person{}, nil
}

func (s *stubRepository) Count(ctx context.Context) (int, error) {
	return s.count, nil
}

func (s *stubRepository) InsertBatch(ctx context.Context, persons []*person.Person) error {
	return nil
}

func TestNewServer_RegistersRoutes(t *testing.T) {
	repo := &stubRepository{count: 7}
	handler := person.NewHandler(person.NewCache(), repo)
	server := NewServer(":8080", handler)

	body := `{"apelido":"jose","nome":"Jose Roberto","nascimento":"2000-10-01","stack":["Go"]}`
	createReq := httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	server.Handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected POST /pessoas to return 201, got %d: %s", createRec.Code, createRec.Body.String())
	}

	location := createRec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header from POST /pessoas")
	}

	getByIDReq := httptest.NewRequest(http.MethodGet, location, nil)
	getByIDRec := httptest.NewRecorder()
	server.Handler.ServeHTTP(getByIDRec, getByIDReq)

	if getByIDRec.Code != http.StatusOK {
		t.Fatalf("expected GET /pessoas/{id} to return 200, got %d: %s", getByIDRec.Code, getByIDRec.Body.String())
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/pessoas?t=go", nil)
	searchRec := httptest.NewRecorder()
	server.Handler.ServeHTTP(searchRec, searchReq)

	if searchRec.Code != http.StatusOK {
		t.Fatalf("expected GET /pessoas to return 200, got %d: %s", searchRec.Code, searchRec.Body.String())
	}

	countReq := httptest.NewRequest(http.MethodGet, "/contagem-pessoas", nil)
	countRec := httptest.NewRecorder()
	server.Handler.ServeHTTP(countRec, countReq)

	if countRec.Code != http.StatusOK {
		t.Fatalf("expected GET /contagem-pessoas to return 200, got %d: %s", countRec.Code, countRec.Body.String())
	}
	if countRec.Body.String() != "7" {
		t.Fatalf("expected GET /contagem-pessoas body to be 7, got %q", countRec.Body.String())
	}
}
