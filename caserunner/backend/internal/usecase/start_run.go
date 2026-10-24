package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/field"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/flow"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/testcase"
)

// CodeUnknownVariable blocks a case whose request uses a {{name}} that no
// prepare step captures.
const CodeUnknownVariable = "UNKNOWN_VARIABLE"

// InputError is a run request that cannot start; it maps to HTTP 400.
type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

// Runs starts, follows and cancels runs (BRD-001 §11, §12). Start, Get,
// Cancel and Watch share the table of running runs' cancel functions.
type Runs struct {
	Repo   RunRepository
	Events EventPublisher
	Sender RequestSender
	Policy Policy
	Clock  Clock
	IDs    IDGenerator

	MaxCases           int
	MaxConcurrency     int
	DefaultConcurrency int

	// Base is the parent context of every run: runs outlive the HTTP
	// request that started them and stop when Base ends (shutdown).
	Base context.Context

	mu      sync.Mutex
	running map[string]*active
}

type active struct {
	cancel    context.CancelFunc
	cancelled bool
}

// StartRunInput is one run request (BRD-001 §15.3).
type StartRunInput struct {
	Template    curl.Template
	Rules       []testcase.FieldRule
	Cases       []testcase.Case
	Prepare     []PrepareStep
	InjectToken bool
	Concurrency int
	TimeoutMs   int
}

// StartRunOutput is returned as soon as the run is created.
type StartRunOutput struct {
	RunID   string
	Total   int
	Blocked int
}

// tokenVar is the variable Inject token reads (A05).
const tokenVar = "token"

// Start validates every case, creates the run and fires the valid cases in
// the background. It returns an *InputError for a request that cannot run
// and ErrStoreFull when too many runs are in progress.
func (u *Runs) Start(in StartRunInput) (StartRunOutput, error) {
	if err := u.check(in); err != nil {
		return StartRunOutput{}, err
	}
	known := map[string]bool{}
	for _, s := range in.Prepare {
		for _, c := range s.Captures {
			known[c.Var] = true
		}
	}

	r := run.Run{ID: u.IDs.NewID(), Status: run.RunRunning, CreatedAt: u.Clock.Now(), Results: make([]run.CaseResult, len(in.Cases))}
	built := make([]curl.Template, len(in.Cases))
	blocked := 0
	for i, c := range in.Cases {
		res := run.CaseResult{CaseIndex: i, CaseNo: c.No, Name: c.Name, Expect: c.Expect, Input: c.Values, Status: run.Pending}
		if res.CaseNo == 0 {
			res.CaseNo = i + 1
		}
		if res.Name == "" {
			res.Name = fmt.Sprintf("Case %d", res.CaseNo)
		}
		errs := testcase.Validate(c, in.Rules)
		if len(errs) == 0 {
			t, err := u.prebuild(in, c, known)
			if err != nil {
				errs = []run.ValidationError{*err}
			} else {
				built[i] = t
				res.RequestBody = t.BodyRaw
			}
		}
		if len(errs) > 0 {
			res.Status, res.Errors, res.Reason = run.Blocked, errs, blockedReason(errs)
			blocked++
		}
		r.Results[i] = res
	}
	// Register the cancel function before the run becomes visible, so a
	// Cancel that arrives right after Create always finds it.
	ctx, cancel := context.WithCancel(u.Base)
	u.mu.Lock()
	if u.running == nil {
		u.running = map[string]*active{}
	}
	u.running[r.ID] = &active{cancel: cancel}
	u.mu.Unlock()
	if err := u.Repo.Create(r); err != nil {
		u.mu.Lock()
		delete(u.running, r.ID)
		u.mu.Unlock()
		cancel()
		return StartRunOutput{}, err
	}

	go u.execute(ctx, r.ID, in, built)
	return StartRunOutput{RunID: r.ID, Total: len(in.Cases), Blocked: blocked}, nil
}

func (u *Runs) check(in StartRunInput) error {
	if len(in.Cases) == 0 || len(in.Cases) > u.MaxCases {
		return &InputError{fmt.Sprintf("Send between 1 and %d cases", u.MaxCases)}
	}
	if in.Template.BodyKind != curl.BodyJSON {
		return &InputError{"v1 runs JSON bodies only"}
	}
	root, err := field.Decode([]byte(in.Template.BodyRaw))
	if err != nil {
		return &InputError{"The template body is not valid JSON"}
	}
	for _, rule := range in.Rules {
		segs, err := field.ParsePath(rule.Path)
		if err != nil {
			return &InputError{fmt.Sprintf("Field %s: invalid path", rule.Path)}
		}
		if _, ok := field.Lookup(root, segs); !ok {
			return &InputError{fmt.Sprintf("Field %s is not in the template body", rule.Path)}
		}
	}
	if len(in.Prepare) > 10 {
		return &InputError{"Send at most 10 prepare steps"}
	}
	if len(in.Prepare) > 0 && in.InjectToken {
		found := false
		for _, s := range in.Prepare {
			for _, c := range s.Captures {
				found = found || c.Var == tokenVar
			}
		}
		if !found {
			return &InputError{"Inject token needs a capture named token"}
		}
	}
	return nil
}

// prebuild makes the case's request before any variable is known, and
// checks that every {{name}} it uses is captured by a prepare step.
func (u *Runs) prebuild(in StartRunInput, c testcase.Case, known map[string]bool) (curl.Template, *run.ValidationError) {
	t, err := testcase.Build(in.Template, in.Rules, c)
	if err != nil {
		return curl.Template{}, &run.ValidationError{Code: run.CodeTypeMismatch, Message: err.Error()}
	}
	if len(in.Prepare) > 0 && in.InjectToken {
		t = flow.InjectBearer(t, tokenVar)
	}
	var unknown []string
	for _, name := range flow.Placeholders(t) {
		if !known[name] {
			unknown = append(unknown, "{{"+name+"}}")
		}
	}
	if len(unknown) > 0 {
		return curl.Template{}, &run.ValidationError{
			Code:    CodeUnknownVariable,
			Message: fmt.Sprintf("Unknown variable %s: no token step captures it", strings.Join(unknown, ", ")),
		}
	}
	return t, nil
}

func blockedReason(errs []run.ValidationError) string {
	if len(errs) == 1 {
		return errs[0].Message
	}
	return fmt.Sprintf("%s (+%d more)", errs[0].Message, len(errs)-1)
}

// execute fires the pending cases: bounded in parallel, or one at a time
// with the prepare steps before each case (D21). It never retries.
func (u *Runs) execute(ctx context.Context, id string, in StartRunInput, built []curl.Template) {
	timeout := u.Policy.Timeout(in.TimeoutMs)
	snapshot, _ := u.Repo.Get(id)
	var pending []run.CaseResult
	for _, res := range snapshot.Results {
		if res.Status == run.Pending {
			pending = append(pending, res)
		}
	}

	if len(in.Prepare) > 0 {
		for _, res := range pending {
			if ctx.Err() != nil {
				break
			}
			u.runCase(ctx, id, res, in, built[res.CaseIndex], timeout)
		}
	} else {
		sem := make(chan struct{}, u.concurrency(in.Concurrency))
		var wg sync.WaitGroup
	loop:
		for _, res := range pending {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				break loop
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				u.runCase(ctx, id, res, in, built[res.CaseIndex], timeout)
			}()
		}
		wg.Wait()
	}
	u.finish(id)
}

func (u *Runs) concurrency(n int) int {
	switch {
	case n <= 0:
		n = u.DefaultConcurrency
	case n > u.MaxConcurrency:
		n = u.MaxConcurrency
	}
	return max(n, 1)
}

// finish marks never-started cases CANCELLED, sets the run's final status
// and publishes "done".
func (u *Runs) finish(id string) {
	u.mu.Lock()
	a := u.running[id]
	delete(u.running, id)
	u.mu.Unlock()
	cancelled := a != nil && a.cancelled
	if a != nil {
		a.cancel()
	}
	_ = u.Repo.Update(id, func(r *run.Run) {
		for i := range r.Results {
			if !r.Results[i].Status.Finished() {
				r.Results[i].Status = run.Cancelled
				r.Results[i].Reason = "The run was cancelled before this case was sent"
			}
		}
		r.Status = run.RunDone
		if cancelled {
			r.Status = run.RunCancelled
		}
	})
	if final, ok := u.Repo.Get(id); ok {
		u.Events.Publish(id, Event{Kind: EventDone, Run: &final})
	}
}

// runCase sends one case and records its result. A panic is caught so it
// cannot end the run (BRD-001 §19 case 6).
func (u *Runs) runCase(ctx context.Context, id string, res run.CaseResult, in StartRunInput, t curl.Template, timeout time.Duration) {
	defer func() {
		if p := recover(); p != nil {
			res.Status, res.ErrorType, res.Reason = run.Exception, ErrInternal, fmt.Sprintf("Internal error: %v", p)
			u.save(id, res)
		}
	}()
	res.Status, res.StartedAt = run.Running, u.Clock.Now()
	u.save(id, res)

	vars := map[string]string{}
	if len(in.Prepare) > 0 {
		steps, captured := runPrepare(ctx, u.Sender, u.Policy, in.Prepare, timeout)
		for n, s := range steps {
			if s.Status != StepOK {
				res.Status, res.ErrorType = run.Exception, ErrPrepareFailed
				if s.ErrorType == ErrCancelled {
					res.ErrorType = ErrCancelled
				}
				res.Reason = fmt.Sprintf("Prepare step %d failed: %s", n+1, s.Reason)
				u.done(id, res)
				return
			}
		}
		vars = captured
	}

	t, err := flow.Apply(t, vars)
	if err != nil { // prebuild already checked the names; this is a bug
		res.Status, res.ErrorType, res.Reason = run.Exception, ErrInternal, err.Error()
		u.done(id, res)
		return
	}
	req := OutboundRequest{Method: t.Method, URL: t.URL, Headers: t.Headers, Body: t.BodyRaw, Insecure: t.Insecure}
	res.RequestBody = req.Body
	if err := u.Policy.Check(req); err != nil {
		res.Status, res.ErrorType, res.Reason = run.Exception, ErrPolicy, capitalize(err.Error())
		u.done(id, res)
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := u.Clock.Now()
	resp, err := u.Sender.Send(reqCtx, req)
	res.Duration = u.Clock.Now().Sub(start)
	if err != nil {
		f := failedSend(err, req.URL, timeout)
		if errors.Is(ctx.Err(), context.Canceled) {
			f.ErrorType, f.Reason = ErrCancelled, "The run was cancelled while this case was in flight"
		}
		res.Status, res.ErrorType, res.Reason = run.Exception, f.ErrorType, f.Reason
		u.done(id, res)
		return
	}
	res.HTTPStatus, res.ResponseBody, res.Truncated = resp.Status, string(resp.Body), resp.Truncated
	if resp.Duration > 0 {
		res.Duration = resp.Duration
	}
	run.Judge(&res, resp.Headers, resp.Body)
	u.done(id, res)
}

func (u *Runs) save(id string, res run.CaseResult) {
	_ = u.Repo.Update(id, func(r *run.Run) { r.Results[res.CaseIndex] = res })
	u.Events.Publish(id, Event{Kind: EventCase, Case: &res})
}

func (u *Runs) done(id string, res run.CaseResult) { u.save(id, res) }
