package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/field"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/run"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/testcase"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// maxRunRequest bounds a run request: up to 1000 cases with their cells.
const maxRunRequest = 8 << 20

// pingEvery keeps proxies from closing an idle SSE stream (BRD-001 §15.5).
var pingEvery = 15 * time.Second

// DTOs of BRD-001 §15.3–§15.5.
type startRunRequest struct {
	Template templateDTO `json:"template"`
	Fields   []ruleDTO   `json:"fields"`
	Cases    []caseDTO   `json:"cases"`
	Prepare  *struct {
		Steps       []prepareStepDTO `json:"steps"`
		InjectToken *bool            `json:"injectToken"`
	} `json:"prepare"`
	Options struct {
		Concurrency int `json:"concurrency"`
		TimeoutMs   int `json:"timeoutMs"`
	} `json:"options"`
}

type ruleDTO struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Rule string `json:"rule"` // REQUIRED | UNREQUIRED
}

type caseDTO struct {
	No     int               `json:"no"`
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
	Expect string            `json:"expect"`
}

type startRunResponse struct {
	RunID   string `json:"runId"`
	Total   int    `json:"total"`
	Blocked int    `json:"blocked"`
}

type runDTO struct {
	RunID     string          `json:"runId"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"createdAt"`
	Results   []caseResultDTO `json:"results"`
	Summary   summaryDTO      `json:"summary"`
}

type doneDTO struct {
	RunID   string     `json:"runId"`
	Status  string     `json:"status"`
	Summary summaryDTO `json:"summary"`
}

type summaryDTO struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Success   int `json:"success"`
	Failed    int `json:"failed"`
	Exception int `json:"exception"`
	Blocked   int `json:"blocked"`
	Cancelled int `json:"cancelled"`
}

type caseResultDTO struct {
	CaseIndex    int                  `json:"caseIndex"`
	CaseNo       int                  `json:"caseNo"`
	Name         string               `json:"name"`
	Status       string               `json:"status"`
	Reason       string               `json:"reason"`
	Errors       []validationErrorDTO `json:"errors,omitempty"`
	ErrorType    string               `json:"errorType,omitempty"`
	Input        map[string]string    `json:"input"`
	RequestBody  string               `json:"requestBody,omitempty"`
	Expect       string               `json:"expect"`
	FoundIn      string               `json:"foundIn,omitempty"`
	HTTPStatus   int                  `json:"httpStatus,omitempty"`
	ResponseBody string               `json:"responseBody,omitempty"`
	Truncated    bool                 `json:"truncated,omitempty"`
	StartedAt    *time.Time           `json:"startedAt,omitempty"`
	DurationMs   int64                `json:"durationMs"`
}

type validationErrorDTO struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func startRun(uc *usecase.Runs) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req startRunRequest
		if !decodeJSON(w, r, maxRunRequest, &req) {
			return
		}
		in, err := toStartRunInput(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		out, err := uc.Start(in)
		var ie *usecase.InputError
		switch {
		case errors.As(err, &ie):
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", ie.Message)
		case errors.Is(err, usecase.ErrStoreFull):
			writeError(w, http.StatusTooManyRequests, "TOO_MANY_RUNS", "Too many runs are in progress; wait for one to finish")
		case err != nil:
			slog.Error("start run", "err", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL", "Internal server error")
		default:
			writeJSON(w, http.StatusAccepted, startRunResponse{RunID: out.RunID, Total: out.Total, Blocked: out.Blocked})
		}
	}
}

func toStartRunInput(req startRunRequest) (usecase.StartRunInput, error) {
	in := usecase.StartRunInput{
		Template: curl.Template{
			Method: req.Template.Method, URL: req.Template.URL, BodyRaw: req.Template.BodyRaw,
			BodyKind: curl.BodyKind(req.Template.BodyKind), Insecure: req.Template.Insecure,
		},
		Concurrency: req.Options.Concurrency,
		TimeoutMs:   req.Options.TimeoutMs,
		InjectToken: true,
	}
	if in.Template.Method == "" || in.Template.URL == "" {
		return in, fmt.Errorf("the template needs a method and a URL")
	}
	for _, h := range req.Template.Headers {
		in.Template.Headers = append(in.Template.Headers, curl.Header{Name: h.Name, Value: h.Value})
	}
	for _, f := range req.Fields {
		if f.Rule != "REQUIRED" && f.Rule != "UNREQUIRED" {
			return in, fmt.Errorf("field %s: rule must be REQUIRED or UNREQUIRED", f.Path)
		}
		in.Rules = append(in.Rules, testcase.FieldRule{Path: f.Path, Type: field.Type(f.Type), Required: f.Rule == "REQUIRED"})
	}
	for _, c := range req.Cases {
		in.Cases = append(in.Cases, testcase.Case{No: c.No, Name: c.Name, Values: c.Values, Expect: c.Expect})
	}
	if req.Prepare != nil {
		if req.Prepare.InjectToken != nil {
			in.InjectToken = *req.Prepare.InjectToken
		}
		if len(req.Prepare.Steps) > 0 {
			steps, err := toPrepareSteps(req.Prepare.Steps)
			if err != nil {
				return in, err
			}
			in.Prepare = steps
		}
	}
	return in, nil
}

func getRun(uc *usecase.Runs) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got, err := uc.Get(r.PathValue("id"))
		if err != nil {
			runNotFound(w)
			return
		}
		writeJSON(w, http.StatusOK, toRunDTO(got))
	}
}

func cancelRun(uc *usecase.Runs) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := uc.Cancel(id); err != nil {
			runNotFound(w)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"runId": id})
	}
}

func runNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "RUN_NOT_FOUND", "Run has expired, please run it again")
}

// runEvents streams a run as Server-Sent Events: a snapshot first, then
// one "case" event per change, then "done" (BRD-001 §15.5).
func runEvents(uc *usecase.Runs) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, events, stop, err := uc.Watch(r.PathValue("id"))
		if err != nil {
			runNotFound(w)
			return
		}
		defer stop()
		rc := http.NewResponseController(w)
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		send := func(event string, v any) bool {
			b, _ := json.Marshal(v)
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
				return false
			}
			return rc.Flush() == nil
		}
		if !send("snapshot", toRunDTO(snap)) {
			return
		}
		if events == nil { // the run had already finished
			send("done", toDoneDTO(snap))
			return
		}
		ping := time.NewTicker(pingEvery)
		defer ping.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-uc.Base.Done(): // shutting down
				return
			case <-ping.C:
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
					return
				}
			case ev, open := <-events:
				if !open { // dropped as a slow watcher: the client reconnects
					return
				}
				switch ev.Kind {
				case usecase.EventCase:
					if !send("case", toCaseResultDTO(*ev.Case)) {
						return
					}
				case usecase.EventDone:
					send("done", toDoneDTO(*ev.Run))
					return
				}
			}
		}
	}
}

func toRunDTO(r run.Run) runDTO {
	d := runDTO{RunID: r.ID, Status: string(r.Status), CreatedAt: r.CreatedAt, Results: make([]caseResultDTO, len(r.Results)), Summary: toSummaryDTO(r.Summary())}
	for i, c := range r.Results {
		d.Results[i] = toCaseResultDTO(c)
	}
	return d
}

func toDoneDTO(r run.Run) doneDTO {
	return doneDTO{RunID: r.ID, Status: string(r.Status), Summary: toSummaryDTO(r.Summary())}
}

func toSummaryDTO(s run.Summary) summaryDTO {
	return summaryDTO{Total: s.Total, Pending: s.Pending, Running: s.Running, Success: s.Success, Failed: s.Failed, Exception: s.Exception, Blocked: s.Blocked, Cancelled: s.Cancelled}
}

func toCaseResultDTO(c run.CaseResult) caseResultDTO {
	d := caseResultDTO{
		CaseIndex: c.CaseIndex, CaseNo: c.CaseNo, Name: c.Name, Status: string(c.Status), Reason: c.Reason,
		ErrorType: c.ErrorType, Input: c.Input, RequestBody: c.RequestBody, Expect: c.Expect, FoundIn: c.FoundIn,
		HTTPStatus: c.HTTPStatus, ResponseBody: c.ResponseBody, Truncated: c.Truncated, DurationMs: c.Duration.Milliseconds(),
	}
	if d.Input == nil {
		d.Input = map[string]string{}
	}
	if !c.StartedAt.IsZero() {
		t := c.StartedAt
		d.StartedAt = &t
	}
	for _, e := range c.Errors {
		d.Errors = append(d.Errors, validationErrorDTO{Field: e.Field, Code: e.Code, Message: e.Message})
	}
	return d
}
