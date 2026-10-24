// Package memstore keeps runs in memory (BRD-001 D9, §15.6): no database,
// finished runs expire after a TTL, and at most a fixed number are kept.
package memstore

import (
	"context"
	"sync"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// Store implements usecase.RunRepository.
type Store struct {
	mu   sync.Mutex
	runs map[string]*run.Run
	max  int
	ttl  time.Duration
	now  func() time.Time
}

var _ usecase.RunRepository = (*Store)(nil)

// New returns a store keeping at most max runs; a finished run expires ttl
// after it finished. now is the clock (time.Now in production).
func New(max int, ttl time.Duration, now func() time.Time) *Store {
	return &Store{runs: map[string]*run.Run{}, max: max, ttl: ttl, now: now}
}

// Create adds r. When the store is full the oldest finished run makes room;
// if every run is still in progress it returns usecase.ErrStoreFull.
func (s *Store) Create(r run.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked()
	if len(s.runs) >= s.max {
		var oldest *run.Run
		for _, cand := range s.runs {
			if cand.Status != run.RunRunning && (oldest == nil || cand.FinishedAt.Before(oldest.FinishedAt)) {
				oldest = cand
			}
		}
		if oldest == nil {
			return usecase.ErrStoreFull
		}
		delete(s.runs, oldest.ID)
	}
	c := r.Clone()
	s.runs[r.ID] = &c
	return nil
}

// Get returns a copy of the run, or false when it is unknown or expired.
func (s *Store) Get(id string) (run.Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok || s.expired(r) {
		return run.Run{}, false
	}
	return r.Clone(), true
}

// Update applies fn to the stored run under the lock. A run that leaves
// RUNNING gets its FinishedAt, which starts its TTL.
func (s *Store) Update(id string, fn func(*run.Run)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok || s.expired(r) {
		return usecase.ErrRunNotFound
	}
	fn(r)
	if r.Status != run.RunRunning && r.FinishedAt.IsZero() {
		r.FinishedAt = s.now()
	}
	return nil
}

// StartJanitor removes expired runs every interval until ctx ends.
func (s *Store) StartJanitor(ctx context.Context, every time.Duration) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.mu.Lock()
				s.expireLocked()
				s.mu.Unlock()
			}
		}
	}()
}

func (s *Store) expired(r *run.Run) bool {
	return r.Status != run.RunRunning && !r.FinishedAt.IsZero() && s.now().Sub(r.FinishedAt) > s.ttl
}

func (s *Store) expireLocked() {
	for id, r := range s.runs {
		if s.expired(r) {
			delete(s.runs, id)
		}
	}
}

// Len reports how many runs are held (for tests and diagnostics).
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.runs)
}
