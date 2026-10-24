package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestHealth(t *testing.T) {
	rec := serve(t, NewHandler(Info{Version: "test"}, UseCases{}, nil), http.MethodGet, "/api/health")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	var got healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := (healthResponse{Status: "ok", Service: "caserunner", Version: "test"}); got != want {
		t.Errorf("body = %+v, want %+v", got, want)
	}
}

func TestUnknownAPIPathReturnsJSONError(t *testing.T) {
	rec := serve(t, NewHandler(Info{}, UseCases{}, nil), http.MethodGet, "/api/nope")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var got errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", got.Error.Code)
	}
}

func TestPanicBecomesInternalError(t *testing.T) {
	h := recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := serve(t, h, http.MethodGet, "/api/anything")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var got errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != "INTERNAL" {
		t.Errorf("code = %q, want INTERNAL", got.Error.Code)
	}
}
