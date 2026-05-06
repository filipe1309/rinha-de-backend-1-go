package person

import (
	"context"
	"log"
	"sync"
	"time"
)

// Batcher accumulates Person records and batch-inserts them into the database.
type Batcher struct {
	repo          Repository
	ch            chan *Person
	maxBatch      int
	flushInterval time.Duration
	wg            sync.WaitGroup
	stopped       chan struct{}
	stopOnce      sync.Once
}

// NewBatcher creates a new Batcher.
// chanSize: buffered channel capacity.
// maxBatch: flush when this many items are accumulated.
// flushInterval: flush at least this often.
func NewBatcher(repo Repository, chanSize, maxBatch int, flushInterval time.Duration) *Batcher {
	b := &Batcher{
		repo:          repo,
		ch:            make(chan *Person, chanSize),
		maxBatch:      maxBatch,
		flushInterval: flushInterval,
		stopped:       make(chan struct{}),
	}
	b.wg.Add(1)
	return b
}

// Add sends a person to the batcher channel for later insertion.
func (b *Batcher) Add(p *Person) {
	select {
	case <-b.stopped:
		return
	default:
	}

	select {
	case b.ch <- p:
	case <-b.stopped:
	}
}

// Run starts the batcher loop. It blocks until ctx is cancelled.
// Call Wait() after Run returns to ensure all items are flushed.
func (b *Batcher) Run(ctx context.Context) {
	defer b.wg.Done()

	stop := func() {
		b.stopOnce.Do(func() {
			close(b.stopped)
		})
	}
	defer stop()

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
			stop()
			draining := true
			for draining {
				select {
				case p := <-b.ch:
					buf = append(buf, p)
				default:
					draining = false
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
