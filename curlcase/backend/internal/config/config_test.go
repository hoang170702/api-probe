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
		"CURLCASE_BIND":               "0.0.0.0:9090",
		"CURLCASE_ALLOWED_HOSTS":      " localhost, *.uat.example.com ,,10.10.*",
		"CURLCASE_ALLOW_INSECURE":     "true",
		"CURLCASE_MAX_CONCURRENCY":    "3",
		"CURLCASE_DEFAULT_TIMEOUT_MS": "1000",
		"CURLCASE_MAX_TIMEOUT_MS":     "5000",
		"CURLCASE_MAX_CASES":          "10",
		"CURLCASE_RUN_TTL":            "15m",
		"CURLCASE_MAX_RUNS":           "2",
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
		"CURLCASE_ALLOW_INSECURE":  "yes please",
		"CURLCASE_MAX_CONCURRENCY": "0",
		"CURLCASE_MAX_CASES":       "abc",
		"CURLCASE_RUN_TTL":         "-1h",
	}))
	if err == nil {
		t.Fatal("Load: want an error")
	}
	for _, name := range []string{"CURLCASE_ALLOW_INSECURE", "CURLCASE_MAX_CONCURRENCY", "CURLCASE_MAX_CASES", "CURLCASE_RUN_TTL"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
}

func TestLoadRejectsDefaultTimeoutAboveMax(t *testing.T) {
	_, err := Load(env(map[string]string{
		"CURLCASE_DEFAULT_TIMEOUT_MS": "60000",
		"CURLCASE_MAX_TIMEOUT_MS":     "30000",
	}))
	if err == nil || !strings.Contains(err.Error(), "CURLCASE_DEFAULT_TIMEOUT_MS") {
		t.Fatalf("Load: got %v, want an error about CURLCASE_DEFAULT_TIMEOUT_MS", err)
	}
}

func equal(a, b Config) bool {
	return reflect.DeepEqual(a, b)
}
