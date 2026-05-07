package person

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type searchCacheEntry struct {
	results []PersonResponse
	expiry  time.Time
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	cache       *Cache
	repo        Repository
	batcher     *Batcher
	searchCache sync.Map // map[string]*searchCacheEntry
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

	if p, ok := h.cache.GetByID(id); ok {
		writeJSON(w, http.StatusOK, p.ToResponse())
		return
	}

	p, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if p == nil {
		http.NotFound(w, r)
		return
	}

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

	cacheKey := strings.ToLower(term)
	if cached, ok := h.searchCache.Load(cacheKey); ok {
		entry := cached.(*searchCacheEntry)
		if time.Now().Before(entry.expiry) {
			writeJSON(w, http.StatusOK, entry.results)
			return
		}
		h.searchCache.Delete(cacheKey)
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

	h.searchCache.Store(cacheKey, &searchCacheEntry{
		results: responses,
		expiry:  time.Now().Add(15 * time.Second),
	})

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
