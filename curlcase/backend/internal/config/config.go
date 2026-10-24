// Package config loads the CurlCase settings from environment variables (BRD-001 §20).
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds every setting the service reads at startup.
type Config struct {
	Bind               string
	AllowedHosts       []string // empty = every host is allowed (local use only, §13)
	AllowInsecure      bool
	DefaultConcurrency int
	MaxConcurrency     int
	DefaultTimeout     time.Duration
	MaxTimeout         time.Duration
	MaxCases           int
	RunTTL             time.Duration
	MaxRuns            int
}

// DefaultConcurrency is the concurrency a run gets when the tester does not set one (OD4).
const DefaultConcurrency = 5

// Load reads the configuration through getenv (os.Getenv in production) and
// returns every invalid variable at once.
func Load(getenv func(string) string) (Config, error) {
	r := reader{getenv: getenv}
	cfg := Config{
		Bind:               r.str("CURLCASE_BIND", "127.0.0.1:8080"),
		AllowedHosts:       r.list("CURLCASE_ALLOWED_HOSTS"),
		AllowInsecure:      r.boolean("CURLCASE_ALLOW_INSECURE", false),
		DefaultConcurrency: DefaultConcurrency,
		MaxConcurrency:     r.positiveInt("CURLCASE_MAX_CONCURRENCY", 20),
		DefaultTimeout:     r.millis("CURLCASE_DEFAULT_TIMEOUT_MS", 30_000),
		MaxTimeout:         r.millis("CURLCASE_MAX_TIMEOUT_MS", 120_000),
		MaxCases:           r.positiveInt("CURLCASE_MAX_CASES", 1000),
		RunTTL:             r.duration("CURLCASE_RUN_TTL", time.Hour),
		MaxRuns:            r.positiveInt("CURLCASE_MAX_RUNS", 50),
	}
	if r.err == nil {
		if cfg.DefaultTimeout > cfg.MaxTimeout {
			r.fail("CURLCASE_DEFAULT_TIMEOUT_MS", "must not exceed CURLCASE_MAX_TIMEOUT_MS")
		}
		if cfg.DefaultConcurrency > cfg.MaxConcurrency {
			cfg.DefaultConcurrency = cfg.MaxConcurrency
		}
	}
	if r.err != nil {
		return Config{}, r.err
	}
	return cfg, nil
}

type reader struct {
	getenv func(string) string
	err    error
}

func (r *reader) fail(name, msg string) {
	r.err = errors.Join(r.err, fmt.Errorf("%s: %s", name, msg))
}

func (r *reader) str(name, def string) string {
	if v := strings.TrimSpace(r.getenv(name)); v != "" {
		return v
	}
	return def
}

func (r *reader) list(name string) []string {
	var out []string
	for _, item := range strings.Split(r.getenv(name), ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func (r *reader) boolean(name string, def bool) bool {
	v := strings.TrimSpace(r.getenv(name))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.fail(name, fmt.Sprintf("want true or false, got %q", v))
		return def
	}
	return b
}

func (r *reader) positiveInt(name string, def int) int {
	v := strings.TrimSpace(r.getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		r.fail(name, fmt.Sprintf("want a positive integer, got %q", v))
		return def
	}
	return n
}

func (r *reader) millis(name string, def int) time.Duration {
	return time.Duration(r.positiveInt(name, def)) * time.Millisecond
}

func (r *reader) duration(name string, def time.Duration) time.Duration {
	v := strings.TrimSpace(r.getenv(name))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		r.fail(name, fmt.Sprintf("want a positive duration such as 30m or 1h, got %q", v))
		return def
	}
	return d
}
