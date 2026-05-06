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
	mu          sync.Mutex
	batches     [][]*Person
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
			ID:         uuid.New(),
			Apelido:    uuid.New().String()[:8],
			Nome:       "Test",
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
		ID:         uuid.New(),
		Apelido:    "tick-test",
		Nome:       "Test",
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

func TestBatcher_FlushesOnMaxBatch(t *testing.T) {
	repo := &mockRepository{}
	b := NewBatcher(repo, 5000, 5, 1*time.Second) // maxBatch=5, long tick

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	for i := 0; i < 5; i++ {
		b.Add(&Person{
			ID:         uuid.New(),
			Apelido:    uuid.New().String()[:8],
			Nome:       "Test",
			Nascimento: "2000-01-01",
		})
	}

	// Should flush quickly due to batch size
	time.Sleep(50 * time.Millisecond)

	if repo.totalInserted() != 5 {
		t.Errorf("expected 5 inserted after max batch, got %d", repo.totalInserted())
	}

	cancel()
	b.Wait()
}

func TestBatcher_AddAfterShutdownDoesNotBlock(t *testing.T) {
	repo := &mockRepository{}
	b := NewBatcher(repo, 0, 1, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	go b.Run(ctx)
	cancel()
	b.Wait()

	done := make(chan struct{})
	go func() {
		b.Add(&Person{
			ID:         uuid.New(),
			Apelido:    "after-stop",
			Nome:       "Test",
			Nascimento: "2000-01-01",
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected Add not to block after shutdown")
	}
}
