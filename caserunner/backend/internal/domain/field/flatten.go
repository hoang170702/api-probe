package field

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Type is a field's JSON type as shown to the tester.
type Type string

const (
	TypeString  Type = "string"
	TypeNumber  Type = "number"
	TypeBoolean Type = "boolean"
	TypeNull    Type = "null"
	TypeObject  Type = "object" // an empty object, listed so the tester sees it
	TypeArray   Type = "array"  // an empty array, listed so the tester sees it
)

// Cell tokens for values a plain text cell cannot express (BRD-001 §9).
const (
	TokenEmpty = "<EMPTY>" // send ""
	TokenNull  = "<NULL>"  // send null
)

// Field is one leaf of the body.
type Field struct {
	Path string
	Type Type
	// Sample is the value in the curl, written the way a tester would type
	// it in a cell, so a row of samples rebuilds the original body.
	Sample string
}

// Flatten lists the leaves of root in body order. A scalar root has no
// fields; an empty object or array below the root is itself a field.
func Flatten(root *Node) []Field {
	var out []Field
	var walk func(n *Node, path string)
	walk = func(n *Node, path string) {
		switch n.Kind {
		case Object:
			if len(n.Members) == 0 && path != "" {
				out = append(out, Field{Path: path, Type: TypeObject, Sample: "{}"})
			}
			for _, m := range n.Members {
				walk(m.Value, appendKey(path, m.Key))
			}
		case Array:
			if len(n.Items) == 0 && path != "" {
				out = append(out, Field{Path: path, Type: TypeArray, Sample: "[]"})
			}
			for i, item := range n.Items {
				walk(item, path+"["+strconv.Itoa(i)+"]")
			}
		default:
			if path != "" {
				out = append(out, leaf(n, path))
			}
		}
	}
	walk(root, "")
	return out
}

func leaf(n *Node, path string) Field {
	switch n.Kind {
	case String:
		if n.Text == "" {
			return Field{Path: path, Type: TypeString, Sample: TokenEmpty}
		}
		return Field{Path: path, Type: TypeString, Sample: n.Text}
	case Number:
		return Field{Path: path, Type: TypeNumber, Sample: n.Text}
	case Bool:
		return Field{Path: path, Type: TypeBoolean, Sample: n.Text}
	default:
		return Field{Path: path, Type: TypeNull, Sample: TokenNull}
	}
}

// appendKey adds an object key to a path: a.b normally, a["x.y"] when the
// key would otherwise be ambiguous (it is empty or holds . [ ] " or \).
func appendKey(path, key string) string {
	if key != "" && !strings.ContainsAny(key, `.[]"\`) {
		if path == "" {
			return key
		}
		return path + "." + key
	}
	return path + "[" + quote(key) + "]"
}

func quote(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // encoding a string cannot fail
	return strings.TrimSuffix(b.String(), "\n")
}
