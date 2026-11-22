package issue_test

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/actions/issue-token/internal/issue"
	"github.com/tmknom/gates/actions/issue-token/internal/request"
)

func maskLines(s string) []string {
	var masks []string
	for _, line := range lines(s) {
		if strings.HasPrefix(line, "::add-mask::") {
			masks = append(masks, line)
		}
	}
	return masks
}

//trace:checks AC-QUALITY-002
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
