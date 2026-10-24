package field

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Segment is one step of a field path: an object key or an array index.
type Segment struct {
	Key     string
	Index   int
	IsIndex bool
}

// ParsePath reads a path written as Flatten writes it: a.b, items[0].sku,
// x["key.with.dot"].
func ParsePath(path string) ([]Segment, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	var segs []Segment
	for i := 0; i < len(path); {
		switch {
		case path[i] == '[' && i+1 < len(path) && path[i+1] == '"':
			end, key, err := quotedKey(path, i+1)
			if err != nil {
				return nil, err
			}
			if end >= len(path) || path[end] != ']' {
				return nil, fmt.Errorf("missing ] after the quoted key at character %d", end+1)
			}
			segs = append(segs, Segment{Key: key})
			i = end + 1
		case path[i] == '[':
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("missing ] at character %d", i+1)
			}
			n, err := strconv.Atoi(path[i+1 : i+end])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid array index %q", path[i+1:i+end])
			}
			segs = append(segs, Segment{Index: n, IsIndex: true})
			i += end + 1
		default:
			if path[i] == '.' {
				if i == 0 {
					return nil, fmt.Errorf("a path cannot start with .")
				}
				i++
			}
			end := i
			for end < len(path) && path[end] != '.' && path[end] != '[' {
				end++
			}
			if end == i {
				return nil, fmt.Errorf("empty key at character %d", i+1)
			}
			segs = append(segs, Segment{Key: path[i:end]})
			i = end
		}
	}
	return segs, nil
}

// quotedKey decodes the JSON string starting at path[start] (a quote) and
// returns the index just after its closing quote.
func quotedKey(path string, start int) (int, string, error) {
	for j := start + 1; j < len(path); j++ {
		switch path[j] {
		case '\\':
			j++
		case '"':
			var key string
			if err := json.Unmarshal([]byte(path[start:j+1]), &key); err != nil {
				return 0, "", fmt.Errorf("invalid quoted key %s", path[start:j+1])
			}
			return j + 1, key, nil
		}
	}
	return 0, "", fmt.Errorf("unterminated quoted key at character %d", start+1)
}

// Lookup follows segs from root and returns the node there.
func Lookup(root *Node, segs []Segment) (*Node, bool) {
	n := root
	for _, s := range segs {
		switch {
		case s.IsIndex:
			if n.Kind != Array || s.Index >= len(n.Items) {
				return nil, false
			}
			n = n.Items[s.Index]
		default:
			if n.Kind != Object {
				return nil, false
			}
			var next *Node
			for _, m := range n.Members {
				if m.Key == s.Key {
					next = m.Value
					break
				}
			}
			if next == nil {
				return nil, false
			}
			n = next
		}
	}
	return n, true
}
