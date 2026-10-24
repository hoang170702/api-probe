package usecase

import "github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"

// Watch returns the run's current state and a channel of later events
// (BRD-001 §15.5). It subscribes before reading the snapshot, so no event
// falls between the two; an event may repeat what the snapshot already
// shows, which watchers apply idempotently by caseIndex. When the snapshot
// is already finished, events is nil. Call stop when done watching.
func (u *Runs) Watch(id string) (snapshot run.Run, events <-chan Event, stop func(), err error) {
	ch, cancel := u.Events.Subscribe(id)
	r, ok := u.Repo.Get(id)
	if !ok {
		cancel()
		return run.Run{}, nil, func() {}, ErrRunNotFound
	}
	if r.Status != run.RunRunning {
		cancel()
		return r, nil, func() {}, nil
	}
	return r, ch, cancel, nil
}
