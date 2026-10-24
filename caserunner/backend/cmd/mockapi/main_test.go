package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func call(t *testing.T, srv *httptest.Server, method, path, auth, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestEndpoints(t *testing.T) {
	srv := httptest.NewServer(newAPI().handler())
	defer srv.Close()
	tests := []struct {
		path, body string
		status     int
		contains   string
	}{
		{"/ok", "", 200, `"code":"00"`},
		{"/created", "", 201, `"code":"00"`},
		{"/error", "", 500, "ERR_INTERNAL"},
		{"/redirect", "", 302, ""},
		{"/validate", `{"data":{"amount":100}}`, 200, `"code":"00"`},
		{"/validate", `{"data":{"note":"x"}}`, 400, "ERR_AMOUNT_REQUIRED"},
		{"/validate", `not json`, 400, "ERR_BAD_JSON"},
		{"/slow?ms=20", "", 200, "slow"},
	}
	for _, tt := range tests {
		status, body := call(t, srv, "POST", tt.path, "", tt.body)
		if status != tt.status || !strings.Contains(body, tt.contains) {
			t.Errorf("POST %s = %d %s, want %d containing %q", tt.path, status, body, tt.status, tt.contains)
		}
	}
}

func TestSlowWaits(t *testing.T) {
	srv := httptest.NewServer(newAPI().handler())
	defer srv.Close()
	start := time.Now()
	call(t, srv, "POST", "/slow?ms=150", "", "")
	if d := time.Since(start); d < 150*time.Millisecond {
		t.Errorf("answered after %s, want at least 150ms", d)
	}
}

func TestLoginThenSecure(t *testing.T) {
	srv := httptest.NewServer(newAPI().handler())
	defer srv.Close()
	if status, _ := call(t, srv, "POST", "/secure", "Bearer forged", ""); status != 401 {
		t.Errorf("forged token: %d, want 401", status)
	}
	_, b1 := call(t, srv, "POST", "/login", "", "")
	_, b2 := call(t, srv, "POST", "/login", "", "")
	var r1, r2 struct {
		Data struct{ AccessToken string } `json:"data"`
	}
	_ = json.Unmarshal([]byte(b1), &r1)
	_ = json.Unmarshal([]byte(b2), &r2)
	if r1.Data.AccessToken == "" || r1.Data.AccessToken == r2.Data.AccessToken {
		t.Fatalf("every login must issue a new token: %q %q", r1.Data.AccessToken, r2.Data.AccessToken)
	}
	if status, _ := call(t, srv, "POST", "/secure", "Bearer "+r1.Data.AccessToken, ""); status != 200 {
		t.Errorf("issued token: %d, want 200", status)
	}
}

func TestHits(t *testing.T) {
	srv := httptest.NewServer(newAPI().handler())
	defer srv.Close()
	call(t, srv, "POST", "/ok", "", "")
	call(t, srv, "POST", "/ok", "", "")
	call(t, srv, "POST", "/error", "", "")
	_, body := call(t, srv, "GET", "/hits", "", "")
	var hits map[string]int
	_ = json.Unmarshal([]byte(body), &hits)
	if hits["/ok"] != 2 || hits["/error"] != 1 || len(hits) != 2 {
		t.Errorf("hits = %v", hits)
	}
	call(t, srv, "POST", "/hits/reset", "", "")
	_, body = call(t, srv, "GET", "/hits", "", "")
	if body != "{}" {
		t.Errorf("after reset: %s", body)
	}
}
