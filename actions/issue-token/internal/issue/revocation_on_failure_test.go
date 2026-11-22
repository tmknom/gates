package issue_test

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmknom/gates/actions/issue-token/internal/issue"
)

//trace:checks AC-OPS-002
func TestRun_InvokeFailureWritesNothingAndDoesNotRevoke(t *testing.T) {
	t.Parallel()

	h := defaultHandlers(t)
	h.lambda = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Amzn-Errortype", "AccessDeniedException")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"Message":"denied"}`))
	}
	f := newFixture(t, h)

	err := issue.Run(t.Context(), f.cfg)
	if err == nil || !strings.Contains(err.Error(), "AccessDeniedException") {
		t.Fatalf("Run() error = %v, want the AWS error code of the Invoke", err)
	}
	if state, output := readFile(t, f.cfg.StatePath), readFile(t, f.cfg.OutputPath); state != "" || output != "" {
		t.Errorf("GITHUB_STATE = %q, GITHUB_OUTPUT = %q, want both empty", state, output)
	}
	if n := len(f.github.received()); n != 0 {
		t.Errorf("revocation requests = %d, want 0 because no token was extracted", n)
	}
}

//trace:checks AC-ISSUANCE-008
func TestRun_FailureAfterExtractingTokenRevokesIt(t *testing.T) {
	t.Parallel()

	token, _ := expectedSuccess(t)
	missingPath := func(path string) string { return filepath.Join(filepath.Dir(path), "missing") }
	tests := map[string]struct {
		lambdaPayload string
		configure     func(*issue.Config)
		wantErr       string // 空なら文言を確かめない（response の文言である）
		wantState     string
	}{
		"missing expires_at": {
			lambdaPayload: "success_expires_at_missing.json",
		},
		"unwritable GITHUB_STATE": {
			configure: func(cfg *issue.Config) { cfg.StatePath = missingPath(cfg.StatePath) },
			wantErr:   "could not write to GITHUB_STATE",
		},
		"unwritable GITHUB_OUTPUT": {
			configure: func(cfg *issue.Config) { cfg.OutputPath = missingPath(cfg.OutputPath) },
			wantErr:   "could not write to GITHUB_OUTPUT",
			wantState: "token=" + token + "\n",
		},
	}
	for name, tt := range tests {
		t.Run(name+" revokes the token and fails", func(t *testing.T) {
			t.Parallel()

			h := defaultHandlers(t)
			if tt.lambdaPayload != "" {
				h.lambda = lambdaReturns(readTestdata(t, tt.lambdaPayload))
			}
			f := newFixture(t, h)
			if tt.configure != nil {
				tt.configure(&f.cfg)
			}

			err := issue.Run(t.Context(), f.cfg)
			if err == nil || (tt.wantErr != "" && err.Error() != tt.wantErr) {
				t.Fatalf("Run() error = %v, want %q", err, tt.wantErr)
			}
			revocations := f.github.received()
			if len(revocations) != 1 || revocations[0].method != http.MethodDelete || revocations[0].header.Get("Authorization") != "Bearer "+token {
				t.Errorf("revocation requests = %+v, want one DELETE with the extracted token", revocations)
			}
			if got := readFileIfExists(t, f.cfg.StatePath); got != tt.wantState {
				t.Errorf("GITHUB_STATE = %q, want %q", got, tt.wantState)
			}
			if got := readFileIfExists(t, f.cfg.OutputPath); got != "" {
				t.Errorf("GITHUB_OUTPUT = %q, want empty", got)
			}
		})
	}
}

func TestRun_JoinsTheFailureToWriteTheRevocationResult(t *testing.T) {
	t.Parallel()

	h := defaultHandlers(t)
	h.lambda = lambdaReturns(readTestdata(t, "success_expires_at_missing.json"))
	f := newFixture(t, h)
	f.stdout.onWrite = failOn("revocation ")

	err := issue.Run(t.Context(), f.cfg)
	if !errors.Is(err, errWrite) {
		t.Errorf("Run() error = %v, want it to include %v of the revocation result", err, errWrite)
	}
	if n := len(f.github.received()); n != 1 {
		t.Errorf("revocation requests = %d, want 1", n)
	}
}

// panicOn は、prefix で始まる行が書かれたときに panic する。段階の途中で不変条件が破れた状況を作るために使う。
func panicOn(prefix string) func(string) error {
	return func(p string) error {
		if strings.HasPrefix(p, prefix) {
			panic(errPanic) //nolint:forbidigo // 段階の途中の panic を、標準出力への書き込みで起こすため
		}
		return nil
	}
}

var errPanic = errors.New("panic in a stage")

//trace:checks AC-ISSUANCE-008
func TestRun_PanicAfterExtractingTokenRevokesItAndPassesThePanicOn(t *testing.T) {
	t.Parallel()

	token, _ := expectedSuccess(t)
	f := newFixture(t, defaultHandlers(t))
	f.stdout.onWrite = panicOn("::add-mask::" + token)

	recovered := func() (r any) {
		defer func() { r = recover() }()
		_ = issue.Run(t.Context(), f.cfg)
		return nil
	}()
	if err, _ := recovered.(error); !errors.Is(err, errPanic) {
		t.Errorf("recovered = %v, want %v", recovered, errPanic)
	}
	if n := len(f.github.received()); n != 1 {
		t.Errorf("revocation requests = %d, want 1", n)
	}
}

// readFileIfExists は、ファイルがなければ空を返す。書けないファイルの代わりに存在しないパスを使うため。
func readFileIfExists(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}
