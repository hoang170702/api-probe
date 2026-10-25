package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/broker"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/memstore"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/sender"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/system"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// upstream is a fake API: /ok answers code 00, /slow waits for release.
type upstream struct {
	srv     *httptest.Server
	hits    atomic.Int64
	release chan struct{}
}

func newUpstream(t *testing.T) *upstream {
	u := &upstream{release: make(chan struct{})}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path == "/slow" {
			select {
			case <-u.release:
			case <-r.Context().Done():
				return
			}
		}
		if strings.Contains(string(body), `"amount":0`) {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"code":"ERR_AMOUNT_INVALID"}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":"00"}`))
	}))
	t.Cleanup(u.srv.Close)
	return u
}

func runsServer(t *testing.T, maxRuns int) (*httptest.Server, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	out := sender.New()
	runs := &usecase.Runs{
		Repo: memstore.New(maxRuns, time.Hour, time.Now), Events: broker.New(), Sender: out,
		Policy: usecase.Policy{DefaultTimeout: 5 * time.Second, MaxTimeout: 5 * time.Second},
		Clock:  system.Clock{}, IDs: system.IDs{},
		MaxCases: 1000, MaxConcurrency: 20, DefaultConcurrency: 5, Base: ctx,
	}
	srv := httptest.NewServer(Guard(NewHandler(Info{}, UseCases{Runs: runs}, nil), nil))
	t.Cleanup(func() { cancel(); srv.Close() })
	return srv, cancel
}

func runBody(url string, cases ...string) string {
	return `{"template":{"method":"POST","url":"` + url + `","headers":[{"name":"Content-Type","value":"application/json"}],` +
		`"bodyRaw":"{\"data\":{\"amount\":100000,\"note\":\"x\"}}","bodyKind":"json"},` +
		`"fields":[{"path":"data.amount","type":"number","rule":"REQUIRED"},{"path":"data.note","type":"string","rule":"UNREQUIRED"}],` +
		`"cases":[` + strings.Join(cases, ",") + `],"options":{"concurrency":2,"timeoutMs":3000}}`
}

func post(t *testing.T, url, body string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

type sseEvent struct{ name, data string }

// readEvents reads SSE events until "done" or the stream ends.
func readEvents(t *testing.T, url string) []sseEvent {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" || resp.Header.Get("X-Accel-Buffering") != "no" {
		t.Errorf("headers: %v", resp.Header)
	}
	var evs []sseEvent
	var cur sseEvent
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		case line == "" && cur.name != "":
			evs = append(evs, cur)
			if cur.name == "done" {
				return evs
			}
			cur = sseEvent{}
		}
	}
	return evs
}

func TestRunOverHTTPWithSSE(t *testing.T) {
	api := newUpstream(t)
	srv, _ := runsServer(t, 10)
	resp, b := post(t, srv.URL+"/api/runs", runBody(api.srv.URL+"/ok",
		`{"no":1,"name":"ok","values":{"data.amount":"100000","data.note":"x"},"expect":"\"code\":\"00\""}`,
		`{"no":2,"name":"zero","values":{"data.amount":"0","data.note":""},"expect":"ERR_AMOUNT_INVALID"}`,
		`{"no":3,"name":"blocked","values":{"data.amount":"","data.note":"x"},"expect":"00"}`,
	))
	if resp.StatusCode != 202 {
		t.Fatalf("start = %d %s", resp.StatusCode, b)
	}
	var started startRunResponse
	_ = json.Unmarshal(b, &started)
	if started.Total != 3 || started.Blocked != 1 || !strings.HasPrefix(started.RunID, "r_") {
		t.Fatalf("started = %+v", started)
	}

	evs := readEvents(t, srv.URL+"/api/runs/"+started.RunID+"/events")
	if len(evs) < 2 || evs[0].name != "snapshot" || evs[len(evs)-1].name != "done" {
		t.Fatalf("events = %+v", evs)
	}
	var done doneDTO
	_ = json.Unmarshal([]byte(evs[len(evs)-1].data), &done)
	if done.Status != "DONE" || done.Summary.Success != 2 || done.Summary.Blocked != 1 {
		t.Errorf("done = %+v", done)
	}

	gr, err := http.Get(srv.URL + "/api/runs/" + started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Body.Close()
	var snap map[string]any
	_ = json.NewDecoder(gr.Body).Decode(&snap)
	results := snap["results"].([]any)
	second := results[1].(map[string]any)
	if second["status"] != "SUCCESS" || second["foundIn"] != "body" || second["httpStatus"].(float64) != 400 || second["caseNo"].(float64) != 2 {
		t.Errorf("case 2 = %v", second)
	}
	if !strings.Contains(second["requestBody"].(string), `"amount":0`) || strings.Contains(second["requestBody"].(string), "note") {
		t.Errorf("case 2 request body = %v", second["requestBody"])
	}
	third := results[2].(map[string]any)
	if third["status"] != "BLOCKED" || third["errors"].([]any)[0].(map[string]any)["code"] != "REQUIRED_MISSING" {
		t.Errorf("case 3 = %v", third)
	}
	if api.hits.Load() != 2 {
		t.Errorf("upstream hits = %d, want 2 (the BLOCKED case is never sent)", api.hits.Load())
	}
}

func TestEventsOfAFinishedRunAreSnapshotThenDone(t *testing.T) {
	api := newUpstream(t)
	srv, _ := runsServer(t, 10)
	_, b := post(t, srv.URL+"/api/runs", runBody(api.srv.URL+"/ok", `{"values":{"data.amount":"1"},"expect":"00"}`))
	var started startRunResponse
	_ = json.Unmarshal(b, &started)
	readEvents(t, srv.URL+"/api/runs/"+started.RunID+"/events") // wait for it to finish
	evs := readEvents(t, srv.URL+"/api/runs/"+started.RunID+"/events")
	if len(evs) != 2 || evs[0].name != "snapshot" || evs[1].name != "done" {
		t.Errorf("events = %+v", evs)
	}
}

func TestCancelOverHTTP(t *testing.T) {
	api := newUpstream(t)
	srv, _ := runsServer(t, 10)
	case1 := `{"values":{"data.amount":"1"},"expect":"00"}`
	_, b := post(t, srv.URL+"/api/runs", runBody(api.srv.URL+"/slow", case1, case1, case1, case1, case1))
	var started startRunResponse
	_ = json.Unmarshal(b, &started)
	for api.hits.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	if resp, _ := post(t, srv.URL+"/api/runs/"+started.RunID+"/cancel", "{}"); resp.StatusCode != 202 {
		t.Fatalf("cancel = %d", resp.StatusCode)
	}
	evs := readEvents(t, srv.URL+"/api/runs/"+started.RunID+"/events")
	var done doneDTO
	_ = json.Unmarshal([]byte(evs[len(evs)-1].data), &done)
	if done.Status != "CANCELLED" || done.Summary.Cancelled != 3 || done.Summary.Exception != 2 {
		t.Errorf("done = %+v", done)
	}
}

func TestRunErrors(t *testing.T) {
	api := newUpstream(t)
	srv, _ := runsServer(t, 1)
	ok := `{"values":{"data.amount":"1"},"expect":"00"}`

	if resp, b := post(t, srv.URL+"/api/runs", strings.Replace(runBody(api.srv.URL+"/ok", ok), `"rule":"REQUIRED"`, `"rule":"MAYBE"`, 1)); resp.StatusCode != 400 || !strings.Contains(string(b), "REQUIRED or UNREQUIRED") {
		t.Errorf("bad rule: %d %s", resp.StatusCode, b)
	}
	if resp, b := post(t, srv.URL+"/api/runs", runBody(api.srv.URL+"/ok")); resp.StatusCode != 400 || !strings.Contains(string(b), "between 1 and") {
		t.Errorf("no cases: %d %s", resp.StatusCode, b)
	}

	// The only slot is taken by a run that stays in progress.
	if resp, _ := post(t, srv.URL+"/api/runs", runBody(api.srv.URL+"/slow", ok)); resp.StatusCode != 202 {
		t.Fatalf("first run: %d", resp.StatusCode)
	}
	if resp, b := post(t, srv.URL+"/api/runs", runBody(api.srv.URL+"/ok", ok)); resp.StatusCode != 429 || !strings.Contains(string(b), "TOO_MANY_RUNS") {
		t.Errorf("full store: %d %s", resp.StatusCode, b)
	}
	close(api.release)

	for _, path := range []string{"/api/runs/r_nope", "/api/runs/r_nope/events"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 404 || !strings.Contains(string(b), "Run has expired, please run it again") {
			t.Errorf("GET %s = %d %s", path, resp.StatusCode, b)
		}
	}
	if resp, _ := post(t, srv.URL+"/api/runs/r_nope/cancel", "{}"); resp.StatusCode != 404 {
		t.Errorf("cancel unknown = %d", resp.StatusCode)
	}
	// The guard still applies to the new endpoints.
	resp, err := http.Post(srv.URL+"/api/runs", "text/plain", strings.NewReader(runBody(api.srv.URL+"/ok", ok)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 415 {
		t.Errorf("text/plain = %d, want 415", resp.StatusCode)
	}
}

func TestSSEPing(t *testing.T) {
	old := pingEvery
	pingEvery = 20 * time.Millisecond
	t.Cleanup(func() { pingEvery = old })
	api := newUpstream(t)
	srv, _ := runsServer(t, 10)
	_, b := post(t, srv.URL+"/api/runs", runBody(api.srv.URL+"/slow", `{"values":{"data.amount":"1"},"expect":"00"}`))
	var started startRunResponse
	_ = json.Unmarshal(b, &started)
	resp, err := http.Get(srv.URL + "/api/runs/" + started.RunID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if sc.Text() == ": ping" {
			close(api.release)
			return
		}
	}
	t.Error("no ping on an idle stream")
}
