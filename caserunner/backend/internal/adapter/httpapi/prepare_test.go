package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/sender"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

func prepareHandler(allowed ...string) http.Handler {
	policy := usecase.Policy{AllowedHosts: allowed, DefaultTimeout: 5 * time.Second, MaxTimeout: 5 * time.Second}
	return NewHandler(Info{}, UseCases{TryPrepare: usecase.TryPrepare{Sender: sender.New(), Policy: policy}}, nil)
}

func postPrepare(t *testing.T, h http.Handler, body string) (*httptest.ResponseRecorder, prepareTryResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/prepare/try", strings.NewReader(body)))
	var res prepareTryResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("decode %s: %v", rec.Body, err)
		}
	}
	return rec, res
}

// A fake auth API: /login issues a token, /me requires it.
func authAPI(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"code":"00","data":{"accessToken":"tok-123","expiresIn":300}}`))
		case "/me":
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"user":{"id":"u-7"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPrepareTryCapturesAndChains(t *testing.T) {
	api := authAPI(t)
	body := `{"steps":[
	  {"template":{"method":"POST","url":"` + api.URL + `/login","headers":[{"name":"Content-Type","value":"application/json"}],"bodyRaw":"{\"u\":\"a\"}","bodyKind":"json"},
	   "captures":[{"var":"token","path":""}]},
	  {"template":{"method":"GET","url":"` + api.URL + `/me","headers":[{"name":"Authorization","value":"Bearer {{token}}"}],"bodyRaw":"","bodyKind":"none"},
	   "captures":[{"var":"userId","path":"user.id"}]}
	]}`
	rec, res := postPrepare(t, prepareHandler(), body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, %s", rec.Code, rec.Body)
	}
	if len(res.Steps) != 2 || res.Steps[0].Status != "OK" || res.Steps[1].Status != "OK" {
		t.Fatalf("steps = %+v", res.Steps)
	}
	if got := res.Steps[0].Captured[0]; got != (capturedDTO{Var: "token", Path: "data.accessToken", Value: "tok-123"}) {
		t.Errorf("token capture = %+v", got)
	}
	if res.Vars["token"] != "tok-123" || res.Vars["userId"] != "u-7" {
		t.Errorf("vars = %v", res.Vars)
	}
	if res.Steps[0].HTTPStatus != 200 || !strings.Contains(res.Steps[0].ResponseBody, `"accessToken"`) {
		t.Errorf("step 1 = %+v", res.Steps[0])
	}
}

func TestPrepareTryReportsAFailedStepIn200(t *testing.T) {
	api := authAPI(t)
	body := `{"steps":[{"template":{"method":"GET","url":"` + api.URL + `/me","headers":[],"bodyRaw":"","bodyKind":"none"},"captures":[]}]}`
	rec, res := postPrepare(t, prepareHandler(), body)
	if rec.Code != http.StatusOK || res.Steps[0].Status != "FAILED" || res.Steps[0].Reason != "Expected a 2xx response, got HTTP 401" {
		t.Fatalf("got %d %+v", rec.Code, res.Steps)
	}
	if res.Steps[0].Captured == nil {
		t.Error("captured must be [] not null")
	}
}

func TestPrepareTryRejectsBadInput(t *testing.T) {
	tpl := `{"method":"GET","url":"https://a.local","headers":[],"bodyRaw":"","bodyKind":"none"}`
	tests := []struct{ name, body, want string }{
		{"no steps", `{"steps":[]}`, "between 1 and 10"},
		{"too many steps", `{"steps":[` + strings.TrimSuffix(strings.Repeat(`{"template":`+tpl+`},`, 11), ",") + `]}`, "between 1 and 10"},
		{"missing URL", `{"steps":[{"template":{"method":"GET","url":""}}]}`, "needs a method and a URL"},
		{"bad variable", `{"steps":[{"template":` + tpl + `,"captures":[{"var":"2fa","path":""}]}]}`, "is not a valid variable name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, _ := postPrepare(t, prepareHandler(), tt.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.want) {
				t.Errorf("got %d %s, want 400 containing %q", rec.Code, rec.Body, tt.want)
			}
		})
	}
}
