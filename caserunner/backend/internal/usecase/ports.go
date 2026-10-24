package usecase

import (
	"context"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
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
