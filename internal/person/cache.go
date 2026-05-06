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
