package revocation

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

type fakeRevoker struct {
	status int
	err    error
	got    credential.InstallationAccessToken
}

func (f *fakeRevoker) Revoke(_ context.Context, token credential.InstallationAccessToken) (int, error) {
	f.got = token
	return f.status, f.err
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

//trace:checks AC-31 AC-32
func TestAttempt(t *testing.T) {
	tests := []struct {
		name   string
		status int
		err    error
		want   string
	}{
		{
			name:   "204 writes an info line without a warning",
			status: 204,
			want:   "revocation succeeded: HTTP status 204\n",
		},
		{
			name:   "401 writes a warning with the status",
			status: 401,
			want:   "::warning::revocation failed: HTTP status 401\n",
		},
		{
			name: "client error writes a warning with the error message",
			err:  errors.New("cause"),
			want: "::warning::revocation failed: cause\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			client := &fakeRevoker{status: tt.status, err: tt.err}
			if err := Attempt(context.Background(), &buf, client, credential.NewInstallationAccessToken("ghs_x")); err != nil {
				t.Fatalf("Attempt() error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("Attempt() wrote %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("the given token reaches the client", func(t *testing.T) {
		client := &fakeRevoker{status: 204}
		token := credential.NewInstallationAccessToken("ghs_given")
		if err := Attempt(context.Background(), &bytes.Buffer{}, client, token); err != nil {
			t.Fatalf("Attempt() error = %v", err)
		}
		if client.got != token {
			t.Errorf("client received a different token")
		}
	})

	writeFailures := []struct {
		name   string
		status int
		err    error
	}{
		{"write failure of the success line is returned", 204, nil},
		{"write failure of the status warning is returned", 401, nil},
		{"write failure of the error warning is returned", 0, errors.New("cause")},
	}
	for _, tt := range writeFailures {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeRevoker{status: tt.status, err: tt.err}
			err := Attempt(context.Background(), failingWriter{}, client, credential.NewInstallationAccessToken("ghs_x"))
			if !errors.Is(err, errWrite) {
				t.Errorf("Attempt() error = %v, want %v", err, errWrite)
			}
		})
	}
}
