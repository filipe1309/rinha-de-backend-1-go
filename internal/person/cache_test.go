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

func TestCache_SetReturnsFalseOnDuplicate(t *testing.T) {
	c := NewCache()
	p1 := &Person{ID: uuid.New(), Apelido: "josé", Nome: "José"}
	p2 := &Person{ID: uuid.New(), Apelido: "josé", Nome: "Outro José"}

	if !c.Set(p1) {
		t.Fatal("first Set should return true")
	}
	if c.Set(p2) {
		t.Fatal("second Set with same nickname should return false")
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
