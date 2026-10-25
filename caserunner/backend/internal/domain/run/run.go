// Package run holds the result model of a run and the rule that judges a
// case by its expect code (BRD-001 §12, §15.4).
package run

import (
	"maps"
	"slices"
	"time"
)

// Status is the state of one case.
type Status string

const (
	Pending   Status = "PENDING"
	Running   Status = "RUNNING"
	Success   Status = "SUCCESS"
	Failed    Status = "FAILED"
	Exception Status = "EXCEPTION"
	Blocked   Status = "BLOCKED"
	Cancelled Status = "CANCELLED"
)

// Finished reports whether a case has its final status.
func (s Status) Finished() bool { return s != Pending && s != Running }

// RunStatus is the state of a whole run.
type RunStatus string

const (
	RunRunning   RunStatus = "RUNNING"
	RunDone      RunStatus = "DONE"
	RunCancelled RunStatus = "CANCELLED"
)

// Validation error codes (BRD-001 §9).
const (
	CodeRequiredMissing    = "REQUIRED_MISSING"
	CodeTypeMismatch       = "TYPE_MISMATCH"
	CodeInvalidExpectation = "INVALID_EXPECTATION"
)

// ValidationError is one reason a case is BLOCKED.
type ValidationError struct {
	Field   string
	Code    string
	Message string
}

// Where an expect code was found (BRD-001 §12).
const (
	InStatus = "status"
	InHeader = "header"
	InBody   = "body"
)

// CaseResult is the outcome of one case.
type CaseResult struct {
	CaseIndex    int
	CaseNo       int
	Name         string
	Status       Status
	Reason       string
	Errors       []ValidationError // when BLOCKED
	ErrorType    string            // when EXCEPTION
	Input        map[string]string
	RequestBody  string
	Expect       string
	FoundIn      string
	HTTPStatus   int
	ResponseBody string
	Truncated    bool
	StartedAt    time.Time
	Duration     time.Duration
}

// Summary counts cases by status.
type Summary struct {
	Total, Pending, Running, Success, Failed, Exception, Blocked, Cancelled int
}

// Run is one execution of a set of cases.
type Run struct {
	ID         string
	Status     RunStatus
	CreatedAt  time.Time
	FinishedAt time.Time
	Results    []CaseResult // index = CaseIndex
}

// Summary counts the run's results by status.
func (r Run) Summary() Summary {
	s := Summary{Total: len(r.Results)}
	for _, c := range r.Results {
		switch c.Status {
		case Pending:
			s.Pending++
		case Running:
			s.Running++
		case Success:
			s.Success++
		case Failed:
			s.Failed++
		case Exception:
			s.Exception++
		case Blocked:
			s.Blocked++
		case Cancelled:
			s.Cancelled++
		}
	}
	return s
}

// Clone returns a deep copy, so a reader never shares memory with the writer.
func (r Run) Clone() Run {
	c := r
	c.Results = make([]CaseResult, len(r.Results))
	for i, res := range r.Results {
		res.Errors = slices.Clone(res.Errors)
		res.Input = maps.Clone(res.Input)
		c.Results[i] = res
	}
	return c
}
