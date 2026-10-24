package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"
)

// RequestSender sends one HTTP request to the API under test. It never
// retries and never follows redirects (BRD-001 §11.2).
type RequestSender interface {
	Send(ctx context.Context, req OutboundRequest) (InboundResponse, error)
}

// OutboundRequest is a fully substituted request ready to send.
type OutboundRequest struct {
	Method   string
	URL      string
	Headers  []curl.Header
	Body     string
	Insecure bool // skip TLS verification; allowed only by policy (§13)
}

// InboundResponse is what came back, with the body capped (§11.2).
type InboundResponse struct {
	Status    int
	Headers   []curl.Header
	Body      []byte
	Truncated bool
	Duration  time.Duration
}

// SendError is a request that got no usable response. Type is one of the
// errorType values of §12.
type SendError struct {
	Type string
	Err  error
}

func (e *SendError) Error() string { return e.Type + ": " + e.Err.Error() }
func (e *SendError) Unwrap() error { return e.Err }

// errorType values of BRD-001 §12.
const (
	ErrTimeout         = "TIMEOUT"
	ErrConnection      = "CONNECTION"
	ErrDNS             = "DNS"
	ErrTLS             = "TLS"
	ErrInvalidResponse = "INVALID_RESPONSE"
	ErrCancelled       = "CANCELLED"
	ErrPolicy          = "POLICY"
)

// RunRepository keeps runs while they execute and for a while after
// (BRD-001 §15.6). Get returns a copy; every change goes through Update,
// which applies fn under the repository's lock.
type RunRepository interface {
	Create(r run.Run) error
	Get(id string) (run.Run, bool)
	Update(id string, fn func(*run.Run)) error
}

// Errors a RunRepository returns.
var (
	ErrRunNotFound = errors.New("run not found or expired")
	ErrStoreFull   = errors.New("too many runs are in progress")
)
