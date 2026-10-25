// Package testcase validates the cells of one case and builds its request
// body (BRD-001 §9, §10).
package testcase

import (
	"fmt"
	"strings"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/field"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"
)

// FieldRule is one body field and whether every case must fill it (D3).
type FieldRule struct {
	Path     string
	Type     field.Type
	Required bool
}

// Case is one row of the case table: cell text keyed by field path.
type Case struct {
	No     int
	Name   string
	Values map[string]string
	Expect string
}

// Validate returns every reason the case cannot be sent; none means valid.
func Validate(c Case, rules []FieldRule) []run.ValidationError {
	var errs []run.ValidationError
	for _, r := range rules {
		cell := c.Values[r.Path]
		if cell == "" {
			if r.Required {
				errs = append(errs, run.ValidationError{
					Field: r.Path, Code: run.CodeRequiredMissing,
					Message: fmt.Sprintf("Field %s is Required. Enter a value or change it to Unrequired.", r.Path),
				})
			}
			continue
		}
		if _, err := value(r.Type, cell); err != nil {
			errs = append(errs, run.ValidationError{
				Field: r.Path, Code: run.CodeTypeMismatch,
				Message: fmt.Sprintf("Field %s expects type %s, got '%s'", r.Path, r.Type, cell),
			})
		}
	}
	if strings.TrimSpace(c.Expect) == "" {
		errs = append(errs, run.ValidationError{Code: run.CodeInvalidExpectation, Message: "Case needs an expect code"})
	}
	return errs
}

// Build returns t with its JSON body rebuilt from the case: a filled cell
// replaces the field's value, an empty Unrequired cell removes the key.
// Method, URL and headers are unchanged. Call it only for a valid case.
func Build(t curl.Template, rules []FieldRule, c Case) (curl.Template, error) {
	root, err := field.Decode([]byte(t.BodyRaw))
	if err != nil {
		return curl.Template{}, fmt.Errorf("the template body is not JSON: %w", err)
	}
	for _, r := range rules {
		segs, err := field.ParsePath(r.Path)
		if err != nil {
			return curl.Template{}, fmt.Errorf("field %s: %w", r.Path, err)
		}
		cell := c.Values[r.Path]
		if cell == "" {
			if r.Required {
				return curl.Template{}, fmt.Errorf("field %s is Required but empty", r.Path)
			}
			if err := field.Delete(root, segs); err != nil {
				return curl.Template{}, fmt.Errorf("field %s: %w", r.Path, err)
			}
			continue
		}
		v, err := value(r.Type, cell)
		if err != nil {
			return curl.Template{}, fmt.Errorf("field %s: %w", r.Path, err)
		}
		if err := field.Set(root, segs, v); err != nil {
			return curl.Template{}, fmt.Errorf("field %s: %w", r.Path, err)
		}
	}
	out := t
	out.BodyRaw = string(field.Encode(root))
	return out, nil
}

// value turns cell text into a JSON value of the field's type (BRD-001 §9).
func value(typ field.Type, cell string) (*field.Node, error) {
	switch cell {
	case field.TokenNull:
		return &field.Node{Kind: field.Null}, nil // valid for every type
	case field.TokenEmpty:
		return &field.Node{Kind: field.String, Text: ""}, nil // a deliberate "" for any type
	}
	switch typ {
	case field.TypeString:
		return &field.Node{Kind: field.String, Text: cell}, nil
	case field.TypeNumber:
		if n, err := field.Decode([]byte(cell)); err == nil && n.Kind == field.Number {
			return n, nil // the text is kept: no float conversion
		}
	case field.TypeBoolean:
		if cell == "true" || cell == "false" {
			return &field.Node{Kind: field.Bool, Text: cell}, nil
		}
	case field.TypeObject:
		if n, err := field.Decode([]byte(cell)); err == nil && n.Kind == field.Object {
			return n, nil
		}
	case field.TypeArray:
		if n, err := field.Decode([]byte(cell)); err == nil && n.Kind == field.Array {
			return n, nil
		}
	default: // TypeNull: the curl sent null, so any JSON value fits; other text is a string
		if n, err := field.Decode([]byte(cell)); err == nil {
			return n, nil
		}
		return &field.Node{Kind: field.String, Text: cell}, nil
	}
	return nil, fmt.Errorf("expects type %s", typ)
}
