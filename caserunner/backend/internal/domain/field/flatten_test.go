package field

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func flatten(t *testing.T, body string) []Field {
	t.Helper()
	root, err := Decode([]byte(body))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return Flatten(root)
}

// The example of BRD-001 §7.2.
func TestFlattenBRDExample(t *testing.T) {
	got := flatten(t, `{
	  "requestId": "REQ001",
	  "data": {
	    "amount": 100000,
	    "account": {"id": "0123456789", "type": "SAVING"},
	    "note": "test",
	    "urgent": false
	  },
	  "items": [{"sku": "A1", "qty": 2}]
	}`)
	want := []Field{
		{"requestId", TypeString, "REQ001"},
		{"data.amount", TypeNumber, "100000"},
		{"data.account.id", TypeString, "0123456789"},
		{"data.account.type", TypeString, "SAVING"},
		{"data.note", TypeString, "test"},
		{"data.urgent", TypeBoolean, "false"},
		{"items[0].sku", TypeString, "A1"},
		{"items[0].qty", TypeNumber, "2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []Field
	}{
		{
			"key order is the body's, not alphabetical",
			`{"z":1,"a":2,"m":{"y":true,"b":null}}`,
			[]Field{{"z", TypeNumber, "1"}, {"a", TypeNumber, "2"}, {"m.y", TypeBoolean, "true"}, {"m.b", TypeNull, TokenNull}},
		},
		{
			"large numbers and decimals keep their text",
			`{"acc":12345678901234567890,"amt":100000.50,"e":1e-7,"neg":-0}`,
			[]Field{{"acc", TypeNumber, "12345678901234567890"}, {"amt", TypeNumber, "100000.50"}, {"e", TypeNumber, "1e-7"}, {"neg", TypeNumber, "-0"}},
		},
		{
			"arrays of objects and nested arrays",
			`{"items":[{"sku":"A1"},{"sku":"B2","tags":["x",["y"]]}]}`,
			[]Field{
				{"items[0].sku", TypeString, "A1"}, {"items[1].sku", TypeString, "B2"},
				{"items[1].tags[0]", TypeString, "x"}, {"items[1].tags[1][0]", TypeString, "y"},
			},
		},
		{
			"empty object and array are fields, empty string is <EMPTY>",
			`{"meta":{},"list":[],"note":""}`,
			[]Field{{"meta", TypeObject, "{}"}, {"list", TypeArray, "[]"}, {"note", TypeString, TokenEmpty}},
		},
		{
			"ambiguous keys are quoted",
			`{"a.b":1,"x":{"[0]":2,"":3,"say \"hi\"":4,"tên":5}}`,
			[]Field{
				{`["a.b"]`, TypeNumber, "1"}, {`x["[0]"]`, TypeNumber, "2"}, {`x[""]`, TypeNumber, "3"},
				{`x["say \"hi\""]`, TypeNumber, "4"}, {"x.tên", TypeNumber, "5"},
			},
		},
		{
			"top-level array",
			`[{"id":1}]`,
			[]Field{{"[0].id", TypeNumber, "1"}},
		},
		{"empty root object has no fields", `{}`, nil},
		{"scalar root has no fields", `"just text"`, nil},
		{
			"strings keep escapes decoded and unicode intact",
			`{"s":"line\nbreak é <b>"}`,
			[]Field{{"s", TypeString, "line\nbreak é <b>"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flatten(t, tt.body); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"duplicate key", `{"a":1,"a":2}`, `duplicate key "a"`},
		{"trailing data", `{} {}`, "unexpected data"},
		{"too deep", strings.Repeat("[", maxDepth+2) + strings.Repeat("]", maxDepth+2), "nesting deeper"},
		{"invalid JSON", `{"a":}`, ""}, // the wording is encoding/json's, not ours
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode([]byte(tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
	if _, err := Decode([]byte(`{"a":1,"a":2}`)); !errors.Is(err, ErrDuplicateKey) {
		t.Errorf("duplicate key error does not wrap ErrDuplicateKey: %v", err)
	}
}
