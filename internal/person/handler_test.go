package person

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func testHandler() *Handler {
	cache := NewCache()
	repo := &mockRepository{}
	return NewHandler(cache, repo)
}

func TestHandler_Create_Valid(t *testing.T) {
	h := testHandler()

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
	h := testHandler()

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
	h := testHandler()

	body := `{"apelido":"test","nome":1,"nascimento":"2000-01-01","stack":null}`
	req := httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Create_Unprocessable_MissingNome(t *testing.T) {
	h := testHandler()

	body := `{"apelido":"test","nascimento":"2000-01-01","stack":null}`
	req := httptest.NewRequest(http.MethodPost, "/pessoas", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetByID_Found(t *testing.T) {
	h := testHandler()

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
	h := testHandler()

	id := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/pessoas/"+id, nil)
	w := httptest.NewRecorder()
	h.GetByID(w, req, id)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_Search_MissingTerm(t *testing.T) {
	h := testHandler()

	req := httptest.NewRequest(http.MethodGet, "/pessoas", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_Search_WithTerm(t *testing.T) {
	h := testHandler()

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
}

func TestHandler_Count(t *testing.T) {
	h := testHandler()

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
