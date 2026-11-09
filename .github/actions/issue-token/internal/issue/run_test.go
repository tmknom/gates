package issue_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/issue"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/request"
)

const canceled = "the job was canceled"

func lines(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// lastLine は、標準出力の最後の行である。失敗の注釈は最後に書かれる。
func lastLine(s string) string {
	all := lines(s)
	return all[len(all)-1]
}

func maskLines(s string) []string {
	var masks []string
	for _, line := range lines(s) {
		if strings.HasPrefix(line, "::add-mask::") {
			masks = append(masks, line)
		}
	}
	return masks
}

func TestRun_SuccessWritesStateOutputsAndLogsAfterMasking(t *testing.T) {
	t.Parallel()

	f := newFixture(t, defaultHandlers(t))
	token, items := expectedSuccess(t)

	if err := issue.Run(t.Context(), f.cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	stdout := f.stdout.String()
	wantMasks := []string{
		"::add-mask::" + testRequestToken,
		"::add-mask::" + testAWSJWT,
		"::add-mask::" + testAccessKeyID,
		"::add-mask::" + testSecretAccessKey,
		"::add-mask::" + testSessionToken,
		"::add-mask::" + testGATESJWT,
		"::add-mask::" + token,
	}
	if got := maskLines(stdout); !slices.Equal(got, wantMasks) {
		t.Errorf("mask lines = %q, want %q", got, wantMasks)
	}
	invoking := "Invoking the Lambda function at " + testNow.UTC().Format(time.RFC3339)
	issued := "Issued the token: request_id=" + items.RequestID + " expires_at=" + items.ExpiresAt +
		" repositories=" + items.Repositories + " permissions=" + items.Permissions
	all := lines(stdout)
	invokingAt, tokenMaskAt, issuedAt := slices.Index(all, invoking), slices.Index(all, wantMasks[6]), slices.Index(all, issued)
	if invokingAt < 0 || issuedAt < 0 || invokingAt >= tokenMaskAt || issuedAt != len(all)-1 {
		t.Errorf("stdout lines = %q, want %q before the mask of the token and %q at the end", all, invoking, issued)
	}

	if got, want := readFile(t, f.cfg.StatePath), "token="+token+"\n"; got != want {
		t.Errorf("GITHUB_STATE = %q, want %q", got, want)
	}
	wantOutput := "token=" + token + "\n" +
		"expires-at=" + items.ExpiresAt + "\n" +
		"repositories=" + items.Repositories + "\n" +
		"permissions=" + items.Permissions + "\n"
	if got := readFile(t, f.cfg.OutputPath); got != wantOutput {
		t.Errorf("GITHUB_OUTPUT = %q, want %q", got, wantOutput)
	}

	// OIDC は AWS 用と GATES 用の 1 回ずつ。どの要求も送り直さない
	counts := []int{len(f.oidc.received()), len(f.sts.received()), len(f.lambda.received()), len(f.github.received())}
	if want := []int{2, 1, 1, 0}; !slices.Equal(counts, want) {
		t.Errorf("requests to OIDC, STS, Lambda, GitHub = %v, want %v", counts, want)
	}
}

func TestRun_PassesValuesBetweenStages(t *testing.T) {
	t.Parallel()

	f := newFixture(t, defaultHandlers(t))

	if err := issue.Run(t.Context(), f.cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	oidcRequests, stsRequests, lambdaRequests := f.oidc.received(), f.sts.received(), f.lambda.received()
	if len(oidcRequests) != 2 || len(stsRequests) != 1 || len(lambdaRequests) != 1 {
		t.Fatalf("requests to OIDC, STS, Lambda = %d, %d, %d, want 2, 1, 1", len(oidcRequests), len(stsRequests), len(lambdaRequests))
	}
	var audiences []string
	for _, req := range oidcRequests {
		if got, want := req.header.Get("Authorization"), "Bearer "+testRequestToken; got != want {
			t.Errorf("OIDC Authorization = %q, want %q", got, want)
		}
		query, _ := url.ParseQuery(strings.SplitN(req.requestURI, "?", 2)[1])
		audiences = append(audiences, query.Get("audience"))
	}
	if want := []string{"sts.amazonaws.com", testAudience}; !slices.Equal(audiences, want) {
		t.Errorf("OIDC audiences = %q, want %q", audiences, want)
	}

	stsBody, _ := url.ParseQuery(stsRequests[0].body)
	if got := stsBody.Get("WebIdentityToken"); got != testAWSJWT {
		t.Errorf("STS WebIdentityToken = %q, want the OIDC JWT for AWS STS %q", got, testAWSJWT)
	}
	if got := stsBody.Get("RoleArn"); got != testRoleARN {
		t.Errorf("STS RoleArn = %q, want %q", got, testRoleARN)
	}

	invoke := lambdaRequests[0]
	if want := "/2015-03-31/functions/" + testFunctionName + "/invocations"; invoke.requestURI != want {
		t.Errorf("Lambda request URI = %q, want %q", invoke.requestURI, want)
	}
	if auth, want := invoke.header.Get("Authorization"), "Credential="+testAccessKeyID+"/"; !strings.Contains(auth, want) {
		t.Errorf("Lambda Authorization = %q, want it signed with the AWS credentials from STS", auth)
	}
	if auth, want := invoke.header.Get("Authorization"), "/"+testRegion+"/lambda/"; !strings.Contains(auth, want) {
		t.Errorf("Lambda Authorization = %q, want the region %q", auth, testRegion)
	}
	if got := invoke.header.Get("X-Amz-Security-Token"); got != testSessionToken {
		t.Errorf("Lambda X-Amz-Security-Token = %q, want %q", got, testSessionToken)
	}
	tokenRequest, err := request.Parse(testRepositories, testPermissions)
	if err != nil {
		t.Fatalf("request.Parse() error = %v", err)
	}
	if want := string(tokenRequest.Payload(credential.NewOIDCJWT(testGATESJWT))); invoke.body != want {
		t.Errorf("Lambda payload = %s, want %s", invoke.body, want)
	}
}

func TestRun_DoesNotOutputCredentials(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*handlers){
		"success":                        func(*handlers) {},
		"failure after extracting token": func(h *handlers) { h.lambda = lambdaReturns(readTestdata(t, "success_expires_at_missing.json")) },
		"failure at AWS STS":             func(h *handlers) { h.sts = stsReturns(http.StatusForbidden, stsAccessDeniedXML) },
	}
	for name, modify := range tests {
		t.Run(name+" shows credentials only in mask lines", func(t *testing.T) {
			t.Parallel()

			h := defaultHandlers(t)
			modify(&h)
			f := newFixture(t, h)
			token, _ := expectedSuccess(t)
			secrets := []string{testRequestToken, testAWSJWT, testAccessKeyID, testSecretAccessKey, testSessionToken, testGATESJWT, token}

			err := issue.Run(t.Context(), f.cfg)

			for _, line := range lines(f.stdout.String()) {
				for _, secret := range secrets {
					if strings.Contains(line, secret) && line != "::add-mask::"+secret {
						t.Errorf("stdout line %q contains a credential outside its mask", line)
					}
				}
			}
			for _, secret := range secrets {
				if err != nil && strings.Contains(err.Error(), secret) {
					t.Errorf("Run() error = %q contains the credential %q", err, secret)
				}
				for _, rec := range []*recorder{f.oidc, f.sts, f.lambda, f.github} {
					for _, req := range rec.received() {
						if strings.Contains(req.requestURI, secret) {
							t.Errorf("request URI %q contains the credential %q", req.requestURI, secret)
						}
					}
				}
			}
		})
	}
}

// noRequests は、どのサーバーにも要求が届いていないことを確かめる。
func noRequests(f *fixture) bool {
	return len(f.oidc.received())+len(f.sts.received())+len(f.lambda.received())+len(f.github.received()) == 0
}

func TestRun_MasksRequestTokenBeforeReportingMissingPrerequisite(t *testing.T) {
	t.Parallel()

	f := newFixture(t, defaultHandlers(t))
	f.cfg.Repositories = ""

	err := issue.Run(t.Context(), f.cfg)
	if err == nil || !strings.Contains(err.Error(), "input repositories is empty") {
		t.Fatalf("Run() error = %v, want the name of the empty input", err)
	}
	want := []string{"::add-mask::" + testRequestToken, "::error::" + err.Error()}
	if got := lines(f.stdout.String()); !slices.Equal(got, want) {
		t.Errorf("stdout lines = %q, want %q", got, want)
	}
	if !noRequests(f) {
		t.Error("a server received a request, want none before the prerequisites are met")
	}
}

func TestRun_EmptyRequestTokenIsNotMasked(t *testing.T) {
	t.Parallel()

	f := newFixture(t, defaultHandlers(t))
	f.cfg.OIDCRequestToken = ""

	err := issue.Run(t.Context(), f.cfg)
	if err == nil || !strings.Contains(err.Error(), "id-token: write") {
		t.Fatalf("Run() error = %v, want it to ask for id-token: write", err)
	}
	if want := []string{"::error::" + err.Error()}; !slices.Equal(lines(f.stdout.String()), want) {
		t.Errorf("stdout = %q, want only %q without a mask of the empty token", f.stdout.String(), want)
	}
	if !noRequests(f) {
		t.Error("a server received a request, want none before the prerequisites are met")
	}
}

func TestRun_InvalidInputsStopBeforeCommunicating(t *testing.T) {
	t.Parallel()

	f := newFixture(t, defaultHandlers(t))
	f.cfg.Repositories = "example-org/repo-a,,example-org"
	_, parseErr := request.Parse(f.cfg.Repositories, f.cfg.Permissions)

	err := issue.Run(t.Context(), f.cfg)
	if err == nil || !strings.HasPrefix(err.Error(), "invalid inputs:\n") || !strings.Contains(err.Error(), parseErr.Error()) {
		t.Fatalf("Run() error = %v, want invalid inputs with %q", err, parseErr)
	}
	if !noRequests(f) {
		t.Error("a server received a request, want none for invalid inputs")
	}
}

func TestRun_InvalidRequestURLStopsBeforeCommunicating(t *testing.T) {
	t.Parallel()

	f := newFixture(t, defaultHandlers(t))
	f.cfg.OIDCRequestURL = "http://token.example/token?api-version=2.0"

	err := issue.Run(t.Context(), f.cfg)
	if err == nil || !strings.HasPrefix(err.Error(), "invalid ACTIONS_ID_TOKEN_REQUEST_URL: ") {
		t.Fatalf("Run() error = %v, want the invalid request URL", err)
	}
	if strings.Contains(err.Error(), f.cfg.OIDCRequestURL) {
		t.Errorf("Run() error = %q, want it without the URL", err)
	}
	if !noRequests(f) {
		t.Error("a server received a request, want none for an invalid request URL")
	}
}

func TestRun_StageFailureBeforeInvokeStopsTheLaterStages(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		modify     func(*handlers)
		wantPrefix string
		wantCounts []int // OIDC、STS、Lambda、GitHub
	}{
		"OIDC JWT for AWS STS fails": {
			modify:     func(h *handlers) { h.oidc = oidcReturns(map[string]string{testAudience: testGATESJWT}) },
			wantPrefix: "could not get the OIDC JWT for AWS STS: ",
			wantCounts: []int{1, 0, 0, 0},
		},
		"AWS STS denies": {
			modify:     func(h *handlers) { h.sts = stsReturns(http.StatusForbidden, stsAccessDeniedXML) },
			wantPrefix: "AssumeRoleWithWebIdentity failed: ",
			wantCounts: []int{1, 1, 0, 0},
		},
		"OIDC JWT for GATES fails": {
			modify:     func(h *handlers) { h.oidc = oidcReturns(map[string]string{"sts.amazonaws.com": testAWSJWT}) },
			wantPrefix: "could not get the OIDC JWT for GATES: ",
			wantCounts: []int{2, 1, 0, 0},
		},
	}
	for name, tt := range tests {
		t.Run(name+" returns the stage and stops", func(t *testing.T) {
			t.Parallel()

			h := defaultHandlers(t)
			tt.modify(&h)
			f := newFixture(t, h)

			err := issue.Run(t.Context(), f.cfg)
			if err == nil || !strings.HasPrefix(err.Error(), tt.wantPrefix) {
				t.Fatalf("Run() error = %v, want prefix %q", err, tt.wantPrefix)
			}
			if got := lastLine(f.stdout.String()); got != "::error::"+err.Error() {
				t.Errorf("last stdout line = %q, want the error annotation", got)
			}
			counts := []int{len(f.oidc.received()), len(f.sts.received()), len(f.lambda.received()), len(f.github.received())}
			if !slices.Equal(counts, tt.wantCounts) {
				t.Errorf("requests to OIDC, STS, Lambda, GitHub = %v, want %v", counts, tt.wantCounts)
			}
		})
	}
}

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

// cancelOn は、line の 1 行が書かれたときに cancel を呼ぶ。書き込みの時点でキャンセルの要求を出すために使う。
func cancelOn(line string, cancel context.CancelFunc) func(string) error {
	return func(p string) error {
		if p == line+"\n" {
			cancel()
		}
		return nil
	}
}

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

var errWrite = errors.New("write failed")

// failOn は、prefix で始まる書き込みを失敗させる。
func failOn(prefix string) func(string) error {
	return func(p string) error {
		if strings.HasPrefix(p, prefix) {
			return errWrite
		}
		return nil
	}
}

func TestRun_StdoutWriteFailureStopsTheStage(t *testing.T) {
	t.Parallel()

	token, _ := expectedSuccess(t)
	tests := map[string]struct {
		prefix     string
		wantCounts []int // OIDC、STS、Lambda、GitHub
		wantState  string
	}{
		"mask of the request token stops before communicating": {
			prefix:     "::add-mask::" + testRequestToken,
			wantCounts: []int{0, 0, 0, 0},
		},
		"mask of the OIDC JWT for AWS STS stops before AWS STS": {
			prefix:     "::add-mask::" + testAWSJWT,
			wantCounts: []int{1, 0, 0, 0},
		},
		"mask of the AWS credentials stops before the OIDC JWT for GATES": {
			prefix:     "::add-mask::" + testSecretAccessKey,
			wantCounts: []int{1, 1, 0, 0},
		},
		"mask of the OIDC JWT for GATES stops before Invoke": {
			prefix:     "::add-mask::" + testGATESJWT,
			wantCounts: []int{2, 1, 0, 0},
		},
		"log of the Invoke time stops before Invoke": {
			prefix:     "Invoking ",
			wantCounts: []int{2, 1, 0, 0},
		},
		"mask of the token revokes it": {
			prefix:     "::add-mask::" + token,
			wantCounts: []int{2, 1, 1, 1},
		},
		"log of the issued token revokes it": {
			prefix:     "Issued ",
			wantCounts: []int{2, 1, 1, 1},
			wantState:  "token=" + token + "\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, defaultHandlers(t))
			f.stdout.onWrite = failOn(tt.prefix)

			err := issue.Run(t.Context(), f.cfg)
			if !errors.Is(err, errWrite) {
				t.Fatalf("Run() error = %v, want %v", err, errWrite)
			}
			counts := []int{len(f.oidc.received()), len(f.sts.received()), len(f.lambda.received()), len(f.github.received())}
			if !slices.Equal(counts, tt.wantCounts) {
				t.Errorf("requests to OIDC, STS, Lambda, GitHub = %v, want %v", counts, tt.wantCounts)
			}
			if got := readFile(t, f.cfg.StatePath); got != tt.wantState {
				t.Errorf("GITHUB_STATE = %q, want %q", got, tt.wantState)
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

func TestRun_JoinsTheFailureToWriteTheAnnotation(t *testing.T) {
	t.Parallel()

	f := newFixture(t, defaultHandlers(t))
	f.cfg.Repositories = ""
	f.stdout.onWrite = failOn("::error::")

	err := issue.Run(t.Context(), f.cfg)
	if !errors.Is(err, errWrite) || !strings.Contains(err.Error(), "input repositories is empty") {
		t.Errorf("Run() error = %v, want both the failure of the stage and %v", err, errWrite)
	}
}
