package issue_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/gates/actions/issue-token/internal/issue"
)

//trace:checks AC-OPS-002 AC-QUALITY-002
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
