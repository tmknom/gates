package issue_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tmknom/gates/actions/issue-token/internal/issue"
)

// cancelOn は、line の 1 行が書かれたときに cancel を呼ぶ。書き込みの時点でキャンセルの要求を出すために使う。
func cancelOn(line string, cancel context.CancelFunc) func(string) error {
	return func(p string) error {
		if p == line+"\n" {
			cancel()
		}
		return nil
	}
}

const canceled = "the job was canceled"

//trace:checks AC-ISSUANCE-009
func TestRun_CancellationRequest(t *testing.T) {
	t.Parallel()

	token, _ := expectedSuccess(t)
	tests := map[string]struct {
		setup          func(t *testing.T, cancel context.CancelFunc) *fixture
		wantInvokes    int
		wantRevocation bool
	}{
		"during AWS STS cuts the communication": {
			setup: func(t *testing.T, cancel context.CancelFunc) *fixture {
				h := defaultHandlers(t)
				h.sts = func(_ http.ResponseWriter, r *http.Request) {
					cancel()
					<-r.Context().Done()
				}
				return newFixture(t, h)
			},
		},
		"after the OIDC JWT for GATES does not invoke": {
			setup: func(t *testing.T, cancel context.CancelFunc) *fixture {
				f := newFixture(t, defaultHandlers(t))
				f.stdout.onWrite = cancelOn("::add-mask::"+testGATESJWT, cancel)
				return f
			},
		},
		"during Invoke waits for the result and revokes": {
			setup: func(t *testing.T, cancel context.CancelFunc) *fixture {
				h := defaultHandlers(t)
				success := readTestdata(t, "success.json")
				h.lambda = func(w http.ResponseWriter, r *http.Request) {
					cancel()
					// Invoke が要求で打ち切られたなら、応答を書かずに終える
					select {
					case <-r.Context().Done():
						return
					case <-time.After(200 * time.Millisecond):
					}
					_, _ = w.Write(success)
				}
				return newFixture(t, h)
			},
			wantInvokes:    1,
			wantRevocation: true,
		},
		"after the token is masked revokes": {
			setup: func(t *testing.T, cancel context.CancelFunc) *fixture {
				f := newFixture(t, defaultHandlers(t))
				f.stdout.onWrite = cancelOn("::add-mask::"+token, cancel)
				return f
			},
			wantInvokes:    1,
			wantRevocation: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			f := tt.setup(t, cancel)

			err := issue.Run(ctx, f.cfg)
			if err == nil || err.Error() != canceled {
				t.Fatalf("Run() error = %v, want %q", err, canceled)
			}
			if got := lastLine(f.stdout.String()); got != "::error::"+canceled {
				t.Errorf("last stdout line = %q, want the cancellation annotation", got)
			}
			if n := len(f.lambda.received()); n != tt.wantInvokes {
				t.Errorf("Invoke requests = %d, want %d", n, tt.wantInvokes)
			}
			revocations := f.github.received()
			if tt.wantRevocation != (len(revocations) == 1) || len(revocations) > 1 {
				t.Errorf("revocation requests = %d, want revocation %v", len(revocations), tt.wantRevocation)
			}
			if tt.wantRevocation && len(revocations) == 1 && revocations[0].header.Get("Authorization") != "Bearer "+token {
				t.Errorf("revocation Authorization = %q, want the extracted token", revocations[0].header.Get("Authorization"))
			}
			if state, output := readFile(t, f.cfg.StatePath), readFile(t, f.cfg.OutputPath); state != "" || output != "" {
				t.Errorf("GITHUB_STATE = %q, GITHUB_OUTPUT = %q, want both empty", state, output)
			}
		})
	}
}

func TestRun_CancellationRequestAfterTheCheckCompletesTheRun(t *testing.T) {
	t.Parallel()

	token, items := expectedSuccess(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f := newFixture(t, defaultHandlers(t))
	// 確認より後に観測できる書き込みは、最後の成功のログだけである
	f.stdout.onWrite = cancelOn("Issued the token: request_id="+items.RequestID+" expires_at="+items.ExpiresAt+
		" repositories="+items.Repositories+" permissions="+items.Permissions, cancel)

	if err := issue.Run(ctx, f.cfg); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if ctx.Err() == nil {
		t.Fatal("the cancellation request was not made, want it at the success log")
	}
	if n := len(f.github.received()); n != 0 {
		t.Errorf("revocation requests = %d, want 0", n)
	}
	if got, want := readFile(t, f.cfg.StatePath), "token="+token+"\n"; got != want {
		t.Errorf("GITHUB_STATE = %q, want %q", got, want)
	}
	if readFile(t, f.cfg.OutputPath) == "" {
		t.Error("GITHUB_OUTPUT is empty, want the outputs")
	}
}
