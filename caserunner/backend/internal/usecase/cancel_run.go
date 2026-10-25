package usecase

// Cancel stops a run (BRD-001 §11.2): cases not yet sent become CANCELLED,
// cases in flight end as EXCEPTION/CANCELLED. Cancelling a finished run
// does nothing; an unknown run is ErrRunNotFound.
func (u *Runs) Cancel(id string) error {
	u.mu.Lock()
	a := u.running[id]
	if a != nil {
		a.cancelled = true
	}
	u.mu.Unlock()
	if a != nil {
		a.cancel()
		return nil
	}
	if _, ok := u.Repo.Get(id); !ok {
		return ErrRunNotFound
	}
	return nil
}
