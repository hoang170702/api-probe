package curl

import (
	"errors"
	"slices"
	"testing"
)

func values(toks []token) []string {
	out := make([]string, len(toks))
	for i, t := range toks {
		out[i] = t.value
	}
	return out
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"plain words", "curl  -X\tPOST https://a", []string{"curl", "-X", "POST", "https://a"}},
		{"single quotes keep everything", `curl 'a "b" \n $X'`, []string{"curl", `a "b" \n $X`}},
		{"double quotes escape only \" \\ $ and backtick", `curl "a \"b\" \\ \$X \n"`, []string{"curl", `a "b" \ $X \n`}},
		{"quote inside the other quote", `curl "it's" '"x"'`, []string{"curl", "it's", `"x"`}},
		{"empty quoted word", `curl '' ""`, []string{"curl", "", ""}},
		{"adjacent quotes join into one word", `curl a'b'"c"d`, []string{"curl", "abcd"}},
		{"backslash escapes outside quotes", `curl a\ b`, []string{"curl", "a b"}},
		{"bash line continuation", "curl \\\n  -X POST \\\n  https://a", []string{"curl", "-X", "POST", "https://a"}},
		{"bash line continuation with CRLF", "curl \\\r\n  https://a\r\n", []string{"curl", "https://a"}},
		{"ANSI-C quoting from Chrome", `curl $'{"note":"it\'s\nok","t":"\x41é"}'`, []string{"curl", "{\"note\":\"it's\nok\",\"t\":\"Aé\"}"}},
		{"ANSI-C octal and unknown escape", `curl $'\101\q'`, []string{"curl", `A\q`}},
		{
			"cmd line continuation and ^ escapes",
			"curl ^\"https://a/x?q=1^&r=a%^20b^\" ^\r\n  -H ^\"accept: */*^\"",
			[]string{"curl", "https://a/x?q=1&r=a%20b", "-H", "accept: */*"},
		},
		{
			"cmd JSON body from Chrome",
			"curl ^\"https://a^\" ^\n  --data-raw ^\"^{^\\^\"amount^\\^\":100000,^\\^\"note^\\^\":^\\^\"a b^\\^\"^}^\"",
			[]string{"curl", "https://a", "--data-raw", `{"amount":100000,"note":"a b"}`},
		},
		{"cmd plain double quotes with \\\"", "curl \"https://a\" ^\n -d \"{\\\"a\\\":1}\"", []string{"curl", "https://a", "-d", `{"a":1}`}},
		{"cmd backslashes before a quote", "curl ^\n \"a\\\\\\\"b\" \"c\\\\\" d\\e", []string{"curl", `a\"b`, `c\`, `d\e`}},
		{"cmd doubled quote inside quotes", "curl ^\n \"say \"\"hi\"\"\"", []string{"curl", `say "hi"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, err := tokenize(tt.input)
			if err != nil {
				t.Fatalf("tokenize: %v", err)
			}
			if got := values(toks); !slices.Equal(got, tt.want) {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestTokenizePositions(t *testing.T) {
	toks, err := tokenize("curl  -H 'X: 1'")
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for _, tok := range toks {
		got = append(got, tok.pos)
	}
	if want := []int{1, 7, 10}; !slices.Equal(got, want) {
		t.Errorf("positions = %v, want %v", got, want)
	}
}

func TestTokenizeUnterminatedQuote(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantPos int
	}{
		{"single quote", "curl -d '{\"a\":1}", 9},
		{"double quote", `curl -d "abc`, 9},
		{"ANSI-C quote", `curl $'abc`, 6},
		{"cmd quote", "curl ^\n \"abc", 9},
		{"multi-byte text before the quote counts characters", "curl -H 'Tên: Hoà' -d 'x", 23},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tokenize(tt.input)
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want *ParseError", err)
			}
			if pe.Code != CodeParseError || pe.Pos != tt.wantPos {
				t.Errorf("got code %s pos %d, want %s pos %d (%v)", pe.Code, pe.Pos, CodeParseError, tt.wantPos, pe)
			}
		})
	}
}

func TestLooksLikeCmd(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"curl 'https://a' \\\n -H 'x: ^\"'", false},
		{"curl ^\"https://a^\" ^\n -H ^\"x: y^\"", true},
		{"curl ^\"https://a^\"", true},
		{"curl 'https://a'", false},
		{`curl 'https://a' -d '{"note":"x^"}'`, false}, // ^" inside a bash string
	}
	for _, tt := range tests {
		if got := looksLikeCmd(tt.input); got != tt.want {
			t.Errorf("looksLikeCmd(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
