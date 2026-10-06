package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fast keeps retry tests quick.
var fast = []string{"--interval", "1ms", "--timeout", "200ms"}

func runSmoke(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(append(append([]string{}, fast...), args...), &out, &errb)
	return code, out.String(), errb.String()
}

func TestPasses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"answer":"appeals#0"}`)
	}))
	defer srv.Close()

	code, out, _ := runSmoke("--url", srv.URL, "--expect-contains", "appeals#0")
	if code != 0 || !strings.Contains(out, "ok:") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, "hello")
	}))
	defer srv.Close()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"wrong status", []string{"--url", srv.URL + "/missing", "--retries", "2"}, "status 404, want 200"},
		{"body mismatch", []string{"--url", srv.URL, "--expect-contains", "nope", "--retries", "2"}, `does not contain "nope"`},
		{"connection refused", []string{"--url", "http://127.0.0.1:1", "--retries", "2"}, "FAIL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, errOut := runSmoke(tt.args...)
			if code != 1 || !strings.Contains(errOut, tt.want) {
				t.Fatalf("code=%d stderr=%q, want exit 1 containing %q", code, errOut, tt.want)
			}
		})
	}
}

func TestRetriesUntilServiceIsUp(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, "ready")
	}))
	defer srv.Close()

	code, out, _ := runSmoke("--url", srv.URL, "--retries", "5")
	if code != 0 || calls.Load() != 3 || !strings.Contains(out, "attempt 3/5") {
		t.Fatalf("code=%d calls=%d out=%q", code, calls.Load(), out)
	}
}

func TestSlowResponseTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()

	code, _, _ := runSmoke("--url", srv.URL, "--retries", "1")
	if code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
}

func TestSendsBodyAndHeaders(t *testing.T) {
	var gotMethod, gotAuth, gotType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotMethod, gotAuth, gotType, gotBody = r.Method, r.Header.Get("Authorization"), r.Header.Get("Content-Type"), string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	code, _, errOut := runSmoke("--url", srv.URL, "--body", `{"q":1}`,
		"--header", "Authorization: Bearer abc", "--expect-status", "201")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if gotMethod != "POST" || gotAuth != "Bearer abc" || gotType != "application/json" || gotBody != `{"q":1}` {
		t.Fatalf("method=%q auth=%q type=%q body=%q", gotMethod, gotAuth, gotType, gotBody)
	}
}

func TestUsageErrors(t *testing.T) {
	tests := map[string][]string{
		"no url":           {},
		"zero retries":     {"--url", "http://x", "--retries", "0"},
		"malformed header": {"--url", "http://x", "--header", "nocolon"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			if code, _, _ := runSmoke(args...); code != 2 {
				t.Fatalf("code=%d, want 2", code)
			}
		})
	}
}
