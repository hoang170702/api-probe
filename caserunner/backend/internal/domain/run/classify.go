package run

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
)

// Classify looks for expect in the response (BRD-001 §12, D14): first as
// the whole HTTP status code, then case-sensitively inside the body, then
// inside each application header as "Name: value". The status must match
// exactly and transport headers are skipped, or a code such as 00 would
// match every HTTP 200/500, every Date and many Content-Length values. It
// returns where the code was found, or "" when it was not.
func Classify(expect string, status int, headers []curl.Header, body []byte) string {
	if expect == "" {
		return ""
	}
	if expect == strconv.Itoa(status) {
		return InStatus
	}
	if bytes.Contains(body, []byte(expect)) {
		return InBody
	}
	for _, h := range headers {
		if transportHeaders[strings.ToLower(h.Name)] {
			continue
		}
		if strings.Contains(h.Name+": "+h.Value, expect) {
			return InHeader
		}
	}
	return ""
}

// transportHeaders carry protocol data, not the API's answer (A13).
var transportHeaders = map[string]bool{
	"date": true, "content-length": true, "content-type": true, "server": true, "connection": true,
	"etag": true, "last-modified": true, "expires": true, "cache-control": true, "vary": true,
	"transfer-encoding": true, "keep-alive": true, "age": true,
}

// Judge sets the status and reason of a case that got a response.
func Judge(res *CaseResult, headers []curl.Header, body []byte) {
	res.FoundIn = Classify(res.Expect, res.HTTPStatus, headers, body)
	// The code is shown as typed: Go quoting would turn "code":"00" into "\"code\":\"00\"".
	if res.FoundIn != "" {
		res.Status = Success
		res.Reason = fmt.Sprintf("Found %s in the %s (HTTP %d)", res.Expect, res.FoundIn, res.HTTPStatus)
		return
	}
	res.Status = Failed
	res.Reason = fmt.Sprintf("%s not found in the response (HTTP %d)", res.Expect, res.HTTPStatus)
}
