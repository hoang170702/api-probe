package usecase

import "github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"

// Get returns the current state of a run, or ErrRunNotFound when it is
// unknown or has expired.
func (u *Runs) Get(id string) (run.Run, error) {
	r, ok := u.Repo.Get(id)
	if !ok {
		return run.Run{}, ErrRunNotFound
	}
	return r, nil
}
