// Package flow holds the rules of prepare steps (BRD-001 §11.3): {{name}}
// placeholders, their substitution, and capturing values such as a token
// from a step's response.
package flow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/field"
)

// Capture stores the value at Path of a step's JSON response in variable
// Var. An empty Path searches for a token automatically.
type Capture struct {
	Var  string
	Path string
}

// Captured is a value a capture found, and the path it was read from.
type Captured struct {
	Var   string
	Path  string
	Value string
}

var (
	varName     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)
)

// ValidVar reports whether name can be used as {{name}}.
func ValidVar(name string) bool { return varName.MatchString(name) }

// Placeholders lists the variable names used in the URL, header values and
// body of t, once each, in order of appearance.
func Placeholders(t curl.Template) []string {
	var names []string
	add := func(s string) {
		for _, m := range placeholder.FindAllStringSubmatch(s, -1) {
			if !slices.Contains(names, m[1]) {
				names = append(names, m[1])
			}
		}
	}
	add(t.URL)
	for _, h := range t.Headers {
		add(h.Value)
	}
	add(t.BodyRaw)
	return names
}

// Apply returns t with every {{name}} replaced by vars[name]. Inside a JSON
// body the value is JSON-escaped, so a value holding quotes keeps the body
// valid. Unknown names are an error that lists them.
func Apply(t curl.Template, vars map[string]string) (curl.Template, error) {
	var unknown []string
	for _, name := range Placeholders(t) {
		if _, ok := vars[name]; !ok {
			unknown = append(unknown, "{{"+name+"}}")
		}
	}
	if len(unknown) > 0 {
		return curl.Template{}, fmt.Errorf("unknown variable %s: no prepare step captures it", strings.Join(unknown, ", "))
	}
	raw := func(name string) string { return vars[name] }
	out := t
	out.URL = substitute(t.URL, raw)
	out.Headers = make([]curl.Header, len(t.Headers))
	for i, h := range t.Headers {
		out.Headers[i] = curl.Header{Name: h.Name, Value: substitute(h.Value, raw)}
	}
	if t.BodyKind == curl.BodyJSON {
		out.BodyRaw = substitute(t.BodyRaw, func(name string) string { return jsonEscape(vars[name]) })
	} else {
		out.BodyRaw = substitute(t.BodyRaw, raw)
	}
	return out, nil
}

func substitute(s string, value func(name string) string) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		return value(placeholder.FindStringSubmatch(m)[1])
	})
}

// jsonEscape returns s as it appears between the quotes of a JSON string.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s) // marshaling a string cannot fail
	return string(b[1 : len(b)-1])
}

// InjectBearer sets the Authorization header of t to "Bearer {{name}}",
// replacing any Authorization header the curl had.
func InjectBearer(t curl.Template, name string) curl.Template {
	out := t
	out.Headers = []curl.Header{}
	for _, h := range t.Headers {
		if !strings.EqualFold(h.Name, "Authorization") {
			out.Headers = append(out.Headers, h)
		}
	}
	out.Headers = append(out.Headers, curl.Header{Name: "Authorization", Value: "Bearer {{" + name + "}}"})
	return out
}

// tokenKeys are the keys searched for a token, most preferred first.
var tokenKeys = []string{
	"access_token", "accessToken", "token", "id_token", "idToken", "jwt",
	"auth_token", "authToken", "session_token", "sessionToken",
}

// Extract reads the value of c from a JSON response body.
func Extract(body []byte, c Capture) (Captured, error) {
	root, err := field.Decode(body)
	if err != nil {
		return Captured{}, fmt.Errorf("%s: the response is not JSON", c.Var)
	}
	if c.Path == "" {
		return findToken(root, c.Var)
	}
	segs, err := field.ParsePath(c.Path)
	if err != nil {
		return Captured{}, fmt.Errorf("%s: invalid path %q: %v", c.Var, c.Path, err)
	}
	n, ok := field.Lookup(root, segs)
	if !ok {
		return Captured{}, fmt.Errorf("%s: no value at %s", c.Var, c.Path)
	}
	switch n.Kind {
	case field.String, field.Number, field.Bool:
		if n.Text == "" {
			return Captured{}, fmt.Errorf("%s: the value at %s is empty", c.Var, c.Path)
		}
		return Captured{Var: c.Var, Path: c.Path, Value: n.Text}, nil
	default:
		return Captured{}, fmt.Errorf("%s: the value at %s is not a single value", c.Var, c.Path)
	}
}

func findToken(root *field.Node, name string) (Captured, error) {
	fields := field.Flatten(root)
	for _, key := range tokenKeys {
		for _, f := range fields {
			if f.Type != field.TypeString || f.Sample == field.TokenEmpty {
				continue
			}
			segs, err := field.ParsePath(f.Path)
			if err != nil {
				continue
			}
			if last := segs[len(segs)-1]; !last.IsIndex && strings.EqualFold(last.Key, key) {
				return Captured{Var: name, Path: f.Path, Value: f.Sample}, nil
			}
		}
	}
	return Captured{}, fmt.Errorf("%s: no token field found in the response (looked for %s); set the path", name, strings.Join(tokenKeys, ", "))
}
