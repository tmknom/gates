package awsapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

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

// newRecordingServer は、body を返す TLS のサーバーと、要求の期限を記録する Option を作る。
func newRecordingServer(t *testing.T, body string) (*deadlineRecorder, []awsapi.Option) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	recorder := &deadlineRecorder{next: server.Client().Transport}
	return recorder, []awsapi.Option{awsapi.WithBaseEndpoint(server.URL), awsapi.WithTransport(recorder)}
}

func TestSTSClient_AssumeRoleWithWebIdentity_SetsTimeout(t *testing.T) {
	t.Parallel()

	recorder, opts := newRecordingServer(t, stsSuccessXML)
	client := awsapi.NewSTSClient(testRegion, opts...)

	start := time.Now()
	if _, err := client.AssumeRoleWithWebIdentity(t.Context(), "arn:aws:iam::123456789012:role/gates", credential.NewOIDCJWT("jwt")); err != nil {
		t.Fatalf("AssumeRoleWithWebIdentity() error = %v", err)
	}

	if !recorder.ok {
		t.Fatal("the request has no deadline")
	}
	if got := recorder.deadline.Sub(start); got < 29*time.Second || got > 31*time.Second {
		t.Errorf("deadline is %v after the start, want about 30s", got)
	}
}

func TestLambdaClient_Invoke_SetsTimeout(t *testing.T) {
	t.Parallel()

	recorder, opts := newRecordingServer(t, `{"ok":true}`)
	client := awsapi.NewLambdaClient(testRegion, testCredentials(), opts...)

	start := time.Now()
	if _, err := client.Invoke(t.Context(), "gates", []byte(`{}`)); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	if !recorder.ok {
		t.Fatal("the request has no deadline")
	}
	if got := recorder.deadline.Sub(start); got < 89*time.Second || got > 91*time.Second {
		t.Errorf("deadline is %v after the start, want about 90s", got)
	}
}
