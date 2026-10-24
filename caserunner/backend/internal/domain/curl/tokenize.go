package curl

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// token is one shell word and the 1-based position where it starts.
type token struct {
	value string
	pos   int
}

// tokenize splits the command into words using bash rules, or Windows cmd
// rules when the text was copied with "Copy as cURL (cmd)".
func tokenize(input string) ([]token, error) {
	src := []rune(input)
	if looksLikeCmd(input) {
		return tokenizeCmd(src)
	}
	return tokenizeBash(src)
}

// looksLikeCmd reports whether lines are continued with ^ (cmd) rather than
// \ (bash). A single line is cmd when it uses the ^" escape.
func looksLikeCmd(input string) bool {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	for _, line := range lines[:len(lines)-1] {
		line = strings.TrimRight(line, " \t")
		switch {
		case strings.HasSuffix(line, "^"):
			return true
		case strings.HasSuffix(line, `\`):
			return false
		}
	}
	return cmdQuote.MatchString(input)
}

// cmdQuote is a word opened with ^", as Chrome's cmd copy writes every
// argument; a ^" inside a bash string (a regex in a body) does not count.
var cmdQuote = regexp.MustCompile(`(^|\s)\^"`)

func isBlank(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

// newlineAt returns the length of the line break starting at i (\n or \r\n), or 0.
func newlineAt(src []rune, i int) int {
	switch {
	case i < len(src) && src[i] == '\n':
		return 1
	case i+1 < len(src) && src[i] == '\r' && src[i+1] == '\n':
		return 2
	}
	return 0
}

// words accumulates tokens; a word exists once anything, even an empty quoted string, was read.
type words struct {
	out    []token
	cur    strings.Builder
	start  int
	inWord bool
}

func (w *words) begin(pos int) {
	if !w.inWord {
		w.inWord, w.start = true, pos
	}
}

func (w *words) add(pos int, r rune) {
	w.begin(pos)
	w.cur.WriteRune(r)
}

func (w *words) end() {
	if w.inWord {
		w.out = append(w.out, token{value: w.cur.String(), pos: w.start})
		w.cur.Reset()
		w.inWord = false
	}
}

func tokenizeBash(src []rune) ([]token, error) {
	var w words
	for i := 0; i < len(src); {
		c, pos := src[i], i+1
		switch {
		case c == '\\':
			if n := newlineAt(src, i+1); n > 0 { // line continuation
				i += 1 + n
				continue
			}
			if i+1 < len(src) {
				w.add(pos, src[i+1])
			}
			i += 2
		case c == '\'':
			end := indexRune(src, i+1, '\'')
			if end < 0 {
				return nil, syntaxError(pos, "Missing closing ' for the quote opened")
			}
			w.begin(pos)
			w.cur.WriteString(string(src[i+1 : end]))
			i = end + 1
		case c == '$' && i+1 < len(src) && src[i+1] == '\'':
			next, err := readANSIC(src, i+2, &w, pos)
			if err != nil {
				return nil, err
			}
			i = next
		case c == '"':
			next, err := readDoubleQuoted(src, i+1, &w, pos)
			if err != nil {
				return nil, err
			}
			i = next
		case isBlank(c):
			w.end()
			i++
		default:
			w.add(pos, c)
			i++
		}
	}
	w.end()
	return w.out, nil
}

func indexRune(src []rune, from int, r rune) int {
	for i := from; i < len(src); i++ {
		if src[i] == r {
			return i
		}
	}
	return -1
}

// readDoubleQuoted reads a bash "..." string starting after the opening quote.
func readDoubleQuoted(src []rune, i int, w *words, openPos int) (int, error) {
	w.begin(openPos)
	for i < len(src) {
		switch c := src[i]; c {
		case '"':
			return i + 1, nil
		case '\\':
			if n := newlineAt(src, i+1); n > 0 {
				i += 1 + n
				continue
			}
			if i+1 < len(src) && strings.ContainsRune("\"\\$`", src[i+1]) {
				w.cur.WriteRune(src[i+1])
				i += 2
				continue
			}
			w.cur.WriteRune(c)
			i++
		default:
			w.cur.WriteRune(c)
			i++
		}
	}
	return 0, syntaxError(openPos, `Missing closing " for the quote opened`)
}

// readANSIC reads a bash $'...' string, which Chrome uses for bodies holding
// quotes or newlines, starting after the opening $'.
func readANSIC(src []rune, i int, w *words, openPos int) (int, error) {
	w.begin(openPos)
	simple := map[rune]rune{'n': '\n', 't': '\t', 'r': '\r', 'a': '\a', 'b': '\b', 'f': '\f', 'v': '\v', 'e': 0x1b, 'E': 0x1b, '\\': '\\', '\'': '\'', '"': '"', '?': '?'}
	for i < len(src) {
		c := src[i]
		if c == '\'' {
			return i + 1, nil
		}
		if c != '\\' || i+1 >= len(src) {
			w.cur.WriteRune(c)
			i++
			continue
		}
		e := src[i+1]
		if r, ok := simple[e]; ok {
			w.cur.WriteRune(r)
			i += 2
			continue
		}
		var digits, base int
		switch {
		case e == 'x':
			digits, base = 2, 16
		case e == 'u':
			digits, base = 4, 16
		case e == 'U':
			digits, base = 8, 16
		case e >= '0' && e <= '7':
			digits, base = 3, 8
		}
		if digits == 0 {
			w.cur.WriteRune('\\')
			w.cur.WriteRune(e)
			i += 2
			continue
		}
		start := i + 2
		if base == 8 {
			start = i + 1
		}
		end := start
		for end < len(src) && end-start < digits && isDigit(src[end], base) {
			end++
		}
		if end == start {
			w.cur.WriteRune('\\')
			w.cur.WriteRune(e)
			i += 2
			continue
		}
		n, _ := strconv.ParseUint(string(src[start:end]), base, 32)
		if base == 8 || e == 'x' {
			w.cur.WriteByte(byte(n)) // a byte, as bash does; UTF-8 sequences reassemble
		} else if utf8.ValidRune(rune(n)) {
			w.cur.WriteRune(rune(n))
		}
		i = end
	}
	return 0, syntaxError(openPos, "Missing closing ' for the $' quote opened")
}

func isDigit(r rune, base int) bool {
	switch {
	case r >= '0' && r <= '7':
		return true
	case base == 16 && (r >= '8' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'):
		return true
	}
	return false
}

// tokenizeCmd applies the two layers a Windows command line goes through:
// cmd.exe removes ^ escapes and ^ line continuations, then curl.exe splits
// the result into arguments with the Microsoft C runtime rules.
func tokenizeCmd(src []rune) ([]token, error) {
	var line []rune
	var pos []int
	inQuote := false
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == '^' && !inQuote:
			if n := newlineAt(src, i+1); n > 0 { // line continuation
				i += 1 + n
				continue
			}
			if i+1 < len(src) {
				line, pos = append(line, src[i+1]), append(pos, i+2)
			}
			i += 2
			continue
		}
		line, pos = append(line, c), append(pos, i+1)
		i++
	}
	return splitWindowsArgs(line, pos)
}

// splitWindowsArgs splits like CommandLineToArgvW: 2n backslashes before a
// quote become n and the quote toggles quoting; 2n+1 become n and a literal
// quote; "" inside quotes is a literal quote.
func splitWindowsArgs(line []rune, pos []int) ([]token, error) {
	var w words
	inQuote, quotePos := false, 0
	for i := 0; i < len(line); {
		c := line[i]
		switch {
		case c == '\\':
			j := i
			for j < len(line) && line[j] == '\\' {
				j++
			}
			n := j - i
			if j < len(line) && line[j] == '"' {
				for range n / 2 {
					w.add(pos[i], '\\')
				}
				if n%2 == 1 {
					w.add(pos[i], '"')
					i = j + 1
				} else {
					i = j // the quote is handled next
				}
				continue
			}
			for k := i; k < j; k++ {
				w.add(pos[k], '\\')
			}
			i = j
		case c == '"':
			if inQuote && i+1 < len(line) && line[i+1] == '"' {
				w.add(pos[i], '"')
				i += 2
				continue
			}
			w.begin(pos[i])
			inQuote = !inQuote
			if inQuote {
				quotePos = pos[i]
			}
			i++
		case isBlank(c) && !inQuote:
			w.end()
			i++
		default:
			w.add(pos[i], c)
			i++
		}
	}
	if inQuote {
		return nil, syntaxError(quotePos, `Missing closing " for the quote opened`)
	}
	w.end()
	return w.out, nil
}
