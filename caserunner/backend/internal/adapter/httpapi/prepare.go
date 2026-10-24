package httpapi

import (
	"fmt"
	"net/http"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/flow"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// maxPrepareSteps bounds one prepare chain (BRD-001 §15.7).
const maxPrepareSteps = 10

// DTOs of BRD-001 §15.7.
type prepareTryRequest struct {
	Steps   []prepareStepDTO `json:"steps"`
	Options struct {
		TimeoutMs int `json:"timeoutMs"`
	} `json:"options"`
}

type prepareStepDTO struct {
	Template templateDTO  `json:"template"`
	Captures []captureDTO `json:"captures"`
}

type captureDTO struct {
	Var  string `json:"var"`
	Path string `json:"path"`
}

type prepareTryResponse struct {
	Steps []stepResultDTO   `json:"steps"`
	Vars  map[string]string `json:"vars"`
}

type stepResultDTO struct {
	Status       string        `json:"status"`
	HTTPStatus   int           `json:"httpStatus,omitempty"`
	DurationMs   int64         `json:"durationMs"`
	Reason       string        `json:"reason"`
	ErrorType    string        `json:"errorType,omitempty"`
	Captured     []capturedDTO `json:"captured"`
	ResponseBody string        `json:"responseBody"`
	Truncated    bool          `json:"truncated"`
}

type capturedDTO struct {
	Var   string `json:"var"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func tryPrepare(uc usecase.TryPrepare) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req prepareTryRequest
		if !decodeJSON(w, r, maxParseRequest, &req) {
			return
		}
		steps, err := toPrepareSteps(req.Steps)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toPrepareTryResponse(uc.Execute(r.Context(), steps, req.Options.TimeoutMs)))
	}
}

func toPrepareSteps(in []prepareStepDTO) ([]usecase.PrepareStep, error) {
	if len(in) == 0 || len(in) > maxPrepareSteps {
		return nil, fmt.Errorf("send between 1 and %d prepare steps", maxPrepareSteps)
	}
	out := make([]usecase.PrepareStep, len(in))
	for i, s := range in {
		if s.Template.Method == "" || s.Template.URL == "" {
			return nil, fmt.Errorf("step %d: the template needs a method and a URL", i+1)
		}
		t := curl.Template{
			Method: s.Template.Method, URL: s.Template.URL, BodyRaw: s.Template.BodyRaw,
			BodyKind: curl.BodyKind(s.Template.BodyKind), Insecure: s.Template.Insecure,
		}
		for _, h := range s.Template.Headers {
			t.Headers = append(t.Headers, curl.Header{Name: h.Name, Value: h.Value})
		}
		out[i].Template = t
		for _, c := range s.Captures {
			if !flow.ValidVar(c.Var) {
				return nil, fmt.Errorf("step %d: %q is not a valid variable name (letters, digits and _, not starting with a digit)", i+1, c.Var)
			}
			out[i].Captures = append(out[i].Captures, flow.Capture{Var: c.Var, Path: c.Path})
		}
	}
	return out, nil
}

func toPrepareTryResponse(out usecase.TryPrepareOutput) prepareTryResponse {
	res := prepareTryResponse{Steps: make([]stepResultDTO, len(out.Steps)), Vars: out.Vars}
	for i, s := range out.Steps {
		d := stepResultDTO{
			Status: s.Status, HTTPStatus: s.HTTPStatus, DurationMs: s.Duration.Milliseconds(),
			Reason: s.Reason, ErrorType: s.ErrorType, Captured: []capturedDTO{},
			ResponseBody: s.ResponseBody, Truncated: s.Truncated,
		}
		for _, c := range s.Captured {
			d.Captured = append(d.Captured, capturedDTO{Var: c.Var, Path: c.Path, Value: c.Value})
		}
		res.Steps[i] = d
	}
	return res
}
