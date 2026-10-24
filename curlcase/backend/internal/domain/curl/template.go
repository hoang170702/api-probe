// Package curl turns a pasted curl command into a request template
// (BRD-001 §6). It never runs a shell and never expands variables.
package curl

import "fmt"

// Header is one request header. Order and duplicates are preserved.
type Header struct {
	Name  string
	Value string
}

// BodyKind says what the request body is; v1 runs only JSON bodies.
type BodyKind string

const (
	BodyNone        BodyKind = "none"
	BodyJSON        BodyKind = "json"
	BodyUnsupported BodyKind = "unsupported"
)

// Template is the request described by the curl command.
type Template struct {
	Method   string
	URL      string
	Headers  []Header
	BodyRaw  string
	BodyKind BodyKind
	Insecure bool // -k/--insecure was given; honoring it is a policy decision (§13)
}

// Result is a parsed command plus the non-fatal notes for the tester.
type Result struct {
	Template Template
	Warnings []string
}

// Error codes of ParseError, part of the API error format (§15.1).
const (
	CodeParseError  = "CURL_PARSE_ERROR" // the text is not a usable curl command
	CodeUnsupported = "CURL_UNSUPPORTED" // valid curl, but uses a feature v1 cannot run
)

// ParseError explains why a command was rejected. Pos is the 1-based
// character position in the input it refers to, or 0 when there is none.
type ParseError struct {
	Code    string
	Message string
	Pos     int
}

func (e *ParseError) Error() string {
	if e.Pos > 0 {
		return fmt.Sprintf("%s at position %d", e.Message, e.Pos)
	}
	return e.Message
}

func syntaxError(pos int, format string, args ...any) *ParseError {
	return &ParseError{Code: CodeParseError, Message: fmt.Sprintf(format, args...), Pos: pos}
}

func unsupported(pos int, format string, args ...any) *ParseError {
	return &ParseError{Code: CodeUnsupported, Message: fmt.Sprintf(format, args...), Pos: pos}
}
