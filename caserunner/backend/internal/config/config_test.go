package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		Bind:               "127.0.0.1:8080",
		DefaultConcurrency: 5,
		MaxConcurrency:     20,
		DefaultTimeout:     30 * time.Second,
		MaxTimeout:         120 * time.Second,
		MaxCases:           1000,
		RunTTL:             time.Hour,
		MaxRuns:            50,
	}
	if !equal(cfg, want) {
		t.Errorf("got %+v\nwant %+v", cfg, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"CASERUNNER_BIND":               "0.0.0.0:9090",
		"CASERUNNER_ALLOWED_HOSTS":      " localhost, *.uat.example.com ,,10.10.*",
		"CASERUNNER_ALLOW_INSECURE":     "true",
		"CASERUNNER_MAX_CONCURRENCY":    "3",
		"CASERUNNER_DEFAULT_TIMEOUT_MS": "1000",
		"CASERUNNER_MAX_TIMEOUT_MS":     "5000",
		"CASERUNNER_MAX_CASES":          "10",
		"CASERUNNER_RUN_TTL":            "15m",
		"CASERUNNER_MAX_RUNS":           "2",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		Bind:               "0.0.0.0:9090",
		AllowedHosts:       []string{"localhost", "*.uat.example.com", "10.10.*"},
		AllowInsecure:      true,
		DefaultConcurrency: 3, // clamped to MaxConcurrency
		MaxConcurrency:     3,
		DefaultTimeout:     time.Second,
		MaxTimeout:         5 * time.Second,
		MaxCases:           10,
		RunTTL:             15 * time.Minute,
		MaxRuns:            2,
	}
	if !equal(cfg, want) {
		t.Errorf("got %+v\nwant %+v", cfg, want)
	}
}

func TestLoadReportsEveryInvalidVariable(t *testing.T) {
	_, err := Load(env(map[string]string{
		"CASERUNNER_ALLOW_INSECURE":  "yes please",
		"CASERUNNER_MAX_CONCURRENCY": "0",
		"CASERUNNER_MAX_CASES":       "abc",
		"CASERUNNER_RUN_TTL":         "-1h",
	}))
	if err == nil {
		t.Fatal("Load: want an error")
	}
	for _, name := range []string{"CASERUNNER_ALLOW_INSECURE", "CASERUNNER_MAX_CONCURRENCY", "CASERUNNER_MAX_CASES", "CASERUNNER_RUN_TTL"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
}

func TestLoadRejectsDefaultTimeoutAboveMax(t *testing.T) {
	_, err := Load(env(map[string]string{
		"CASERUNNER_DEFAULT_TIMEOUT_MS": "60000",
		"CASERUNNER_MAX_TIMEOUT_MS":     "30000",
	}))
	if err == nil || !strings.Contains(err.Error(), "CASERUNNER_DEFAULT_TIMEOUT_MS") {
		t.Fatalf("Load: got %v, want an error about CASERUNNER_DEFAULT_TIMEOUT_MS", err)
	}
}

func equal(a, b Config) bool {
	return reflect.DeepEqual(a, b)
}
