package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// maxParseRequest bounds the pasted curl text, body included.
const maxParseRequest = 2 << 20

type parseRequest struct {
	Curl string `json:"curl"`
}

// DTOs of BRD-001 §15.2.
type parseResponse struct {
	Template templateDTO `json:"template"`
	Fields   []fieldDTO  `json:"fields"`
	Warnings []string    `json:"warnings"`
}

type templateDTO struct {
	Method   string      `json:"method"`
	URL      string      `json:"url"`
	Headers  []headerDTO `json:"headers"`
	BodyRaw  string      `json:"bodyRaw"`
	BodyKind string      `json:"bodyKind"`
	Insecure bool        `json:"insecure"`
}

type headerDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type fieldDTO struct {
	Path        string `json:"path"`
	Type        string `json:"type"`
	SampleValue string `json:"sampleValue"`
}

func parseCurl(uc usecase.ParseCurl) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req parseRequest
		if !decodeJSON(w, r, maxParseRequest, &req) {
			return
		}
		out, err := uc.Execute(req.Curl)
		if err != nil {
			var pe *curl.ParseError
			if errors.As(err, &pe) {
				writeError(w, http.StatusBadRequest, pe.Code, pe.Error())
				return
			}
			slog.Error("parse curl", "err", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL", "Internal server error")
			return
		}
		writeJSON(w, http.StatusOK, toParseResponse(out))
	}
}

func toParseResponse(out usecase.ParsedCurl) parseResponse {
	t := out.Template
	res := parseResponse{
		Template: templateDTO{
			Method: t.Method, URL: t.URL, Headers: []headerDTO{},
			BodyRaw: t.BodyRaw, BodyKind: string(t.BodyKind), Insecure: t.Insecure,
		},
		Fields:   []fieldDTO{},
		Warnings: []string{},
	}
	for _, h := range t.Headers {
		res.Template.Headers = append(res.Template.Headers, headerDTO{Name: h.Name, Value: h.Value})
	}
	for _, f := range out.Fields {
		res.Fields = append(res.Fields, fieldDTO{Path: f.Path, Type: string(f.Type), SampleValue: f.Sample})
	}
	res.Warnings = append(res.Warnings, out.Warnings...)
	return res
}

// decodeJSON reads a JSON request body of at most limit bytes into v and
// writes the error response itself when it cannot.
func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, v any) bool {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit)).Decode(v)
	if err == nil {
		return true
	}
	if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
		writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "The request body is too large")
		return false
	}
	writeError(w, http.StatusBadRequest, "BAD_REQUEST", "The request body must be valid JSON")
	return false
}
