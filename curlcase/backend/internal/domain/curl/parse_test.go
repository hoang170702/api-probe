package curl

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		want         Template
		wantWarnings []string
	}{
		{
			name: "Chrome Copy as cURL (bash)",
			input: `curl 'https://uat-api.example.com/v1/transfers?channel=MB' \
  -H 'accept: application/json' \
  -H 'authorization: Bearer eyJabc' \
  -H 'content-type: application/json' \
  --data-raw '{"data":{"amount":100000,"note":"test"}}'`,
			want: Template{
				Method: "POST",
				URL:    "https://uat-api.example.com/v1/transfers?channel=MB",
				Headers: []Header{
					{"accept", "application/json"},
					{"authorization", "Bearer eyJabc"},
					{"content-type", "application/json"},
				},
				BodyRaw:  `{"data":{"amount":100000,"note":"test"}}`,
				BodyKind: BodyJSON,
			},
		},
		{
			name: "Chrome Copy as cURL (cmd)",
			input: "curl ^\"https://uat-api.example.com/v1/transfers^\" ^\r\n" +
				"  -H ^\"content-type: application/json^\" ^\r\n" +
				"  --data-raw ^\"^{^\\^\"amount^\\^\":100000^}^\"",
			want: Template{
				Method:   "POST",
				URL:      "https://uat-api.example.com/v1/transfers",
				Headers:  []Header{{"content-type", "application/json"}},
				BodyRaw:  `{"amount":100000}`,
				BodyKind: BodyJSON,
			},
		},
		{
			name: "Postman export",
			input: `curl --location --request PUT 'https://api.local/items/1' \
--header 'Content-Type: application/json' \
--data '{"qty": 2}'`,
			want: Template{
				Method:   "PUT",
				URL:      "https://api.local/items/1",
				Headers:  []Header{{"Content-Type", "application/json"}},
				BodyRaw:  `{"qty": 2}`,
				BodyKind: BodyJSON,
			},
			wantWarnings: []string{"Ignored --location: redirects are never followed, a 3xx is reported as the result"},
		},
		{
			name:  "no -X and no body is GET",
			input: "curl https://api.local/ping",
			want:  Template{Method: "GET", URL: "https://api.local/ping", BodyKind: BodyNone},
		},
		{
			name:  "-X wins over the body rule, attached form -XDELETE",
			input: `curl -XDELETE https://api.local/x -H 'Content-Type: application/json' -d '{}'`,
			want: Template{
				Method: "DELETE", URL: "https://api.local/x",
				Headers: []Header{{"Content-Type", "application/json"}},
				BodyRaw: "{}", BodyKind: BodyJSON,
			},
		},
		{
			name:  "-I without -X is HEAD",
			input: "curl -I https://api.local/x",
			want:  Template{Method: "HEAD", URL: "https://api.local/x", BodyKind: BodyNone},
		},
		{
			name:  "duplicate headers keep their order",
			input: `curl https://a.local -H 'X-Tag: 1' -H 'X-Tag: 2' -H "Empty;"`,
			want: Template{
				Method: "GET", URL: "https://a.local", BodyKind: BodyNone,
				Headers: []Header{{"X-Tag", "1"}, {"X-Tag", "2"}, {"Empty", ""}},
			},
		},
		{
			name:         "several -d are joined with &",
			input:        `curl https://a.local -d a=1 --data b=2 --data-ascii c=3`,
			want:         Template{Method: "POST", URL: "https://a.local", BodyRaw: "a=1&b=2&c=3", BodyKind: BodyUnsupported},
			wantWarnings: []string{"3 data options were joined with & into one body, as curl does"},
		},
		{
			name:  "--data-raw keeps a leading @",
			input: `curl https://a.local -H 'Content-Type: text/plain' --data-raw '@me'`,
			want: Template{
				Method: "POST", URL: "https://a.local", BodyRaw: "@me", BodyKind: BodyUnsupported,
				Headers: []Header{{"Content-Type", "text/plain"}},
			},
		},
		{
			name:  "--json adds Content-Type and Accept",
			input: `curl https://a.local --json '{"a":1}'`,
			want: Template{
				Method: "POST", URL: "https://a.local", BodyRaw: `{"a":1}`, BodyKind: BodyJSON,
				Headers: []Header{{"Content-Type", "application/json"}, {"Accept", "application/json"}},
			},
		},
		{
			name:  "--json keeps a Content-Type the tester set",
			input: `curl https://a.local -H 'content-type: application/vnd.x+json' --json '[]'`,
			want: Template{
				Method: "POST", URL: "https://a.local", BodyRaw: `[]`, BodyKind: BodyJSON,
				Headers: []Header{{"content-type", "application/vnd.x+json"}, {"Accept", "application/json"}},
			},
		},
		{
			name:  "-u becomes Basic auth",
			input: `curl -u alice:s3cret https://a.local`,
			want: Template{
				Method: "GET", URL: "https://a.local", BodyKind: BodyNone,
				Headers: []Header{{"Authorization", "Basic YWxpY2U6czNjcmV0"}},
			},
		},
		{
			name:  "-u without a password",
			input: `curl --user alice https://a.local`,
			want: Template{
				Method: "GET", URL: "https://a.local", BodyKind: BodyNone,
				Headers: []Header{{"Authorization", "Basic YWxpY2U6"}},
			},
			wantWarnings: []string{`--user has no password; sending "alice" with an empty password`},
		},
		{
			name:  "-b, -A, -e and --oauth2-bearer become headers",
			input: `curl https://a.local -b 'sid=1; theme=dark' -A 'agent/1' -e 'https://ref.local;auto' --oauth2-bearer tok`,
			want: Template{
				Method: "GET", URL: "https://a.local", BodyKind: BodyNone,
				Headers: []Header{
					{"Cookie", "sid=1; theme=dark"}, {"User-Agent", "agent/1"},
					{"Referer", "https://ref.local"}, {"Authorization", "Bearer tok"},
				},
			},
		},
		{
			name:  "combined short flags -sSLk",
			input: `curl -sSLk https://a.local`,
			want:  Template{Method: "GET", URL: "https://a.local", BodyKind: BodyNone, Insecure: true},
			wantWarnings: []string{
				"Ignored -s", "Ignored -S",
				"Ignored -L: redirects are never followed, a 3xx is reported as the result",
			},
		},
		{
			name:  "ignored and unknown flags only warn",
			input: `curl --compressed -v --max-time 5 --frobnicate https://a.local -i`,
			want:  Template{Method: "GET", URL: "https://a.local", BodyKind: BodyNone},
			wantWarnings: []string{
				"Ignored --compressed", "Ignored -v", "Ignored --max-time: the run's timeout applies",
				"Ignored unknown flag --frobnicate", "Ignored -i",
			},
		},
		{
			name:  "URL from --url",
			input: `curl --url https://a.local/x -X GET`,
			want:  Template{Method: "GET", URL: "https://a.local/x", BodyKind: BodyNone},
		},
		{
			name:         "URL without a scheme gets http",
			input:        `curl localhost:8080/ok`,
			want:         Template{Method: "GET", URL: "http://localhost:8080/ok", BodyKind: BodyNone},
			wantWarnings: []string{"The URL has no scheme; using http://localhost:8080/ok as curl does"},
		},
		{
			name:  "JSON body without Content-Type warns",
			input: `curl https://a.local -d '{"a":1}'`,
			want:  Template{Method: "POST", URL: "https://a.local", BodyRaw: `{"a":1}`, BodyKind: BodyJSON},
			wantWarnings: []string{
				"No Content-Type header: the API may not read the body as JSON (curl itself would send application/x-www-form-urlencoded)",
			},
		},
		{
			name:         "-F is a body v1 cannot run",
			input:        `curl https://a.local -F name=x`,
			want:         Template{Method: "POST", URL: "https://a.local", BodyRaw: "name=x", BodyKind: BodyUnsupported},
			wantWarnings: []string{"-F builds a form body; v1 supports JSON bodies only"},
		},
		{
			name:  "curl.exe and a large number keep their text",
			input: `curl.exe https://a.local -H 'Content-Type: application/json' -d '{"acc":12345678901234567890}'`,
			want: Template{
				Method: "POST", URL: "https://a.local", BodyRaw: `{"acc":12345678901234567890}`, BodyKind: BodyJSON,
				Headers: []Header{{"Content-Type", "application/json"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(got.Template, tt.want) {
				t.Errorf("template\n got  %+v\n want %+v", got.Template, tt.want)
			}
			if !reflect.DeepEqual(got.Warnings, tt.wantWarnings) {
				t.Errorf("warnings\n got  %q\n want %q", got.Warnings, tt.wantWarnings)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode string
		wantPos  int
		wantMsg  string
	}{
		{"empty", "  \n ", CodeParseError, 0, "The text is empty"},
		{"not curl", "wget https://a", CodeParseError, 1, `must start with curl, found "wget"`},
		{"unterminated quote", "curl https://a -d '{", CodeParseError, 19, "Missing closing '"},
		{"no URL", "curl -X POST", CodeParseError, 0, "No URL found"},
		{"two URLs", "curl https://a https://b", CodeParseError, 16, "second URL"},
		{"flag without value", "curl https://a -H", CodeParseError, 16, "-H needs a value"},
		{"body from file", "curl https://a -d @body.json", CodeUnsupported, 16, "reads the body from a file"},
		{"--json from file", "curl https://a --json @-", CodeUnsupported, 16, "reads the body from a file"},
		{"headers from file", "curl https://a -H @h.txt", CodeUnsupported, 16, "reads headers from a file"},
		{"-G", "curl -G https://a -d q=1", CodeUnsupported, 6, "-G is not supported"},
		{"upload", "curl -T f.bin https://a", CodeUnsupported, 6, "-T is not supported"},
		{"non-http scheme", "curl ftp://a/x", CodeUnsupported, 6, "Only http and https"},
		{"no host", "curl https:///x", CodeParseError, 6, "has no host"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want *ParseError", err)
			}
			if pe.Code != tt.wantCode || pe.Pos != tt.wantPos || !strings.Contains(pe.Message, tt.wantMsg) {
				t.Errorf("got %s pos %d %q\nwant %s pos %d containing %q", pe.Code, pe.Pos, pe.Message, tt.wantCode, tt.wantPos, tt.wantMsg)
			}
		})
	}
}

func TestParseErrorMessageIncludesPosition(t *testing.T) {
	_, err := Parse("curl -d 'x")
	if err == nil || err.Error() != "Missing closing ' for the quote opened at position 9" {
		t.Errorf("Error() = %v", err)
	}
}
