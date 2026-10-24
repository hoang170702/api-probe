// Package field reads a JSON body as an ordered tree and lists the fields a
// tester can vary (BRD-001 §7). Key order and number text are preserved:
// nothing is decoded into a map or a float64.
package field

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Kind is the JSON type of a node.
type Kind uint8

const (
	Null Kind = iota
	Bool
	Number
	String
	Object
	Array
)

// Member is one key of an object, in body order.
type Member struct {
	Key   string
	Value *Node
}

// Node is a JSON value. Text holds a number's literal text, a string's
// decoded value, or "true"/"false".
type Node struct {
	Kind    Kind
	Text    string
	Members []Member // Object
	Items   []*Node  // Array
}

// maxDepth bounds nesting so a hostile body cannot exhaust the stack.
const maxDepth = 512

// ErrDuplicateKey is returned when an object repeats a key: a path would
// then point at two values.
var ErrDuplicateKey = errors.New("duplicate key")

// Decode parses one JSON value, keeping key order and number text.
func Decode(data []byte) (*Node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	n, err := decodeValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the JSON value")
	}
	return n, nil
}

func decodeValue(dec *json.Decoder, depth int) (*Node, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("nesting deeper than %d levels", maxDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		if v == '{' {
			return decodeObject(dec, depth)
		}
		return decodeArray(dec, depth)
	case string:
		return &Node{Kind: String, Text: v}, nil
	case json.Number:
		return &Node{Kind: Number, Text: v.String()}, nil
	case bool:
		if v {
			return &Node{Kind: Bool, Text: "true"}, nil
		}
		return &Node{Kind: Bool, Text: "false"}, nil
	default: // nil
		return &Node{Kind: Null}, nil
	}
}

func decodeObject(dec *json.Decoder, depth int) (*Node, error) {
	n := &Node{Kind: Object}
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key := tok.(string) // the decoder only yields string keys inside an object
		if seen[key] {
			return nil, fmt.Errorf("%w %q", ErrDuplicateKey, key)
		}
		seen[key] = true
		val, err := decodeValue(dec, depth+1)
		if err != nil {
			return nil, err
		}
		n.Members = append(n.Members, Member{Key: key, Value: val})
	}
	_, err := dec.Token() // }
	return n, err
}

func decodeArray(dec *json.Decoder, depth int) (*Node, error) {
	n := &Node{Kind: Array}
	for dec.More() {
		val, err := decodeValue(dec, depth+1)
		if err != nil {
			return nil, err
		}
		n.Items = append(n.Items, val)
	}
	_, err := dec.Token() // ]
	return n, err
}
