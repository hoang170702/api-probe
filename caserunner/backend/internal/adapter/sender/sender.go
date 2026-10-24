// Package sender is the outbound HTTP adapter: it sends requests to the API
// under test with one shared connection pool (BRD-001 §11.2).
package sender

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// MaxBody is how much of a response body is read (§11.2).
const MaxBody = 1 << 20

// Sender implements usecase.RequestSender.
type Sender struct {
	verified *http.Client
	insecure *http.Client
}

// New returns a Sender. Callers bound each request with its context.
func New() *Sender {
	return &Sender{verified: client(false), insecure: client(true)}
}

func client(skipVerify bool) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if skipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // only when the policy allows -k (§13)
	}
	return &http.Client{
		Transport: tr,
		// Never follow redirects: the tester must see the 3xx.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Send sends req once. Headers the client must own are dropped: the length
// is computed, and compression is negotiated by the transport so a gzip
// body copied from DevTools is still read as text.
func (s *Sender) Send(ctx context.Context, req usecase.OutboundRequest) (usecase.InboundResponse, error) {
	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}
	r, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return usecase.InboundResponse{}, &usecase.SendError{Type: usecase.ErrConnection, Err: err}
	}
	for _, h := range req.Headers {
		switch strings.ToLower(h.Name) {
		case "content-length", "accept-encoding":
		case "host":
			r.Host = h.Value
		default:
			r.Header.Add(h.Name, h.Value)
		}
	}

	c := s.verified
	if req.Insecure {
		c = s.insecure
	}
	start := time.Now()
	resp, err := c.Do(r)
	if err != nil {
		return usecase.InboundResponse{}, &usecase.SendError{Type: classify(ctx, err), Err: err}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return usecase.InboundResponse{}, &usecase.SendError{Type: classify(ctx, err), Err: err}
	}
	out := usecase.InboundResponse{Status: resp.StatusCode, Headers: headers(resp.Header), Body: data, Duration: time.Since(start)}
	if len(data) > MaxBody {
		out.Body, out.Truncated = data[:MaxBody], true
	}
	return out, nil
}

// headers lists response headers sorted by name, one entry per value.
func headers(h http.Header) []curl.Header {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	slices.Sort(names)
	var out []curl.Header
	for _, name := range names {
		for _, v := range h[name] {
			out = append(out, curl.Header{Name: name, Value: v})
		}
	}
	return out
}

func classify(ctx context.Context, err error) string {
	var (
		dnsErr      *net.DNSError
		certErr     *tls.CertificateVerificationError
		unknownAuth x509.UnknownAuthorityError
		hostErr     x509.HostnameError
		recordErr   tls.RecordHeaderError
	)
	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded):
		return usecase.ErrTimeout
	case errors.Is(err, context.Canceled):
		return usecase.ErrCancelled
	case errors.As(err, &dnsErr):
		return usecase.ErrDNS
	case errors.As(err, &certErr), errors.As(err, &unknownAuth), errors.As(err, &hostErr), errors.As(err, &recordErr):
		return usecase.ErrTLS
	case errors.Is(err, io.ErrUnexpectedEOF):
		return usecase.ErrInvalidResponse
	default:
		return usecase.ErrConnection
	}
}
