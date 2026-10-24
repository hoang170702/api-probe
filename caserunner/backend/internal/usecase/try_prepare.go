package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/flow"
)

// PrepareStep is one prepare curl and what to capture from its response (BRD-001 §11.3).
type PrepareStep struct {
	Template curl.Template
	Captures []flow.Capture
}

// Step outcomes.
const (
	StepOK      = "OK"
	StepFailed  = "FAILED"
	StepSkipped = "SKIPPED"
)

// StepResult is what one prepare step did.
type StepResult struct {
	Status       string
	HTTPStatus   int
	Duration     time.Duration
	Reason       string
	ErrorType    string // set when no response was received
	Captured     []flow.Captured
	ResponseBody string
	Truncated    bool
}

// TryPrepareOutput holds every step's result and the variables captured.
type TryPrepareOutput struct {
	Steps []StepResult
	Vars  map[string]string
}

// TryPrepare runs the prepare steps once so the tester can check what they capture (§15.7).
type TryPrepare struct {
	Sender RequestSender
	Policy Policy
}

func (u TryPrepare) Execute(ctx context.Context, steps []PrepareStep, timeoutMs int) TryPrepareOutput {
	results, vars := runPrepare(ctx, u.Sender, u.Policy, steps, u.Policy.Timeout(timeoutMs))
	return TryPrepareOutput{Steps: results, Vars: vars}
}

// runPrepare runs steps in order, substituting the variables captured so
// far; after the first failure the remaining steps are skipped.
func runPrepare(ctx context.Context, sender RequestSender, policy Policy, steps []PrepareStep, timeout time.Duration) ([]StepResult, map[string]string) {
	vars := map[string]string{}
	results := make([]StepResult, len(steps))
	failed := false
	for i, step := range steps {
		if failed {
			results[i] = StepResult{Status: StepSkipped, Reason: "An earlier step failed"}
			continue
		}
		results[i] = runStep(ctx, sender, policy, step, vars, timeout)
		if results[i].Status != StepOK {
			failed = true
			continue
		}
		for _, c := range results[i].Captured {
			vars[c.Var] = c.Value
		}
	}
	return results, vars
}

func runStep(ctx context.Context, sender RequestSender, policy Policy, step PrepareStep, vars map[string]string, timeout time.Duration) StepResult {
	t, err := flow.Apply(step.Template, vars)
	if err != nil {
		return StepResult{Status: StepFailed, Reason: capitalize(err.Error())}
	}
	req := OutboundRequest{Method: t.Method, URL: t.URL, Headers: t.Headers, Body: t.BodyRaw, Insecure: t.Insecure}
	if err := policy.Check(req); err != nil {
		return StepResult{Status: StepFailed, ErrorType: ErrPolicy, Reason: capitalize(err.Error())}
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := sender.Send(reqCtx, req)
	if err != nil {
		return failedSend(err, req.URL, timeout)
	}

	res := StepResult{
		HTTPStatus: resp.Status, Duration: resp.Duration,
		ResponseBody: string(resp.Body), Truncated: resp.Truncated,
	}
	if resp.Status < 200 || resp.Status > 299 {
		res.Status, res.Reason = StepFailed, fmt.Sprintf("Expected a 2xx response, got HTTP %d", resp.Status)
		return res
	}
	for _, c := range step.Captures {
		got, err := flow.Extract(resp.Body, c)
		if err != nil {
			res.Status, res.Reason = StepFailed, capitalize(err.Error())
			return res
		}
		res.Captured = append(res.Captured, got)
	}
	res.Status = StepOK
	return res
}

// failedSend turns a transport error into a tester-friendly result (§12).
func failedSend(err error, rawURL string, timeout time.Duration) StepResult {
	res := StepResult{Status: StepFailed, ErrorType: ErrConnection}
	var se *SendError
	if errors.As(err, &se) {
		res.ErrorType = se.Type
	}
	switch res.ErrorType {
	case ErrTimeout:
		res.Reason = fmt.Sprintf("Timed out after %s. Cannot determine whether the server processed the request", timeout)
	case ErrCancelled:
		res.Reason = "Cancelled before a response arrived"
	default:
		res.Reason = fmt.Sprintf("Could not get a response from %s (%v)", rawURL, unwrapAll(err))
	}
	return res
}

func unwrapAll(err error) error {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
