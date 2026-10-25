package field

import "fmt"

// Clone returns a deep copy of n, so a shared template is never modified.
func (n *Node) Clone() *Node {
	c := &Node{Kind: n.Kind, Text: n.Text}
	if n.Members != nil {
		c.Members = make([]Member, len(n.Members))
		for i, m := range n.Members {
			c.Members[i] = Member{Key: m.Key, Value: m.Value.Clone()}
		}
	}
	if n.Items != nil {
		c.Items = make([]*Node, len(n.Items))
		for i, item := range n.Items {
			c.Items[i] = item.Clone()
		}
	}
	return c
}

// Set replaces the value at segs, which must already exist in root.
func Set(root *Node, segs []Segment, value *Node) error {
	if len(segs) == 0 {
		return fmt.Errorf("empty path")
	}
	parent, ok := Lookup(root, segs[:len(segs)-1])
	if !ok {
		return fmt.Errorf("path not found")
	}
	last := segs[len(segs)-1]
	if last.IsIndex {
		if parent.Kind != Array || last.Index >= len(parent.Items) {
			return fmt.Errorf("path not found")
		}
		parent.Items[last.Index] = value
		return nil
	}
	if parent.Kind != Object {
		return fmt.Errorf("path not found")
	}
	for i := range parent.Members {
		if parent.Members[i].Key == last.Key {
			parent.Members[i].Value = value
			return nil
		}
	}
	return fmt.Errorf("path not found")
}

// Delete removes the object key at segs (BRD-001 §10): inside an array
// element the key is removed from that element, never the element itself,
// and a parent left empty stays {}.
func Delete(root *Node, segs []Segment) error {
	if len(segs) == 0 {
		return fmt.Errorf("empty path")
	}
	last := segs[len(segs)-1]
	if last.IsIndex {
		return fmt.Errorf("only an object key can be removed, not an array element")
	}
	parent, ok := Lookup(root, segs[:len(segs)-1])
	if !ok || parent.Kind != Object {
		return fmt.Errorf("path not found")
	}
	for i, m := range parent.Members {
		if m.Key == last.Key {
			parent.Members = append(parent.Members[:i:i], parent.Members[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("path not found")
}
