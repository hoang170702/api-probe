package field

import (
	"strings"
	"testing"
)

func mustDecode(t *testing.T, s string) *Node {
	t.Helper()
	n, err := Decode([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func mustPath(t *testing.T, p string) []Segment {
	t.Helper()
	segs, err := ParsePath(p)
	if err != nil {
		t.Fatal(err)
	}
	return segs
}

const sample = `{"requestId":"REQ001","data":{"amount":100000,"account":{"id":"0123456789"},"note":"test"},"items":[{"sku":"A1","qty":2}]}`

func TestEncodeRoundTrip(t *testing.T) {
	for _, in := range []string{
		sample,
		`{"z":1,"a":{"m":[true,false,null,"",[],{}]},"acc":12345678901234567890,"amt":100000.50,"e":1e-7}`,
		`{"s":"tên <b>&\"q\"\\ \n\t"}`,
		`[{"id":1},{"id":2}]`,
		`"just text"`,
	} {
		got := string(Encode(mustDecode(t, in)))
		if got != in {
			t.Errorf("Encode(Decode(%s))\n got  %s", in, got)
		}
	}
}

func TestEncodeCompactsWhitespace(t *testing.T) {
	got := string(Encode(mustDecode(t, "{ \"a\" : [ 1 , 2 ] ,\n \"b\" : { } }")))
	if want := `{"a":[1,2],"b":{}}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestCloneIsIndependent(t *testing.T) {
	orig := mustDecode(t, sample)
	c := orig.Clone()
	if err := Set(c, mustPath(t, "data.amount"), &Node{Kind: Number, Text: "5"}); err != nil {
		t.Fatal(err)
	}
	if err := Delete(c, mustPath(t, "items[0].sku")); err != nil {
		t.Fatal(err)
	}
	if got := string(Encode(orig)); got != sample {
		t.Errorf("original changed: %s", got)
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		path  string
		value *Node
		want  string
	}{
		{"data.amount", &Node{Kind: Number, Text: "12345678901234567890"}, `"amount":12345678901234567890`},
		{"data.note", &Node{Kind: String, Text: ""}, `"note":""`},
		{"data.note", &Node{Kind: Null}, `"note":null`},
		{"items[0].qty", &Node{Kind: Bool, Text: "true"}, `"qty":true`},
		{"data.account", mustDecode(t, `{"id":"X","type":"SAVING"}`), `"account":{"id":"X","type":"SAVING"}`},
		{"items[0]", mustDecode(t, `{"sku":"B2"}`), `"items":[{"sku":"B2"}]`},
	}
	for _, tt := range tests {
		root := mustDecode(t, sample)
		if err := Set(root, mustPath(t, tt.path), tt.value); err != nil {
			t.Fatalf("Set(%s): %v", tt.path, err)
		}
		if got := string(Encode(root)); !strings.Contains(got, tt.want) {
			t.Errorf("Set(%s) = %s, want it to contain %s", tt.path, got, tt.want)
		}
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		body, path, want string
	}{
		{sample, "data.note", `{"requestId":"REQ001","data":{"amount":100000,"account":{"id":"0123456789"}},"items":[{"sku":"A1","qty":2}]}`},
		{sample, "items[0].sku", `{"requestId":"REQ001","data":{"amount":100000,"account":{"id":"0123456789"},"note":"test"},"items":[{"qty":2}]}`},
		{sample, "data.account.id", `{"requestId":"REQ001","data":{"amount":100000,"account":{},"note":"test"},"items":[{"sku":"A1","qty":2}]}`},
		{`{"a":1,"b":2,"c":3}`, "b", `{"a":1,"c":3}`},
	}
	for _, tt := range tests {
		root := mustDecode(t, tt.body)
		if err := Delete(root, mustPath(t, tt.path)); err != nil {
			t.Fatalf("Delete(%s): %v", tt.path, err)
		}
		if got := string(Encode(root)); got != tt.want {
			t.Errorf("Delete(%s)\n got  %s\n want %s", tt.path, got, tt.want)
		}
	}
}

func TestEditMissingPaths(t *testing.T) {
	root := mustDecode(t, sample)
	for _, p := range []string{"nope", "data.nope", "items[5].sku", "requestId.x", "data[0]"} {
		if err := Set(root, mustPath(t, p), &Node{Kind: Null}); err == nil {
			t.Errorf("Set(%s): want an error", p)
		}
		if err := Delete(root, mustPath(t, p)); err == nil {
			t.Errorf("Delete(%s): want an error", p)
		}
	}
	if err := Delete(root, mustPath(t, "items[0]")); err == nil {
		t.Error("deleting an array element must be refused")
	}
	if got := string(Encode(root)); got != sample {
		t.Errorf("failed edits must not change the tree: %s", got)
	}
}
