package field

import "bytes"

// Encode writes n as compact JSON, keeping key order and number text as
// they were decoded.
func Encode(n *Node) []byte {
	var b bytes.Buffer
	encode(&b, n)
	return b.Bytes()
}

func encode(b *bytes.Buffer, n *Node) {
	switch n.Kind {
	case Object:
		b.WriteByte('{')
		for i, m := range n.Members {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(quote(m.Key))
			b.WriteByte(':')
			encode(b, m.Value)
		}
		b.WriteByte('}')
	case Array:
		b.WriteByte('[')
		for i, item := range n.Items {
			if i > 0 {
				b.WriteByte(',')
			}
			encode(b, item)
		}
		b.WriteByte(']')
	case String:
		b.WriteString(quote(n.Text))
	case Number, Bool:
		b.WriteString(n.Text)
	default:
		b.WriteString("null")
	}
}
