package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/flow"
)

// fakeSender answers by URL and records what it was sent.
type fakeSender struct {
	responses map[string]InboundResponse
	errs      map[string]error
	sent      []OutboundRequest
}

func (f *fakeSender) Send(_ context.Context, req OutboundRequest) (InboundResponse, error) {
	f.sent = append(f.sent, req)
	if err, ok := f.errs[req.URL]; ok {
		return InboundResponse{}, err
	}
	return f.responses[req.URL], nil
}

var openPolicy = Policy{DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second}

func step(url string, captures ...flow.Capture) PrepareStep {
	return PrepareStep{Template: curl.Template{Method: "POST", URL: url, BodyKind: curl.BodyNone}, Captures: captures}
}

func TestTryPrepareChainsVariables(t *testing.T) {
	s := &fakeSender{responses: map[string]InboundResponse{
		"https://auth.local/login":        {Status: 200, Body: []byte(`{"data":{"accessToken":"T1"}}`)},
		"https://auth.local/otp?token=T1": {Status: 201, Body: []byte(`{"session":{"id":"S9"}}`)},
	}}
	second := step("https://auth.local/otp?token={{token}}", flow.Capture{Var: "session", Path: "session.id"})
	second.Template.Headers = []curl.Header{{Name: "Authorization", Value: "Bearer {{token}}"}}

	out := TryPrepare{Sender: s, Policy: openPolicy}.Execute(context.Background(),
		[]PrepareStep{step("https://auth.local/login", flow.Capture{Var: "token"}), second}, 0)

	if out.Steps[0].Status != StepOK || out.Steps[1].Status != StepOK {
		t.Fatalf("steps = %+v", out.Steps)
	}
	if got := out.Steps[0].Captured[0]; got.Path != "data.accessToken" || got.Value != "T1" {
		t.Errorf("auto-detected token = %+v", got)
	}
	if out.Vars["token"] != "T1" || out.Vars["session"] != "S9" {
		t.Errorf("vars = %v", out.Vars)
	}
	if h := s.sent[1].Headers[0].Value; h != "Bearer T1" {
		t.Errorf("second step was sent Authorization %q", h)
	}
}

func TestTryPrepareStopsAtTheFirstFailure(t *testing.T) {
	tests := []struct {
		name      string
		sender    *fakeSender
		policy    Policy
		steps     []PrepareStep
		wantType  string
		wantInMsg string
	}{
		{
			name:      "non-2xx",
			sender:    &fakeSender{responses: map[string]InboundResponse{"https://a.local/login": {Status: 401, Body: []byte(`{"code":"AUTH_FAILED"}`)}}},
			steps:     []PrepareStep{step("https://a.local/login", flow.Capture{Var: "token"})},
			wantInMsg: "Expected a 2xx response, got HTTP 401",
		},
		{
			name:      "capture not found",
			sender:    &fakeSender{responses: map[string]InboundResponse{"https://a.local/login": {Status: 200, Body: []byte(`{"ok":true}`)}}},
			steps:     []PrepareStep{step("https://a.local/login", flow.Capture{Var: "token", Path: "data.token"})},
			wantInMsg: "Token: no value at data.token",
		},
		{
			name:      "unknown variable",
			sender:    &fakeSender{},
			steps:     []PrepareStep{step("https://a.local/x?t={{nope}}")},
			wantInMsg: "Unknown variable {{nope}}",
		},
		{
			name:      "host not allowed",
			sender:    &fakeSender{},
			policy:    Policy{AllowedHosts: []string{"*.uat.local"}, DefaultTimeout: time.Second, MaxTimeout: time.Second},
			steps:     []PrepareStep{step("https://prod.bank.com/login")},
			wantType:  ErrPolicy,
			wantInMsg: "Host prod.bank.com is not in CASERUNNER_ALLOWED_HOSTS",
		},
		{
			name:      "timeout",
			sender:    &fakeSender{errs: map[string]error{"https://a.local/login": &SendError{Type: ErrTimeout, Err: context.DeadlineExceeded}}},
			steps:     []PrepareStep{step("https://a.local/login")},
			wantType:  ErrTimeout,
			wantInMsg: "Timed out after 1s",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := tt.policy
			if policy.MaxTimeout == 0 {
				policy = openPolicy
			}
			steps := append(tt.steps, step("https://a.local/never"))
			out := TryPrepare{Sender: tt.sender, Policy: policy}.Execute(context.Background(), steps, 0)

			first := out.Steps[0]
			if first.Status != StepFailed || first.ErrorType != tt.wantType || !strings.Contains(first.Reason, tt.wantInMsg) {
				t.Errorf("first step = %+v, want FAILED %q containing %q", first, tt.wantType, tt.wantInMsg)
			}
			if out.Steps[1].Status != StepSkipped {
				t.Errorf("second step = %+v, want SKIPPED", out.Steps[1])
			}
			for _, r := range tt.sender.sent {
				if r.URL == "https://a.local/never" || strings.Contains(r.URL, "prod.bank.com") {
					t.Errorf("%s was sent", r.URL)
				}
			}
		})
	}
}

func TestPolicy(t *testing.T) {
	p := Policy{AllowedHosts: []string{"localhost", "*.UAT.example.com", "10.10.*"}, DefaultTimeout: 30 * time.Second, MaxTimeout: time.Minute}
	for url, allowed := range map[string]bool{
		"http://localhost:8080/x":          true,
		"https://api.uat.example.com/x":    true,
		"https://a.b.uat.example.com/x":    true,
		"http://10.10.3.4/x":               true,
		"https://api.example.com/x":        false,
		"https://uat.example.com.evil.io/": false,
	} {
		if err := p.Check(OutboundRequest{URL: url}); (err == nil) != allowed {
			t.Errorf("Check(%s) = %v, want allowed=%v", url, err, allowed)
		}
	}
	if err := (Policy{}).Check(OutboundRequest{URL: "https://anything.io", Insecure: true}); err == nil || !strings.Contains(err.Error(), "CASERUNNER_ALLOW_INSECURE") {
		t.Errorf("insecure without permission: %v", err)
	}
	if err := (Policy{AllowInsecure: true}).Check(OutboundRequest{URL: "https://anything.io", Insecure: true}); err != nil {
		t.Errorf("insecure with permission: %v", err)
	}
	for ms, want := range map[int]time.Duration{0: 30 * time.Second, -5: 30 * time.Second, 1500: 1500 * time.Millisecond, 999999: time.Minute} {
		if got := p.Timeout(ms); got != want {
			t.Errorf("Timeout(%d) = %s, want %s", ms, got, want)
		}
	}
}

func TestFailedSendWithoutSendError(t *testing.T) {
	got := failedSend(errors.New("boom"), "https://a.local", time.Second)
	if got.ErrorType != ErrConnection || !strings.Contains(got.Reason, "https://a.local (boom)") {
		t.Errorf("got %+v", got)
	}
}
