package testcase

import (
	"reflect"
	"testing"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/field"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"
)

// The BRD-001 §7.2 body, compact.
const body = `{"requestId":"REQ001","data":{"amount":100000,"account":{"id":"0123456789","type":"SAVING"},"note":"test","urgent":false},"items":[{"sku":"A1","qty":2}]}`

func template() curl.Template {
	return curl.Template{
		Method: "POST", URL: "https://api.local/transfers",
		Headers: []curl.Header{{Name: "Content-Type", Value: "application/json"}},
		BodyRaw: body, BodyKind: curl.BodyJSON,
	}
}

// rules are the flattened fields of body, all Required.
func rules(t *testing.T) []FieldRule {
	t.Helper()
	root, err := field.Decode([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	var out []FieldRule
	for _, f := range field.Flatten(root) {
		out = append(out, FieldRule{Path: f.Path, Type: f.Type, Required: true})
	}
	return out
}

func samples(t *testing.T) map[string]string {
	t.Helper()
	root, _ := field.Decode([]byte(body))
	m := map[string]string{}
	for _, f := range field.Flatten(root) {
		m[f.Path] = f.Sample
	}
	return m
}

func with(m map[string]string, kv ...string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	for i := 0; i < len(kv); i += 2 {
		out[kv[i]] = kv[i+1]
	}
	return out
}

func unrequired(rs []FieldRule, paths ...string) []FieldRule {
	out := append([]FieldRule(nil), rs...)
	for i := range out {
		for _, p := range paths {
			if out[i].Path == p {
				out[i].Required = false
			}
		}
	}
	return out
}

// BRD-001 §19 critical case 9: a row of samples rebuilds the original body.
func TestSampleRowRebuildsTheBody(t *testing.T) {
	c := Case{Values: samples(t), Expect: "00"}
	if errs := Validate(c, rules(t)); errs != nil {
		t.Fatalf("Validate: %+v", errs)
	}
	got, err := Build(template(), rules(t), c)
	if err != nil {
		t.Fatal(err)
	}
	if got.BodyRaw != body {
		t.Errorf("body\n got  %s\n want %s", got.BodyRaw, body)
	}
	if got.URL != template().URL || !reflect.DeepEqual(got.Headers, template().Headers) {
		t.Error("method, URL and headers must not change")
	}
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name   string
		rules  []FieldRule
		values map[string]string
		want   string
	}{
		{
			"large numbers keep every digit",
			rules(t), with(samples(t), "data.amount", "12345678901234567890", "items[0].qty", "100000.50"),
			`{"requestId":"REQ001","data":{"amount":12345678901234567890,"account":{"id":"0123456789","type":"SAVING"},"note":"test","urgent":false},"items":[{"sku":"A1","qty":100000.50}]}`,
		},
		{
			"empty Unrequired removes the key; in an array the element stays",
			unrequired(rules(t), "data.note", "items[0].sku"), with(samples(t), "data.note", "", "items[0].sku", ""),
			`{"requestId":"REQ001","data":{"amount":100000,"account":{"id":"0123456789","type":"SAVING"},"urgent":false},"items":[{"qty":2}]}`,
		},
		{
			"<EMPTY> sends an empty string, <NULL> sends null",
			rules(t), with(samples(t), "data.note", "<EMPTY>", "data.amount", "<NULL>"),
			`{"requestId":"REQ001","data":{"amount":null,"account":{"id":"0123456789","type":"SAVING"},"note":"","urgent":false},"items":[{"sku":"A1","qty":2}]}`,
		},
		{
			"string values are not interpreted",
			rules(t), with(samples(t), "requestId", `{"x":1} "quoted"`, "data.urgent", "true"),
			`{"requestId":"{\"x\":1} \"quoted\"","data":{"amount":100000,"account":{"id":"0123456789","type":"SAVING"},"note":"test","urgent":true},"items":[{"sku":"A1","qty":2}]}`,
		},
		{
			"placeholders pass through to the use case",
			rules(t), with(samples(t), "requestId", "{{token}}"),
			`{"requestId":"{{token}}","data":{"amount":100000,"account":{"id":"0123456789","type":"SAVING"},"note":"test","urgent":false},"items":[{"sku":"A1","qty":2}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Case{Values: tt.values, Expect: "00"}
			if errs := Validate(c, tt.rules); errs != nil {
				t.Fatalf("Validate: %+v", errs)
			}
			got, err := Build(template(), tt.rules, c)
			if err != nil {
				t.Fatal(err)
			}
			if got.BodyRaw != tt.want {
				t.Errorf("body\n got  %s\n want %s", got.BodyRaw, tt.want)
			}
		})
	}
}

func TestBuildObjectAndArrayCells(t *testing.T) {
	tpl := template()
	tpl.BodyRaw = `{"meta":{},"tags":[],"x":null}`
	rs := []FieldRule{{Path: "meta", Type: field.TypeObject}, {Path: "tags", Type: field.TypeArray}, {Path: "x", Type: field.TypeNull}}
	c := Case{Values: map[string]string{"meta": `{"a":1}`, "tags": `["p",2]`, "x": "plain text"}, Expect: "00"}
	if errs := Validate(c, rs); errs != nil {
		t.Fatalf("Validate: %+v", errs)
	}
	got, err := Build(tpl, rs, c)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"meta":{"a":1},"tags":["p",2],"x":"plain text"}`; got.BodyRaw != want {
		t.Errorf("got %s, want %s", got.BodyRaw, want)
	}
}

func TestValidate(t *testing.T) {
	rs := unrequired(rules(t), "data.note")
	c := Case{
		Values: with(samples(t), "data.amount", "abc", "data.urgent", "yes", "requestId", "", "data.note", ""),
		Expect: "  ",
	}
	got := Validate(c, rs)
	want := []run.ValidationError{
		{Field: "requestId", Code: run.CodeRequiredMissing, Message: "Field requestId is Required. Enter a value or change it to Unrequired."},
		{Field: "data.amount", Code: run.CodeTypeMismatch, Message: "Field data.amount expects type number, got 'abc'"},
		{Field: "data.urgent", Code: run.CodeTypeMismatch, Message: "Field data.urgent expects type boolean, got 'yes'"},
		{Code: run.CodeInvalidExpectation, Message: "Case needs an expect code"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestValidateTypeMismatches(t *testing.T) {
	tests := []struct {
		typ  field.Type
		cell string
		ok   bool
	}{
		{field.TypeNumber, "-12.5e3", true},
		{field.TypeNumber, "1 2", false},
		{field.TypeNumber, "0x10", false},
		{field.TypeNumber, `"5"`, false},
		{field.TypeBoolean, "True", false},
		{field.TypeObject, "[1]", false},
		{field.TypeObject, "{bad", false},
		{field.TypeArray, "{}", false},
		{field.TypeString, "<NULL>", true},
		{field.TypeNumber, "<EMPTY>", true},
		{field.TypeObject, "<NULL>", true},
	}
	for _, tt := range tests {
		errs := Validate(Case{Values: map[string]string{"f": tt.cell}, Expect: "x"}, []FieldRule{{Path: "f", Type: tt.typ, Required: true}})
		if (errs == nil) != tt.ok {
			t.Errorf("%s %q: errors %+v, want ok=%v", tt.typ, tt.cell, errs, tt.ok)
		}
	}
}

func TestBuildRefusesAnInvalidCase(t *testing.T) {
	if _, err := Build(template(), rules(t), Case{Values: with(samples(t), "requestId", ""), Expect: "x"}); err == nil {
		t.Error("Build must refuse a Required field left empty")
	}
}
