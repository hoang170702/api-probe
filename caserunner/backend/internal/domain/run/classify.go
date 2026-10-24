package run

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
)

// Classify looks for expect in the response (BRD-001 §12, D14): first as
// the whole HTTP status code, then case-sensitively inside each header as
// "Name: value", then inside the body. The status must match exactly, or a
// code such as 00 would match every HTTP 200 and 500. It returns where the
// code was found, or "" when it was not.
func Classify(expect string, status int, headers []curl.Header, body []byte) string {
	if expect == "" {
		return ""
	}
	if expect == strconv.Itoa(status) {
		return InStatus
	}
	for _, h := range headers {
		if strings.Contains(h.Name+": "+h.Value, expect) {
			return InHeader
		}
	}
	if bytes.Contains(body, []byte(expect)) {
		return InBody
	}
	return ""
}

// Judge sets the status and reason of a case that got a response.
func Judge(res *CaseResult, headers []curl.Header, body []byte) {
	res.FoundIn = Classify(res.Expect, res.HTTPStatus, headers, body)
	if res.FoundIn != "" {
		res.Status = Success
		res.Reason = fmt.Sprintf("Found %q in the %s (HTTP %d)", res.Expect, res.FoundIn, res.HTTPStatus)
		return
	}
	res.Status = Failed
	res.Reason = fmt.Sprintf("%q not found in the response (HTTP %d)", res.Expect, res.HTTPStatus)
}
