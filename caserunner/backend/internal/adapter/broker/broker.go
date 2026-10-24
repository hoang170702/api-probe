// Package broker fans run events out to SSE watchers (BRD-001 §15.5).
package broker

import (
	"sync"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// Buffer is how many events a watcher may fall behind before it is dropped.
const Buffer = 64

// Broker implements usecase.EventPublisher.
type Broker struct {
	mu   sync.Mutex
	subs map[string]map[chan usecase.Event]struct{}
}

var _ usecase.EventPublisher = (*Broker)(nil)

// New returns an empty broker.
func New() *Broker { return &Broker{subs: map[string]map[chan usecase.Event]struct{}{}} }

// Publish never blocks a worker: a watcher whose buffer is full is dropped
// and its channel closed; it reconnects and gets a fresh snapshot.
func (b *Broker) Publish(runID string, ev usecase.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[runID] {
		select {
		case ch <- ev:
		default:
			delete(b.subs[runID], ch)
			close(ch)
		}
	}
}

// Subscribe returns a channel of the run's events and a function that
// stops the subscription; calling it more than once is safe.
func (b *Broker) Subscribe(runID string) (<-chan usecase.Event, func()) {
	ch := make(chan usecase.Event, Buffer)
	b.mu.Lock()
	if b.subs[runID] == nil {
		b.subs[runID] = map[chan usecase.Event]struct{}{}
	}
	b.subs[runID][ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[runID][ch]; ok {
			delete(b.subs[runID], ch)
			close(ch)
		}
		if len(b.subs[runID]) == 0 {
			delete(b.subs, runID)
		}
	}
}
