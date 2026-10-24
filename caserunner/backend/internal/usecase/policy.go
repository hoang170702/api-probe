package usecase

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"
)

// Policy holds the guardrails every outgoing request goes through (BRD-001 §13).
type Policy struct {
	AllowedHosts   []string // patterns such as *.uat.example.com; empty allows every host
	AllowInsecure  bool
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
}

// Check returns why req may not be sent, or nil.
func (p Policy) Check(req OutboundRequest) error {
	u, err := url.Parse(req.URL)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("invalid URL %q", req.URL)
	}
	if !p.hostAllowed(u.Hostname()) {
		return fmt.Errorf("host %s is not in CASERUNNER_ALLOWED_HOSTS", u.Hostname())
	}
	if req.Insecure && !p.AllowInsecure {
		return fmt.Errorf("-k/--insecure needs CASERUNNER_ALLOW_INSECURE=true; install the server's CA instead of skipping TLS verification")
	}
	return nil
}

func (p Policy) hostAllowed(host string) bool {
	if len(p.AllowedHosts) == 0 {
		return true
	}
	host = strings.ToLower(host)
	for _, pattern := range p.AllowedHosts {
		if ok, _ := path.Match(strings.ToLower(pattern), host); ok {
			return true
		}
	}
	return false
}

// Timeout returns the per-request timeout for a requested value in
// milliseconds: the default when it is not positive, clamped to the maximum.
func (p Policy) Timeout(ms int) time.Duration {
	if ms <= 0 {
		return p.DefaultTimeout
	}
	return min(time.Duration(ms)*time.Millisecond, p.MaxTimeout)
}
