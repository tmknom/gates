package github_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/github"
)

// revocationTimeout は Lambda Design の Call Time Budget が定める値である。
const revocationTimeout = 5 * time.Second

// issuedToken は、発行応答から読み取った Installation Access Token を返す。トークンは発行応答からしか作れない。
func issuedToken(t *testing.T) github.InstallationAccessToken {
	t.Helper()
	token, ok := issueBody(t, marshal(t, recordedIssuance())).Token()
	if !ok {
		t.Fatal("Token() is not read from the recorded issuance response")
	}
	return token
}

func respondRevocation(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", requestID)
		w.WriteHeader(status)
		if status != http.StatusNoContent {
			_, _ = io.WriteString(w, `{"message":"`+responseMarker+`"}`)
		}
	}
}

// sentRevocationRequest は、既定の API Host のまま Revoke を呼び、外へ送らずに受け取った要求を返す。
func sentRevocationRequest(t *testing.T) *http.Request {
	t.Helper()
	token := issuedToken(t)
	var sent *http.Request
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		sent = request
		return &http.Response{StatusCode: http.StatusNoContent, Header: http.Header{}, Body: http.NoBody}, nil
	})
	if err := github.NewClient(github.WithTransport(transport)).Revoke(t.Context(), token); err != nil {
		t.Fatalf("Revoke returned error: %v", err)
	}
	return sent
}

func TestRevoke_SendsTokenItselfAsCredential(t *testing.T) {
	sent := sentRevocationRequest(t)

	if sent.Method != http.MethodDelete || sent.URL.String() != "https://api.github.com/installation/token" {
		t.Errorf("request = %s %s, want DELETE https://api.github.com/installation/token", sent.Method, sent.URL)
	}
	wantHeader := map[string]string{
		"Authorization":        "Bearer " + tokenMarker,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2026-03-10",
		"User-Agent":           "gates",
	}
	for name, want := range wantHeader {
		if got := sent.Header.Values(name); len(got) != 1 || got[0] != want {
			t.Errorf("request header %s = %q, want [%q]", name, got, want)
		}
	}
	if sent.Body != nil && sent.Body != http.NoBody {
		t.Error("request has a body")
	}
}

func TestRevoke_WaitsAtMostRevocationTimeout(t *testing.T) {
	deadline, ok := sentRevocationRequest(t).Context().Deadline()
	if !ok {
		t.Fatal("request has no deadline")
	}
	if remaining := time.Until(deadline); remaining > revocationTimeout || remaining < revocationTimeout/2 {
		t.Errorf("remaining time until the deadline = %v, want about %v", remaining, revocationTimeout)
	}
}

func TestRevoke_Status(t *testing.T) {
	t.Run("status 204 returns nil", func(t *testing.T) {
		if err := newAPIStub(t, respondRevocation(http.StatusNoContent)).client(t).Revoke(t.Context(), issuedToken(t)); err != nil {
			t.Errorf("Revoke returned error: %v", err)
		}
	})
	// 失効の成否は 204 との一致だけで決まるため、成功に見える 200 で失敗の側を確かめる
	t.Run("status 200 returns error", func(t *testing.T) {
		err := newAPIStub(t, respondRevocation(http.StatusOK)).client(t).Revoke(t.Context(), issuedToken(t))

		assertRevocationFailed(t, err)
		for _, value := range []string{"status 200", labeledRequestID} {
			if !strings.Contains(err.Error(), value) {
				t.Errorf("error message %q does not contain %q", err.Error(), value)
			}
		}
	})
}

func TestRevoke_CommunicationFailureReturnsError(t *testing.T) {
	closeConnection := func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			panic(err)
		}
		_ = conn.Close()
	}

	err := newAPIStub(t, closeConnection).client(t).Revoke(t.Context(), issuedToken(t))

	assertRevocationFailed(t, err)
}

// assertRevocationFailed は、失効のエラー文言が失効の失敗だと分かる形で始まり、トークンと応答本文を含まないことを確かめる。
func assertRevocationFailed(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Revoke returned nil error")
	}
	if !strings.HasPrefix(err.Error(), "revocation ") {
		t.Errorf("error message %q does not start with %q", err.Error(), "revocation ")
	}
	for _, value := range []string{tokenMarker, responseMarker} {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err.Error(), value)
		}
	}
}

func TestRevoke_ZeroTokenPanicsWithoutRequest(t *testing.T) {
	stub := newAPIStub(t, respondRevocation(http.StatusNoContent))
	client := stub.client(t)
	defer func() {
		if recover() == nil {
			t.Error("Revoke did not panic for the zero InstallationAccessToken")
		}
		if stub.request.Load() != nil {
			t.Error("Revoke sent a request for the zero InstallationAccessToken")
		}
	}()

	_ = client.Revoke(t.Context(), github.InstallationAccessToken{})
}
