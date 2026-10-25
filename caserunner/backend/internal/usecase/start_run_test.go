package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/field"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/flow"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/testcase"
)

// --- fakes of the ports ---

type memRepo struct {
	mu   sync.Mutex
	runs map[string]*run.Run
	full bool
}

func (m *memRepo) Create(r run.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.full {
		return ErrStoreFull
	}
	if m.runs == nil {
		m.runs = map[string]*run.Run{}
	}
	c := r.Clone()
	m.runs[r.ID] = &c
	return nil
}

func (m *memRepo) Get(id string) (run.Run, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return run.Run{}, false
	}
	return r.Clone(), true
}

func (m *memRepo) Update(id string, fn func(*run.Run)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return ErrRunNotFound
	}
	fn(r)
	return nil
}

type recorder struct {
	mu   sync.Mutex
	evs  []Event
	subs []chan Event
}

func (p *recorder) Publish(_ string, ev Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evs = append(p.evs, ev)
	for _, ch := range p.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (p *recorder) Subscribe(string) (<-chan Event, func()) {
	ch := make(chan Event, 1024)
	p.mu.Lock()
	p.subs = append(p.subs, ch)
	p.mu.Unlock()
	return ch, func() {}
}

type sysClock struct{}

func (sysClock) Now() time.Time { return time.Now() }

type seqIDs struct{ n atomic.Int64 }

func (s *seqIDs) NewID() string { return fmt.Sprintf("r_%d", s.n.Add(1)) }

// apiFake answers with fn and tracks calls and requests in flight.
type apiFake struct {
	fn       func(ctx context.Context, req OutboundRequest) (InboundResponse, error)
	calls    atomic.Int64
	inFlight atomic.Int64
	maxSeen  atomic.Int64
	mu       sync.Mutex
	sent     []OutboundRequest
}

func (a *apiFake) Send(ctx context.Context, req OutboundRequest) (InboundResponse, error) {
	a.calls.Add(1)
	a.mu.Lock()
	a.sent = append(a.sent, req)
	a.mu.Unlock()
	n := a.inFlight.Add(1)
	defer a.inFlight.Add(-1)
	for {
		m := a.maxSeen.Load()
		if n <= m || a.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	return a.fn(ctx, req)
}

func ok(body string) func(context.Context, OutboundRequest) (InboundResponse, error) {
	return func(context.Context, OutboundRequest) (InboundResponse, error) {
		return InboundResponse{Status: 200, Body: []byte(body)}, nil
	}
}

// waitCtx blocks until the request's context ends, like a server that never answers.
func waitCtx(ctx context.Context, _ OutboundRequest) (InboundResponse, error) {
	<-ctx.Done()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return InboundResponse{}, &SendError{Type: ErrTimeout, Err: ctx.Err()}
	}
	return InboundResponse{}, &SendError{Type: ErrCancelled, Err: ctx.Err()}
}

// --- helpers ---

const tplBody = `{"requestId":"REQ001","data":{"amount":100000,"note":"test"}}`

func tpl() curl.Template {
	return curl.Template{
		Method: "POST", URL: "https://api.local/transfers",
		Headers: []curl.Header{{Name: "Content-Type", Value: "application/json"}},
		BodyRaw: tplBody, BodyKind: curl.BodyJSON,
	}
}

func rules() []testcase.FieldRule {
	return []testcase.FieldRule{
		{Path: "requestId", Type: field.TypeString, Required: true},
		{Path: "data.amount", Type: field.TypeNumber, Required: true},
		{Path: "data.note", Type: field.TypeString, Required: false},
	}
}

func cs(expect string, kv ...string) testcase.Case {
	v := map[string]string{"requestId": "REQ001", "data.amount": "100000", "data.note": "test"}
	for i := 0; i < len(kv); i += 2 {
		v[kv[i]] = kv[i+1]
	}
	return testcase.Case{Values: v, Expect: expect}
}

func newRuns(api *apiFake) (*Runs, *memRepo, *recorder) {
	repo, pub := &memRepo{}, &recorder{}
	return &Runs{
		Repo: repo, Events: pub, Sender: api, Clock: sysClock{}, IDs: &seqIDs{},
		Policy:   Policy{DefaultTimeout: 2 * time.Second, MaxTimeout: 5 * time.Second},
		MaxCases: 1000, MaxConcurrency: 20, DefaultConcurrency: 5, Base: context.Background(),
	}, repo, pub
}

func waitDone(t *testing.T, repo *memRepo, id string) run.Run {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if r, _ := repo.Get(id); r.Status != run.RunRunning {
			return r
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("run %s did not finish", id)
	return run.Run{}
}

func start(t *testing.T, u *Runs, in StartRunInput) StartRunOutput {
	t.Helper()
	if in.Template.URL == "" {
		in.Template = tpl()
	}
	if in.Rules == nil {
		in.Rules = rules()
	}
	out, err := u.Start(in)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return out
}

// --- tests ---

func TestRunJudgesEveryCase(t *testing.T) {
	api := &apiFake{fn: func(_ context.Context, req OutboundRequest) (InboundResponse, error) {
		switch {
		case strings.Contains(req.Body, `"amount":0`):
			return InboundResponse{Status: 400, Body: []byte(`{"code":"ERR_AMOUNT_INVALID"}`)}, nil
		case strings.Contains(req.Body, `"amount":7`):
			return InboundResponse{Status: 200, Body: []byte(`{"code":"99"}`)}, nil
		default:
			return InboundResponse{Status: 200, Body: []byte(`{"code":"00"}`)}, nil
		}
	}}
	u, repo, pub := newRuns(api)
	out := start(t, u, StartRunInput{Cases: []testcase.Case{
		cs(`"code":"00"`),
		cs("ERR_AMOUNT_INVALID", "data.amount", "0"),
		cs(`"code":"00"`, "data.amount", "7"),
		cs("00", "requestId", ""),   // Required empty → BLOCKED (D4)
		cs("00", "data.note", ""),   // Unrequired empty → key removed (D5)
		cs("", "data.amount", "12"), // empty expect → BLOCKED (D15)
	}})
	if out.Total != 6 || out.Blocked != 2 {
		t.Errorf("out = %+v", out)
	}
	r := waitDone(t, repo, out.RunID)
	want := []run.Status{run.Success, run.Success, run.Failed, run.Blocked, run.Success, run.Blocked}
	for i, w := range want {
		if r.Results[i].Status != w {
			t.Errorf("case %d: %s (%s), want %s", i+1, r.Results[i].Status, r.Results[i].Reason, w)
		}
	}
	if got := r.Results[1].Reason; got != `Found ERR_AMOUNT_INVALID in the body (HTTP 400)` {
		t.Errorf("reason = %q", got)
	}
	if r.Results[3].Errors[0].Code != run.CodeRequiredMissing || r.Results[5].Errors[0].Code != run.CodeInvalidExpectation {
		t.Errorf("blocked errors: %+v / %+v", r.Results[3].Errors, r.Results[5].Errors)
	}
	if api.calls.Load() != 4 {
		t.Errorf("sent %d requests, want 4 (BLOCKED cases are never sent)", api.calls.Load())
	}
	for _, req := range api.sent {
		if strings.Contains(req.Body, `"amount":100000`) && !strings.Contains(req.Body, `"note"`) {
			return // case 5: the note key is absent
		}
	}
	t.Error("no request without the note key: an empty Unrequired field must be removed")
	_ = pub
}

func TestDoneEventAndRunStatus(t *testing.T) {
	u, repo, pub := newRuns(&apiFake{fn: ok(`{"code":"00"}`)})
	out := start(t, u, StartRunInput{Cases: []testcase.Case{cs("00"), cs("00")}})
	r := waitDone(t, repo, out.RunID)
	if r.Status != run.RunDone || r.Summary().Success != 2 {
		t.Fatalf("run = %s %+v", r.Status, r.Summary())
	}
	pub.mu.Lock()
	defer pub.mu.Unlock()
	last := pub.evs[len(pub.evs)-1]
	if last.Kind != EventDone || last.Run.Summary().Success != 2 {
		t.Errorf("last event = %+v", last)
	}
}

// BRD-001 §19 case 5.
func TestConcurrencyNeverExceedsTheLimit(t *testing.T) {
	api := &apiFake{fn: func(context.Context, OutboundRequest) (InboundResponse, error) {
		time.Sleep(3 * time.Millisecond)
		return InboundResponse{Status: 200, Body: []byte(`{"code":"00"}`)}, nil
	}}
	u, repo, _ := newRuns(api)
	cases := make([]testcase.Case, 100)
	for i := range cases {
		cases[i] = cs("00")
	}
	out := start(t, u, StartRunInput{Cases: cases, Concurrency: 5})
	waitDone(t, repo, out.RunID)
	if m := api.maxSeen.Load(); m > 5 || m < 2 {
		t.Errorf("max in flight = %d, want 2..5", m)
	}
	if api.calls.Load() != 100 {
		t.Errorf("calls = %d", api.calls.Load())
	}
}

func TestConcurrencyIsClamped(t *testing.T) {
	u, _, _ := newRuns(nil)
	for n, want := range map[int]int{0: 5, -3: 5, 3: 3, 99: 20} {
		if got := u.concurrency(n); got != want {
			t.Errorf("concurrency(%d) = %d, want %d", n, got, want)
		}
	}
}

// BRD-001 §19 case 4.
func TestTimeoutIsNotRetried(t *testing.T) {
	api := &apiFake{fn: waitCtx}
	u, repo, _ := newRuns(api)
	out := start(t, u, StartRunInput{Cases: []testcase.Case{cs("00")}, TimeoutMs: 50})
	r := waitDone(t, repo, out.RunID)
	res := r.Results[0]
	if res.Status != run.Exception || res.ErrorType != ErrTimeout || !strings.Contains(res.Reason, "Cannot determine whether the server processed the request") {
		t.Errorf("result = %s %s %q", res.Status, res.ErrorType, res.Reason)
	}
	if api.calls.Load() != 1 {
		t.Errorf("calls = %d, want exactly 1 (no retry)", api.calls.Load())
	}
}

// BRD-001 §19 case 6.
func TestPanicInOneCaseDoesNotStopTheRun(t *testing.T) {
	api := &apiFake{fn: func(_ context.Context, req OutboundRequest) (InboundResponse, error) {
		if strings.Contains(req.Body, "boom") {
			panic("boom")
		}
		return InboundResponse{Status: 200, Body: []byte(`{"code":"00"}`)}, nil
	}}
	u, repo, _ := newRuns(api)
	out := start(t, u, StartRunInput{Cases: []testcase.Case{cs("00"), cs("00", "requestId", "boom"), cs("00")}})
	r := waitDone(t, repo, out.RunID)
	if r.Results[1].Status != run.Exception || r.Results[1].ErrorType != ErrInternal {
		t.Errorf("panicking case = %+v", r.Results[1])
	}
	if r.Results[0].Status != run.Success || r.Results[2].Status != run.Success {
		t.Errorf("other cases: %s %s", r.Results[0].Status, r.Results[2].Status)
	}
}

// BRD-001 §19 case 7.
func TestCancel(t *testing.T) {
	api := &apiFake{fn: waitCtx}
	u, repo, _ := newRuns(api)
	cases := make([]testcase.Case, 10)
	for i := range cases {
		cases[i] = cs("00")
	}
	out := start(t, u, StartRunInput{Cases: cases, Concurrency: 2})
	for api.inFlight.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	if err := u.Cancel(out.RunID); err != nil {
		t.Fatal(err)
	}
	r := waitDone(t, repo, out.RunID)
	s := r.Summary()
	if r.Status != run.RunCancelled || s.Exception != 2 || s.Cancelled != 8 {
		t.Fatalf("run %s %+v", r.Status, s)
	}
	for _, res := range r.Results {
		if res.Status == run.Exception && res.ErrorType != ErrCancelled {
			t.Errorf("in-flight case errorType = %s", res.ErrorType)
		}
	}
	if err := u.Cancel(out.RunID); err != nil {
		t.Errorf("cancelling a finished run: %v", err)
	}
	if err := u.Cancel("r_unknown"); !errors.Is(err, ErrRunNotFound) {
		t.Errorf("unknown run: %v", err)
	}
}

// BRD-001 §19 case 10 and D21.
func TestPrepareStepsRunBeforeEveryCase(t *testing.T) {
	var logins atomic.Int64
	api := &apiFake{fn: func(_ context.Context, req OutboundRequest) (InboundResponse, error) {
		if req.URL == "https://auth.local/login" {
			n := logins.Add(1)
			if n == 3 {
				return InboundResponse{Status: 401, Body: []byte(`{"code":"AUTH"}`)}, nil
			}
			return InboundResponse{Status: 200, Body: []byte(fmt.Sprintf(`{"data":{"accessToken":"tok-%d"}}`, n))}, nil
		}
		return InboundResponse{Status: 200, Body: []byte(`{"code":"00"}`)}, nil
	}}
	u, repo, _ := newRuns(api)
	login := PrepareStep{Template: curl.Template{Method: "POST", URL: "https://auth.local/login", BodyKind: curl.BodyNone}, Captures: []flow.Capture{{Var: "token"}}}
	tp := tpl()
	tp.Headers = append(tp.Headers, curl.Header{Name: "Authorization", Value: "Bearer stale"})
	out := start(t, u, StartRunInput{
		Template: tp, Prepare: []PrepareStep{login}, InjectToken: true, Concurrency: 20,
		Cases: []testcase.Case{cs("00"), cs("00"), cs("00"), cs("00")},
	})
	r := waitDone(t, repo, out.RunID)
	want := []run.Status{run.Success, run.Success, run.Exception, run.Success}
	for i, w := range want {
		if r.Results[i].Status != w {
			t.Errorf("case %d = %s (%s), want %s", i+1, r.Results[i].Status, r.Results[i].Reason, w)
		}
	}
	if r.Results[2].ErrorType != ErrPrepareFailed || !strings.Contains(r.Results[2].Reason, "Prepare step 1 failed: Expected a 2xx response, got HTTP 401") {
		t.Errorf("failed prepare: %+v", r.Results[2])
	}
	var tokens []string
	for _, req := range api.sent {
		if req.URL != "https://auth.local/login" {
			var auth []string
			for _, h := range req.Headers {
				if strings.EqualFold(h.Name, "Authorization") {
					auth = append(auth, h.Value)
				}
			}
			if len(auth) != 1 {
				t.Fatalf("Authorization headers = %q, want exactly the injected one", auth)
			}
			tokens = append(tokens, auth[0])
		}
	}
	if strings.Join(tokens, ",") != "Bearer tok-1,Bearer tok-2,Bearer tok-4" {
		t.Errorf("tokens = %q, want a fresh one per case", tokens)
	}
	if api.maxSeen.Load() != 1 {
		t.Errorf("max in flight = %d, want 1: with prepare steps cases run one at a time", api.maxSeen.Load())
	}
}

func TestUnknownVariableBlocksTheCase(t *testing.T) {
	api := &apiFake{fn: ok(`{"code":"00"}`)}
	u, repo, _ := newRuns(api)
	out := start(t, u, StartRunInput{Cases: []testcase.Case{cs("00", "data.note", "{{otp}}"), cs("00")}})
	r := waitDone(t, repo, out.RunID)
	if r.Results[0].Status != run.Blocked || r.Results[0].Errors[0].Code != CodeUnknownVariable {
		t.Errorf("case 1 = %+v", r.Results[0])
	}
	if r.Results[1].Status != run.Success {
		t.Errorf("case 2 = %s", r.Results[1].Status)
	}
}

func TestPolicyAppliesToCases(t *testing.T) {
	api := &apiFake{fn: ok(`{"code":"00"}`)}
	u, repo, _ := newRuns(api)
	u.Policy.AllowedHosts = []string{"*.uat.local"}
	out := start(t, u, StartRunInput{Cases: []testcase.Case{cs("00")}})
	r := waitDone(t, repo, out.RunID)
	if r.Results[0].Status != run.Exception || r.Results[0].ErrorType != ErrPolicy || api.calls.Load() != 0 {
		t.Errorf("result = %+v, calls %d", r.Results[0], api.calls.Load())
	}
}

func TestStartRejectsBadRequests(t *testing.T) {
	u, _, _ := newRuns(&apiFake{fn: ok("{}")})
	u.MaxCases = 2
	nonJSON := tpl()
	nonJSON.BodyKind = curl.BodyUnsupported
	login := PrepareStep{Template: curl.Template{Method: "POST", URL: "https://a/login"}, Captures: []flow.Capture{{Var: "jwt"}}}
	tests := []struct {
		name string
		in   StartRunInput
		want string
	}{
		{"no cases", StartRunInput{Template: tpl(), Rules: rules()}, "between 1 and 2 cases"},
		{"too many cases", StartRunInput{Template: tpl(), Rules: rules(), Cases: []testcase.Case{cs("0"), cs("0"), cs("0")}}, "between 1 and 2 cases"},
		{"not JSON", StartRunInput{Template: nonJSON, Rules: rules(), Cases: []testcase.Case{cs("0")}}, "JSON bodies only"},
		{"unknown field", StartRunInput{Template: tpl(), Rules: []testcase.FieldRule{{Path: "data.x", Type: field.TypeString}}, Cases: []testcase.Case{cs("0")}}, "not in the template body"},
		{"inject without a token capture", StartRunInput{Template: tpl(), Rules: rules(), Cases: []testcase.Case{cs("0")}, Prepare: []PrepareStep{login}, InjectToken: true}, "needs a capture named token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := u.Start(tt.in)
			var ie *InputError
			if !errors.As(err, &ie) || !strings.Contains(ie.Message, tt.want) {
				t.Errorf("err = %v, want an InputError containing %q", err, tt.want)
			}
		})
	}
}

func TestStartWhenTheStoreIsFull(t *testing.T) {
	u, repo, _ := newRuns(&apiFake{fn: ok("{}")})
	repo.full = true
	if _, err := u.Start(StartRunInput{Template: tpl(), Rules: rules(), Cases: []testcase.Case{cs("0")}}); !errors.Is(err, ErrStoreFull) {
		t.Errorf("err = %v", err)
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.running) != 0 {
		t.Error("a run that could not be created must not stay registered")
	}
}

func TestWatch(t *testing.T) {
	release := make(chan struct{})
	api := &apiFake{fn: func(context.Context, OutboundRequest) (InboundResponse, error) {
		<-release
		return InboundResponse{Status: 200, Body: []byte(`{"code":"00"}`)}, nil
	}}
	u, repo, _ := newRuns(api)
	out := start(t, u, StartRunInput{Cases: []testcase.Case{cs("00"), cs("00")}})
	snap, events, stop, err := u.Watch(out.RunID)
	if err != nil || snap.Status != run.RunRunning || events == nil {
		t.Fatalf("Watch: %v %s %v", err, snap.Status, events)
	}
	defer stop()
	close(release)
	var done bool
	for ev := range events {
		if ev.Kind == EventDone {
			done = true
			break
		}
	}
	if !done {
		t.Fatal("no done event")
	}
	waitDone(t, repo, out.RunID)
	snap, events, _, err = u.Watch(out.RunID)
	if err != nil || snap.Status != run.RunDone || events != nil {
		t.Errorf("Watch of a finished run: %v %s %v", err, snap.Status, events)
	}
	if _, _, _, err := u.Watch("r_unknown"); !errors.Is(err, ErrRunNotFound) {
		t.Errorf("unknown: %v", err)
	}
	if _, err := u.Get("r_unknown"); !errors.Is(err, ErrRunNotFound) {
		t.Errorf("Get unknown: %v", err)
	}
}
