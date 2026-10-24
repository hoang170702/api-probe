package run

import (
	"testing"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
)

func TestClassify(t *testing.T) {
	headers := []curl.Header{{Name: "Content-Type", Value: "application/json"}, {Name: "X-Result-Code", Value: "E42"}}
	tests := []struct {
		name   string
		expect string
		status int
		body   string
		want   string
	}{
		{"code in the body of an error response", "ERR_AMOUNT_INVALID", 400, `{"code":"ERR_AMOUNT_INVALID"}`, InBody},
		{"status code itself", "401", 401, `{"code":"AUTH"}`, InStatus},
		{"status wins over the body", "200", 200, `{"code":"200"}`, InStatus},
		{"header line", "X-Result-Code: E42", 200, `{}`, InHeader},
		{"header value only", "E42", 200, `{}`, InHeader},
		{"success code with its key", `"code":"00"`, 200, `{"amount":100000,"code":"00"}`, InBody},
		{"short code matches inside a number in the body (accepted, D14)", "00", 500, `{"amount":100000}`, InBody},
		{"00 does not match HTTP 200 or 500 by itself", "00", 200, `{"code":"99"}`, ""},
		{"a part of the status does not match", "40", 401, `{}`, ""},
		{"case-sensitive", "err_x", 400, `{"code":"ERR_X"}`, ""},
		{"not found", "ERR_X", 500, `{"code":"99"}`, ""},
		{"empty expect never matches", "", 200, `{}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.expect, tt.status, headers, []byte(tt.body)); got != tt.want {
				t.Errorf("Classify = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJudge(t *testing.T) {
	ok := CaseResult{Expect: "ERR_X", HTTPStatus: 400}
	Judge(&ok, nil, []byte(`{"code":"ERR_X"}`))
	if ok.Status != Success || ok.FoundIn != InBody || ok.Reason != `Found ERR_X in the body (HTTP 400)` {
		t.Errorf("found: %+v", ok)
	}
	quoted := CaseResult{Expect: `"code":"00"`, HTTPStatus: 200}
	Judge(&quoted, nil, []byte(`{"code":"00"}`))
	if quoted.Reason != `Found "code":"00" in the body (HTTP 200)` {
		t.Errorf("quoted expect reason = %q", quoted.Reason)
	}
	miss := CaseResult{Expect: "00", HTTPStatus: 500}
	Judge(&miss, nil, []byte(`{"code":"99"}`))
	if miss.Status != Failed || miss.FoundIn != "" || miss.Reason != `00 not found in the response (HTTP 500)` {
		t.Errorf("not found: %+v", miss)
	}
}

func TestSummaryAndClone(t *testing.T) {
	r := Run{Results: []CaseResult{
		{Status: Success}, {Status: Success}, {Status: Failed}, {Status: Blocked, Errors: []ValidationError{{Field: "a"}}},
		{Status: Exception}, {Status: Cancelled}, {Status: Pending, Input: map[string]string{"a": "1"}}, {Status: Running},
	}}
	want := Summary{Total: 8, Success: 2, Failed: 1, Blocked: 1, Exception: 1, Cancelled: 1, Pending: 1, Running: 1}
	if got := r.Summary(); got != want {
		t.Errorf("Summary = %+v, want %+v", got, want)
	}

	c := r.Clone()
	c.Results[0].Status = Failed
	c.Results[3].Errors[0].Field = "changed"
	c.Results[6].Input["a"] = "changed"
	if r.Results[0].Status != Success || r.Results[3].Errors[0].Field != "a" || r.Results[6].Input["a"] != "1" {
		t.Error("Clone shares memory with the original")
	}
	if !Success.Finished() || Pending.Finished() || Running.Finished() {
		t.Error("Finished")
	}
}
