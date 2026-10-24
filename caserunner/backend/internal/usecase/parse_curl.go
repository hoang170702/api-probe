// Package usecase holds the application rules of API Case Runner: each use case
// coordinates the domain packages and, from Step 5 on, the ports it needs.
package usecase

import (
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/field"
)

// ParsedCurl is the request template and the fields a tester can vary.
type ParsedCurl struct {
	Template curl.Template
	Fields   []field.Field
	Warnings []string
}

// ParseCurl turns pasted curl text into a template and its field list (BRD-001 §5 steps 1–2).
type ParseCurl struct{}

// Execute returns a *curl.ParseError when the command or its body cannot be used.
func (ParseCurl) Execute(input string) (ParsedCurl, error) {
	res, err := curl.Parse(input)
	if err != nil {
		return ParsedCurl{}, err
	}
	out := ParsedCurl{Template: res.Template, Warnings: res.Warnings}
	if res.Template.BodyKind != curl.BodyJSON {
		return out, nil
	}
	root, err := field.Decode([]byte(res.Template.BodyRaw))
	if err != nil {
		return ParsedCurl{}, &curl.ParseError{Code: curl.CodeUnsupported, Message: "The JSON body cannot be split into fields: " + err.Error()}
	}
	out.Fields = field.Flatten(root)
	if len(out.Fields) == 0 {
		out.Warnings = append(out.Warnings, "The JSON body has no fields to vary")
	}
	return out, nil
}
