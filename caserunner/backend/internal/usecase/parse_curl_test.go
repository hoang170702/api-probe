package usecase

import (
	"errors"
	"slices"
	"testing"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
)

func TestParseCurlWithoutAJSONBodyHasNoFields(t *testing.T) {
	for _, in := range []string{
		"curl https://a.local",                  // no body
		"curl https://a.local -d 'a=1&b=2'",     // not JSON
		"curl https://a.local -F 'file=@x.png'", // multipart
	} {
		got, err := ParseCurl{}.Execute(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got.Fields != nil {
			t.Errorf("%s: fields = %+v, want none", in, got.Fields)
		}
	}
}

func TestParseCurlWarnsWhenTheBodyHasNoFields(t *testing.T) {
	got, err := ParseCurl{}.Execute(`curl https://a.local -H 'Content-Type: application/json' -d '{}'`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"The JSON body has no fields to vary"}; !slices.Equal(got.Warnings, want) {
		t.Errorf("warnings = %q, want %q", got.Warnings, want)
	}
}

func TestParseCurlRejectsDuplicateKeys(t *testing.T) {
	_, err := ParseCurl{}.Execute(`curl https://a.local -H 'Content-Type: application/json' -d '{"a":1,"a":2}'`)
	var pe *curl.ParseError
	if !errors.As(err, &pe) || pe.Code != curl.CodeUnsupported {
		t.Fatalf("err = %v, want a %s ParseError", err, curl.CodeUnsupported)
	}
}
