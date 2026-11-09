package oidc_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/oidc"
)

const (
	audience = "https://gates.example/audience"
	jwt      = "header.payload.signature"
)

func respond(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

// countRequests は、handler が受けた要求の数を数える。自動で送り直さないことを確かめるため。
func countRequests(handler http.Handler) (http.Handler, *atomic.Int32) {
	var count atomic.Int32
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		handler.ServeHTTP(w, r)
	}), &count
}

func TestRetrieve_ReturnsIDTokenValue(t *testing.T) {
	t.Parallel()

	t.Run("200 with value returns the JWT", func(t *testing.T) {
		t.Parallel()

		client := newClient(t, respond(http.StatusOK, `{"count":1,"value":"`+jwt+`"}`), "/token?api-version=2.0")

		got, err := client.Retrieve(t.Context(), audience)
		if err != nil {
			t.Fatalf("Retrieve() error = %v", err)
		}
		if got.Expose() != jwt {
			t.Errorf("Retrieve() = %q, want %q", got.Expose(), jwt)
		}
	})
}

func TestRetrieve_RequestShape(t *testing.T) {
	t.Parallel()

	var got *http.Request
	client := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = w.Write([]byte(`{"value":"` + jwt + `"}`))
	}), "/token?api-version=2.0")

	if _, err := client.Retrieve(t.Context(), audience); err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if got.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", got.Method)
	}
	wantURI := "/token?api-version=2.0&audience=https%3A%2F%2Fgates.example%2Faudience"
	if got.URL.RequestURI() != wantURI {
		t.Errorf("request URI = %q, want %q", got.URL.RequestURI(), wantURI)
	}
	if v := got.Header.Get("Authorization"); v != "Bearer "+requestToken {
		t.Errorf("Authorization = %q, want Bearer %s", v, requestToken)
	}
	if v := got.Header.Get("Accept"); v != "application/json" {
		t.Errorf("Accept = %q, want application/json", v)
	}
	if strings.Contains(got.URL.String(), requestToken) {
		t.Errorf("request URL %q contains the request token", got.URL)
	}
}

func TestRetrieve_URLWithoutQuery(t *testing.T) {
	t.Parallel()

	var got string
	client := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"value":"` + jwt + `"}`))
	}), "/token")

	if _, err := client.Retrieve(t.Context(), audience); err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	want := "/token?audience=https%3A%2F%2Fgates.example%2Faudience"
	if got != want {
		t.Errorf("request URI = %q, want %q", got, want)
	}
}

func TestRetrieve_StatusFailure(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 4096) + "tail-not-shown"
	const redirectTarget = "https://redirect.example/token"

	tests := map[string]struct {
		handler http.Handler
		want    []string
		notWant []string
	}{
		"400 includes the status and the body": {
			handler: respond(http.StatusBadRequest, "bad audience"),
			want:    []string{"400", "bad audience"},
		},
		"500 truncates the body at 4096 bytes": {
			handler: respond(http.StatusInternalServerError, long),
			want:    []string{"500", strings.Repeat("x", 4096)},
			notWant: []string{"tail-not-shown"},
		},
		"204 includes only the status": {
			handler: respond(http.StatusNoContent, jwt),
			want:    []string{"204"},
			notWant: []string{jwt},
		},
		"302 is not followed and includes only the status": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, redirectTarget, http.StatusFound)
			}),
			want:    []string{"302"},
			notWant: []string{"Found", redirectTarget},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, requests := countRequests(tt.handler)
			client := newClient(t, handler, "/token?api-version=2.0")

			_, err := client.Retrieve(t.Context(), audience)
			if err == nil {
				t.Fatal("Retrieve() error = nil")
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("requests = %d, want 1", got)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(err.Error(), w) {
					t.Errorf("error %q contains %q", err, w)
				}
			}
		})
	}
}

func TestRetrieve_InvalidValue(t *testing.T) {
	t.Parallel()

	// 本文の値は文言に出してはならないため、どのケースにも sensitive を入れる
	const sensitive = "sensitive-value"
	tests := map[string]string{
		"value with LF":       `{"value":"` + sensitive + `\n` + sensitive + `"}`,
		"value with CR":       `{"value":"` + sensitive + `\r` + sensitive + `"}`,
		"empty value":         `{"value":"","other":"` + sensitive + `"}`,
		"number value":        `{"value":123,"other":"` + sensitive + `"}`,
		"two JSON objects":    `{"value":"` + sensitive + `"}{"value":"` + sensitive + `"}`,
		"not JSON":            sensitive,
		"JSON array":          `["` + sensitive + `"]`,
		"null":                `null`,
		"invalid UTF-8 value": `{"value":"` + sensitive + "\xff" + `"}`,
	}
	for name, body := range tests {
		t.Run(name+" returns an error without the body", func(t *testing.T) {
			t.Parallel()

			client := newClient(t, respond(http.StatusOK, body), "/token?api-version=2.0")

			_, err := client.Retrieve(t.Context(), audience)
			if err == nil {
				t.Fatal("Retrieve() error = nil")
			}
			if !strings.Contains(err.Error(), "200") {
				t.Errorf("error %q does not contain the status 200", err)
			}
			if strings.Contains(err.Error(), sensitive) {
				t.Errorf("error %q contains the body", err)
			}
		})
	}
}

func TestRetrieve_BodySizeLimit(t *testing.T) {
	t.Parallel()

	const limit = 64 * 1024
	padded := func(size int) string {
		body := `{"value":"` + jwt + `"}`
		return body + strings.Repeat(" ", size-len(body))
	}

	t.Run("body over 64 KiB is rejected without the body", func(t *testing.T) {
		t.Parallel()

		client := newClient(t, respond(http.StatusOK, padded(limit+1)), "/token?api-version=2.0")

		_, err := client.Retrieve(t.Context(), audience)
		if err == nil {
			t.Fatal("Retrieve() error = nil")
		}
		if !strings.Contains(err.Error(), "200") {
			t.Errorf("error %q does not contain the status 200", err)
		}
		if strings.Contains(err.Error(), jwt) {
			t.Errorf("error %q contains the body", err)
		}
	})

	t.Run("error status with body over 64 KiB is rejected without the body", func(t *testing.T) {
		t.Parallel()

		client := newClient(t, respond(http.StatusBadRequest, "short-failure-text"+strings.Repeat(" ", limit)), "/token?api-version=2.0")

		_, err := client.Retrieve(t.Context(), audience)
		if err == nil {
			t.Fatal("Retrieve() error = nil")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("error %q does not contain the status 400", err)
		}
		if strings.Contains(err.Error(), "short-failure-text") {
			t.Errorf("error %q contains the body", err)
		}
	})

	t.Run("body of exactly 64 KiB is read", func(t *testing.T) {
		t.Parallel()

		client := newClient(t, respond(http.StatusOK, padded(limit)), "/token?api-version=2.0")

		got, err := client.Retrieve(t.Context(), audience)
		if err != nil {
			t.Fatalf("Retrieve() error = %v", err)
		}
		if got.Expose() != jwt {
			t.Errorf("Retrieve() = %q, want %q", got.Expose(), jwt)
		}
	})
}

func TestRetrieve_Timeout(t *testing.T) {
	t.Parallel()

	client := newClient(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}), "/token?api-version=2.0")
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Retrieve(ctx, audience)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(%v, context.DeadlineExceeded) = false", err)
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("errors.As(%v, net.Error) with Timeout() = false", err)
	}
	if want := "the request timed out"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

// deadlineRecorder は、Transport が受けた要求の ctx の期限を記録する。
type deadlineRecorder struct {
	next     http.RoundTripper
	deadline time.Time
	ok       bool
}

func (d *deadlineRecorder) RoundTrip(r *http.Request) (*http.Response, error) {
	d.deadline, d.ok = r.Context().Deadline()
	return d.next.RoundTrip(r)
}

func TestRetrieve_SetsRetrievalTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(respond(http.StatusOK, `{"value":"`+jwt+`"}`))
	t.Cleanup(server.Close)
	recorder := &deadlineRecorder{next: server.Client().Transport}
	client, err := oidc.NewClient(server.URL+"/token?api-version=2.0", credential.NewOIDCRequestToken(requestToken), oidc.WithTransport(recorder))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	start := time.Now()
	if _, err := client.Retrieve(t.Context(), audience); err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if !recorder.ok {
		t.Fatal("the request has no deadline")
	}
	if got := recorder.deadline.Sub(start); got < 29*time.Second || got > 31*time.Second {
		t.Errorf("deadline is %v after the start, want about 30s", got)
	}
}

func TestRetrieve_CommunicationFailure(t *testing.T) {
	t.Parallel()

	tests := map[string]http.Handler{
		"connection closed before the response": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			_ = conn.Close()
		}),
		"connection closed while reading the body": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write([]byte(`{"value":"`))
		}),
	}
	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, requests := countRequests(handler)
			client := newClient(t, handler, "/token?api-version=2.0")

			_, err := client.Retrieve(t.Context(), audience)
			if err == nil {
				t.Fatal("Retrieve() error = nil")
			}
			if got := requests.Load(); got != 1 {
				t.Errorf("requests = %d, want 1", got)
			}
			if want := "the request failed without a response"; err.Error() != want {
				t.Errorf("error = %q, want %q", err, want)
			}
			if errors.Unwrap(err) == nil {
				t.Errorf("error %q does not wrap the cause", err)
			}
		})
	}
}
