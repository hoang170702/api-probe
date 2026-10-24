package flow

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
)

func tpl() curl.Template {
	return curl.Template{
		Method: "POST",
		URL:    "https://api.local/v1/{{tenant}}/orders?sid={{ session }}",
		Headers: []curl.Header{
			{Name: "Authorization", Value: "Bearer {{token}}"},
			{Name: "X-Trace", Value: "fixed"},
		},
		BodyRaw:  `{"note":"{{note}}","token":"{{token}}"}`,
		BodyKind: curl.BodyJSON,
	}
}

func TestPlaceholdersInOrderOnce(t *testing.T) {
	if got, want := Placeholders(tpl()), []string{"tenant", "session", "token", "note"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplySubstitutesEverywhere(t *testing.T) {
	got, err := Apply(tpl(), map[string]string{"tenant": "t1", "session": "s9", "token": "eyJ.abc", "note": `say "hi"`})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://api.local/v1/t1/orders?sid=s9" {
		t.Errorf("URL = %q", got.URL)
	}
	if got.Headers[0].Value != "Bearer eyJ.abc" || got.Headers[1].Value != "fixed" {
		t.Errorf("headers = %+v", got.Headers)
	}
	// The value is JSON-escaped inside a JSON body, so the body stays valid.
	if want := `{"note":"say \"hi\"","token":"eyJ.abc"}`; got.BodyRaw != want {
		t.Errorf("body = %s, want %s", got.BodyRaw, want)
	}
}

func TestApplyLeavesNonJSONBodyRaw(t *testing.T) {
	in := curl.Template{BodyRaw: "grant_type=refresh&refresh_token={{token}}", BodyKind: curl.BodyUnsupported}
	got, err := Apply(in, map[string]string{"token": `a"b`})
	if err != nil {
		t.Fatal(err)
	}
	if got.BodyRaw != `grant_type=refresh&refresh_token=a"b` {
		t.Errorf("body = %s", got.BodyRaw)
	}
}

func TestApplyDoesNotTouchTheInput(t *testing.T) {
	in := tpl()
	if _, err := Apply(in, map[string]string{"tenant": "a", "session": "b", "token": "c", "note": "d"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, tpl()) {
		t.Errorf("input was modified: %+v", in)
	}
}

func TestApplyRejectsUnknownVariables(t *testing.T) {
	_, err := Apply(tpl(), map[string]string{"token": "x"})
	if err == nil || !strings.Contains(err.Error(), "{{tenant}}, {{session}}, {{note}}") {
		t.Errorf("err = %v", err)
	}
}

func TestInjectBearerReplacesAuthorization(t *testing.T) {
	in := curl.Template{Headers: []curl.Header{{Name: "authorization", Value: "Bearer old"}, {Name: "Accept", Value: "*/*"}}}
	got := InjectBearer(in, "token")
	want := []curl.Header{{Name: "Accept", Value: "*/*"}, {Name: "Authorization", Value: "Bearer {{token}}"}}
	if !reflect.DeepEqual(got.Headers, want) {
		t.Errorf("got %+v, want %+v", got.Headers, want)
	}
	if got := InjectBearer(curl.Template{}, "token"); len(got.Headers) != 1 {
		t.Errorf("a request without Authorization must get one: %+v", got.Headers)
	}
}

func TestExtractByPath(t *testing.T) {
	body := []byte(`{"data":{"accessToken":"eyJ.path","expiresIn":3600,"list":[{"id":"x"}]}}`)
	tests := []struct {
		path, want string
	}{
		{"data.accessToken", "eyJ.path"},
		{"data.expiresIn", "3600"},
		{"data.list[0].id", "x"},
	}
	for _, tt := range tests {
		got, err := Extract(body, Capture{Var: "v", Path: tt.path})
		if err != nil || got != (Captured{Var: "v", Path: tt.path, Value: tt.want}) {
			t.Errorf("Extract(%s) = %+v, %v", tt.path, got, err)
		}
	}
}

func TestExtractFindsATokenAutomatically(t *testing.T) {
	tests := []struct {
		name, body, wantPath, wantValue string
	}{
		{"OAuth", `{"access_token":"at","token_type":"Bearer","refresh_token":"rt"}`, "access_token", "at"},
		{"nested camelCase", `{"code":"00","data":{"user":{"id":1},"accessToken":"eyJ.n"}}`, "data.accessToken", "eyJ.n"},
		{"preference beats position", `{"token":"plain","result":{"access_token":"preferred"}}`, "result.access_token", "preferred"},
		{"case-insensitive key", `{"Data":{"TOKEN":"T"}}`, "Data.TOKEN", "T"},
		{"empty strings are skipped", `{"access_token":"","jwt":"j"}`, "jwt", "j"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Extract([]byte(tt.body), Capture{Var: "token"})
			if err != nil || got != (Captured{Var: "token", Path: tt.wantPath, Value: tt.wantValue}) {
				t.Errorf("got %+v, %v", got, err)
			}
		})
	}
}

func TestExtractErrors(t *testing.T) {
	tests := []struct {
		body string
		c    Capture
		want string
	}{
		{`<html>`, Capture{Var: "token"}, "not JSON"},
		{`{"user":"a"}`, Capture{Var: "token"}, "no token field found"},
		{`{"token":1}`, Capture{Var: "token"}, "no token field found"}, // a number is not a token
		{`{"a":1}`, Capture{Var: "x", Path: "data.id"}, "x: no value at data.id"},
		{`{"a":{"b":1}}`, Capture{Var: "x", Path: "a"}, "not a single value"},
		{`{"a":""}`, Capture{Var: "x", Path: "a"}, "is empty"},
		{`{"a":1}`, Capture{Var: "x", Path: "a["}, "invalid path"},
	}
	for _, tt := range tests {
		_, err := Extract([]byte(tt.body), tt.c)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Extract(%s, %+v) err = %v, want %q", tt.body, tt.c, err, tt.want)
		}
	}
}

func TestValidVar(t *testing.T) {
	for _, ok := range []string{"token", "_t", "accessToken2"} {
		if !ValidVar(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "2fa", "a-b", "a b", "{{x}}"} {
		if ValidVar(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
