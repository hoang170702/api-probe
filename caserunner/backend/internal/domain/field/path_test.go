package field

import (
	"reflect"
	"strings"
	"testing"
)

// Every path Flatten writes must lead back to its own value.
func TestFlattenPathsRoundTrip(t *testing.T) {
	root, err := Decode([]byte(`{
	  "requestId": "REQ001",
	  "data": {"amount": 100000, "account": {"id": "0123456789"}, "urgent": false, "coupon": null},
	  "items": [{"sku": "A1"}, {"tags": ["x", ["y"]]}],
	  "a.b": 1, "x": {"[0]": 2, "": 3, "say \"hi\"": 4, "tên": 5}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range Flatten(root) {
		segs, err := ParsePath(f.Path)
		if err != nil {
			t.Errorf("ParsePath(%q): %v", f.Path, err)
			continue
		}
		n, ok := Lookup(root, segs)
		if !ok {
			t.Errorf("Lookup(%q) found nothing", f.Path)
			continue
		}
		if got := leaf(n, f.Path); got != f {
			t.Errorf("Lookup(%q) = %+v, want %+v", f.Path, got, f)
		}
	}
}

func TestParsePath(t *testing.T) {
	got, err := ParsePath(`data.items[12]["a.b"].c`)
	if err != nil {
		t.Fatal(err)
	}
	want := []Segment{{Key: "data"}, {Key: "items"}, {Index: 12, IsIndex: true}, {Key: "a.b"}, {Key: "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParsePathErrors(t *testing.T) {
	for _, p := range []string{"", ".a", "a..b", "a[", "a[x]", "a[-1]", `a["b"`, `a["b]`, "a.[0]"} {
		if _, err := ParsePath(p); err == nil {
			t.Errorf("ParsePath(%q): want an error", p)
		}
	}
}

func TestLookupMisses(t *testing.T) {
	root, _ := Decode([]byte(`{"a":{"b":[1]}}`))
	for _, p := range []string{"z", "a.c", "a.b[1]", "a.b.c", "a[0]"} {
		segs, err := ParsePath(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if n, ok := Lookup(root, segs); ok {
			t.Errorf("Lookup(%q) = %+v, want a miss", p, n)
		}
	}
	if !strings.Contains(func() string { _, err := ParsePath("a[x]"); return err.Error() }(), "invalid array index") {
		t.Error("index error should name the problem")
	}
}
