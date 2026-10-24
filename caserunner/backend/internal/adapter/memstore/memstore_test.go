package memstore

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// clock is a settable fake clock.
type clock struct{ t atomic.Int64 }

func (c *clock) now() time.Time          { return time.Unix(0, c.t.Load()) }
func (c *clock) advance(d time.Duration) { c.t.Add(int64(d)) }

func newRun(id string, n int) run.Run {
	return run.Run{ID: id, Status: run.RunRunning, Results: make([]run.CaseResult, n)}
}

func finish(t *testing.T, s *Store, id string) {
	t.Helper()
	if err := s.Update(id, func(r *run.Run) { r.Status = run.RunDone }); err != nil {
		t.Fatal(err)
	}
}

func TestGetReturnsACopy(t *testing.T) {
	c := &clock{}
	s := New(10, time.Hour, c.now)
	if err := s.Create(newRun("r1", 1)); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get("r1")
	got.Results[0].Status = run.Failed
	again, _ := s.Get("r1")
	if again.Results[0].Status != "" {
		t.Error("changing a Get result changed the stored run")
	}
	if _, ok := s.Get("nope"); ok {
		t.Error("unknown id found")
	}
	if err := s.Update("nope", func(*run.Run) {}); !errors.Is(err, usecase.ErrRunNotFound) {
		t.Errorf("Update unknown: %v", err)
	}
}

func TestEvictsTheOldestFinishedRun(t *testing.T) {
	c := &clock{}
	s := New(2, time.Hour, c.now)
	_ = s.Create(newRun("a", 0))
	_ = s.Create(newRun("b", 0))
	finish(t, s, "b")
	c.advance(time.Second)
	finish(t, s, "a") // a finished later than b
	if err := s.Create(newRun("c", 0)); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("b"); ok {
		t.Error("the oldest finished run (b) should have been evicted")
	}
	if _, ok := s.Get("a"); !ok {
		t.Error("a should remain")
	}
}

func TestFullWhenEveryRunIsInProgress(t *testing.T) {
	s := New(2, time.Hour, (&clock{}).now)
	_ = s.Create(newRun("a", 0))
	_ = s.Create(newRun("b", 0))
	if err := s.Create(newRun("c", 0)); !errors.Is(err, usecase.ErrStoreFull) {
		t.Errorf("err = %v, want ErrStoreFull", err)
	}
}

func TestTTL(t *testing.T) {
	c := &clock{}
	s := New(10, time.Hour, c.now)
	_ = s.Create(newRun("r", 0))
	c.advance(5 * time.Hour)
	if _, ok := s.Get("r"); !ok {
		t.Fatal("a running run never expires")
	}
	finish(t, s, "r")
	c.advance(59 * time.Minute)
	if _, ok := s.Get("r"); !ok {
		t.Fatal("expired too early")
	}
	c.advance(2 * time.Minute)
	if _, ok := s.Get("r"); ok {
		t.Error("finished run should expire after the TTL")
	}
}

func TestJanitorRemovesExpiredRuns(t *testing.T) {
	c := &clock{}
	s := New(10, time.Minute, c.now)
	_ = s.Create(newRun("r", 0))
	finish(t, s, "r")
	c.advance(2 * time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartJanitor(ctx, 5*time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for s.Len() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.Len() != 0 {
		t.Error("janitor did not remove the expired run")
	}
}

func TestConcurrentUpdatesAndReads(t *testing.T) {
	s := New(10, time.Hour, (&clock{}).now)
	_ = s.Create(newRun("r", 50))
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = s.Update("r", func(r *run.Run) { r.Results[i].Status = run.Success })
		}()
		go func() {
			defer wg.Done()
			if got, ok := s.Get("r"); ok {
				_ = got.Summary()
			}
		}()
	}
	wg.Wait()
	got, _ := s.Get("r")
	if sum := got.Summary(); sum.Success != 50 {
		t.Errorf("summary = %+v, want 50 successes", sum)
	}
}
