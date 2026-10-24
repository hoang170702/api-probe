package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func postParse(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/parse", strings.NewReader(body))
	NewHandler(Info{}, UseCases{}, nil).ServeHTTP(rec, req)
	return rec
}

func curlJSON(t *testing.T, cmd string) string {
	t.Helper()
	b, err := json.Marshal(parseRequest{Curl: cmd})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseReturnsTemplateAndFields(t *testing.T) {
	rec := postParse(t, curlJSON(t, `curl 'https://uat.local/v1/transfers' -H 'Content-Type: application/json' --compressed --data-raw '{"amount":12345678901234567890,"note":""}'`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	// Decode into a generic value to check the exact JSON names of §15.2.
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"template": map[string]any{
			"method":   "POST",
			"url":      "https://uat.local/v1/transfers",
			"headers":  []any{map[string]any{"name": "Content-Type", "value": "application/json"}},
			"bodyRaw":  `{"amount":12345678901234567890,"note":""}`,
			"bodyKind": "json",
			"insecure": false,
		},
		"fields": []any{
			map[string]any{"path": "amount", "type": "number", "sampleValue": "12345678901234567890"},
			map[string]any{"path": "note", "type": "string", "sampleValue": "<EMPTY>"},
		},
		"warnings": []any{"Ignored --compressed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestParseWithoutBodyReturnsEmptyLists(t *testing.T) {
	rec := postParse(t, curlJSON(t, "curl https://a.local"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"headers":[]`) || !strings.Contains(body, `"fields":[]`) || !strings.Contains(body, `"warnings":[]`) {
		t.Errorf("lists must be [] not null: %s", body)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"curl syntax error", curlJSON(t, "curl -d 'x"), 400, "CURL_PARSE_ERROR", "Missing closing ' for the quote opened at position 9"},
		{"unsupported curl", curlJSON(t, "curl https://a -d @f.json"), 400, "CURL_UNSUPPORTED", "reads the body from a file"},
		{"not JSON", "curl https://a", 400, "BAD_REQUEST", "must be valid JSON"},
		{"too large", curlJSON(t, "curl https://a -d '"+strings.Repeat("x", maxParseRequest)+"'"), 413, "REQUEST_TOO_LARGE", "too large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := postParse(t, tt.body)
			var got errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode %q: %v", rec.Body, err)
			}
			if rec.Code != tt.wantStatus || got.Error.Code != tt.wantCode || !strings.Contains(got.Error.Message, tt.wantMsg) {
				t.Errorf("got %d %+v, want %d %s containing %q", rec.Code, got.Error, tt.wantStatus, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestParseRejectsGET(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(Info{}, UseCases{}, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/parse", nil))
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" || !strings.Contains(rec.Body.String(), "METHOD_NOT_ALLOWED") {
		t.Errorf("got %d Allow=%q %s, want 405 Allow=POST", rec.Code, rec.Header().Get("Allow"), rec.Body)
	}
}
