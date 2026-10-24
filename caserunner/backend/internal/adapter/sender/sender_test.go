package sender

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/domain/curl"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

func TestSendRequestAndResponse(t *testing.T) {
	var got *http.Request
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Add("X-B", "2")
		w.Header().Add("X-A", "1")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"code":"00"}`))
	}))
	defer srv.Close()

	resp, err := New().Send(context.Background(), usecase.OutboundRequest{
		Method: "PUT",
		URL:    srv.URL + "/x?y=1",
		Headers: []curl.Header{
			{Name: "Content-Type", Value: "application/json"},
			{Name: "X-Dup", Value: "a"}, {Name: "X-Dup", Value: "b"},
			{Name: "Content-Length", Value: "999"}, // must not be sent as is
			{Name: "Host", Value: "api.example.com"},
		},
		Body: `{"a":1}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != "PUT" || got.URL.RequestURI() != "/x?y=1" || gotBody != `{"a":1}` || got.ContentLength != 7 {
		t.Errorf("request: %s %s body=%q len=%d", got.Method, got.URL, gotBody, got.ContentLength)
	}
	if got.Host != "api.example.com" || strings.Join(got.Header.Values("X-Dup"), ",") != "a,b" {
		t.Errorf("headers: host=%q X-Dup=%q", got.Host, got.Header.Values("X-Dup"))
	}
	if resp.Status != 201 || string(resp.Body) != `{"code":"00"}` || resp.Truncated {
		t.Errorf("response: %d %q truncated=%v", resp.Status, resp.Body, resp.Truncated)
	}
	if resp.Headers[0].Name > resp.Headers[1].Name {
		t.Errorf("headers should be sorted by name: %+v", resp.Headers)
	}
}

func TestSendDoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/next" {
			t.Error("the redirect was followed")
		}
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer srv.Close()
	resp, err := New().Send(context.Background(), usecase.OutboundRequest{Method: "POST", URL: srv.URL})
	if err != nil || resp.Status != http.StatusFound {
		t.Fatalf("got %d, %v; want 302", resp.Status, err)
	}
}

// Chrome copies "accept-encoding: gzip, deflate, br"; sending it as is would
// return a body the client does not decompress.
func TestSendReadsGzipBodiesAsText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			_, _ = w.Write([]byte("plain"))
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte(`{"token":"zipped"}`))
		_ = gz.Close()
	}))
	defer srv.Close()
	resp, err := New().Send(context.Background(), usecase.OutboundRequest{
		Method: "GET", URL: srv.URL, Headers: []curl.Header{{Name: "accept-encoding", Value: "gzip, deflate, br"}},
	})
	if err != nil || string(resp.Body) != `{"token":"zipped"}` {
		t.Fatalf("body = %q, %v", resp.Body, err)
	}
}

func TestSendCapsTheBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", MaxBody+10)))
	}))
	defer srv.Close()
	resp, err := New().Send(context.Background(), usecase.OutboundRequest{Method: "GET", URL: srv.URL})
	if err != nil || len(resp.Body) != MaxBody || !resp.Truncated {
		t.Fatalf("len=%d truncated=%v err=%v", len(resp.Body), resp.Truncated, err)
	}
}

func TestSendErrors(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer tlsSrv.Close()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closedURL := "http://" + ln.Addr().String()
	ln.Close()

	tests := []struct {
		name string
		url  string
		ctx  func() (context.Context, context.CancelFunc)
		want string
	}{
		{"timeout", slow.URL, func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 50*time.Millisecond)
		}, usecase.ErrTimeout},
		{"connection refused", closedURL, nil, usecase.ErrConnection},
		{"unknown host", "http://no-such-host.invalid", nil, usecase.ErrDNS},
		{"untrusted certificate", tlsSrv.URL, nil, usecase.ErrTLS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.Background(), func() {}
			if tt.ctx != nil {
				ctx, cancel = tt.ctx()
			}
			defer cancel()
			_, err := New().Send(ctx, usecase.OutboundRequest{Method: "GET", URL: tt.url})
			var se *usecase.SendError
			if !errors.As(err, &se) || se.Type != tt.want {
				t.Errorf("err = %v, want type %s", err, tt.want)
			}
		})
	}
}

func TestSendInsecureSkipsVerification(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	defer srv.Close()
	resp, err := New().Send(context.Background(), usecase.OutboundRequest{Method: "GET", URL: srv.URL, Insecure: true})
	if err != nil || resp.Status != 204 {
		t.Fatalf("got %d, %v", resp.Status, err)
	}
}
