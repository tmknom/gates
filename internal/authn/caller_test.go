package authn

import (
	"encoding/json/v2"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
)

const maxInt64 = "9223372036854775807"

// validClaims は、実 JWT と同じく Caller の確定に読まない Claim も含む、正当な Claim の組を返す。
func validClaims() map[string]any {
	return map[string]any{
		"iss":                 "https://token.actions.githubusercontent.com",
		"aud":                 "gates",
		"exp":                 1700000300,
		"iat":                 1700000000,
		"nbf":                 1699999700,
		"jti":                 "example-jti",
		"sub":                 "repo:octo@11/app@22:ref:refs/heads/main",
		"repository_owner_id": "11",
		"repository_id":       "22",
		"run_id":              "33",
		"run_attempt":         "2",
		"actor":               "octocat",
		"actor_id":            "583231",
		"event_name":          "push",
		"ref":                 "refs/heads/main",
		"sha":                 "ffac537e6cbbf934b08745a378932722df287a53",
		"workflow_ref":        "octo/app/.github/workflows/ci.yml@refs/heads/main",
		"job_workflow_ref":    "octo/app/.github/workflows/job.yml@refs/heads/main",
		"environment":         "production",
	}
}

func validAttributes() authz.WorkflowTriggerAttributes {
	return authz.WorkflowTriggerAttributes{
		Actor:          "octocat",
		ActorID:        "583231",
		EventName:      "push",
		Ref:            "refs/heads/main",
		SHA:            "ffac537e6cbbf934b08745a378932722df287a53",
		WorkflowRef:    "octo/app/.github/workflows/ci.yml@refs/heads/main",
		JobWorkflowRef: "octo/app/.github/workflows/job.yml@refs/heads/main",
		Environment:    "production",
	}
}

func marshalClaims(t *testing.T, claims map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return b
}

func claimsWith(overrides map[string]any) map[string]any {
	claims := validClaims()
	maps.Copy(claims, overrides)
	return claims
}

func claimsWithout(names ...string) map[string]any {
	claims := validClaims()
	for _, name := range names {
		delete(claims, name)
	}
	return claims
}

func newCaller(t *testing.T, ownerID, repositoryID, runID, runAttempt int64, attributes authz.WorkflowTriggerAttributes) authz.Caller {
	t.Helper()
	caller, err := authz.NewCaller(ownerID, repositoryID, runID, runAttempt, attributes)
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	return caller
}

func TestCallerFromClaims(t *testing.T) {
	defaultCaller := newCaller(t, 11, 22, 33, 2, validAttributes())
	withAttributes := func(change func(*authz.WorkflowTriggerAttributes)) authz.Caller {
		attributes := validAttributes()
		change(&attributes)
		return newCaller(t, 11, 22, 33, 2, attributes)
	}
	tests := []struct {
		name   string
		claims map[string]any
		want   authz.Caller
	}{
		{
			name:   "valid claims return caller",
			claims: validClaims(),
			want:   defaultCaller,
		},
		{
			name:   "sub with environment returns caller",
			claims: claimsWith(map[string]any{"sub": "repo:octo@11/app@22:environment:production"}),
			want:   defaultCaller,
		},
		{
			name:   "sub with pull_request returns caller",
			claims: claimsWith(map[string]any{"sub": "repo:octo@11/app@22:pull_request"}),
			want:   defaultCaller,
		},
		{
			name:   "sub with nothing after colon returns caller",
			claims: claimsWith(map[string]any{"sub": "repo:octo@11/app@22:"}),
			want:   defaultCaller,
		},
		{
			name:   "names in sub are not compared",
			claims: claimsWith(map[string]any{"sub": "repo:other@11/another@22:ref:refs/heads/main"}),
			want:   defaultCaller,
		},
		{
			name: "all workflow trigger attributes missing return caller",
			claims: claimsWithout("actor", "actor_id", "event_name", "ref", "sha",
				"workflow_ref", "job_workflow_ref", "environment"),
			want: newCaller(t, 11, 22, 33, 2, authz.WorkflowTriggerAttributes{}),
		},
		{
			name:   "null workflow trigger attribute is treated as missing",
			claims: claimsWith(map[string]any{"environment": nil}),
			want:   withAttributes(func(a *authz.WorkflowTriggerAttributes) { a.Environment = "" }),
		},
		{
			name:   "workflow trigger attribute is kept without interpretation",
			claims: claimsWith(map[string]any{"environment": "line1\nline2\"quoted\""}),
			want:   withAttributes(func(a *authz.WorkflowTriggerAttributes) { a.Environment = "line1\nline2\"quoted\"" }),
		},
		{
			name: "ids at the int64 limit return caller",
			claims: claimsWith(map[string]any{
				"sub":                 "repo:octo@" + maxInt64 + "/app@" + maxInt64 + ":pull_request",
				"repository_owner_id": maxInt64,
				"repository_id":       maxInt64,
				"run_id":              maxInt64,
				"run_attempt":         maxInt64,
			}),
			want: newCaller(t, 1<<63-1, 1<<63-1, 1<<63-1, 1<<63-1, validAttributes()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := callerFromClaims(marshalClaims(t, tt.claims))
			if err != nil {
				t.Fatalf("callerFromClaims returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("callerFromClaims() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCallerFromClaims_InvalidClaims(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]any
	}{
		{name: "missing sub returns ErrAuthenticationFailure", claims: claimsWithout("sub")},
		{name: "name only sub returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo/app:ref:refs/heads/main"})},
		{name: "sub without repo prefix returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repository:octo@11/app@22:ref:refs/heads/main"})},
		{name: "sub without slash returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@11:pull_request"})},
		{name: "sub without colon after repository part returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@11/app@22"})},
		{name: "sub with empty owner name returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:@11/app@22:pull_request"})},
		{name: "sub with empty owner id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@/app@22:pull_request"})},
		{name: "sub with empty repository id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@11/app@:pull_request"})},
		{name: "sub with two at signs in owner part returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@9@11/app@22:pull_request"})},
		{name: "sub with non numeric owner id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@abc/app@22:pull_request"})},
		{name: "sub with leading zero owner id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@011/app@22:pull_request"})},
		{name: "sub owner id different from repository_owner_id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@99/app@22:pull_request"})},
		{name: "sub repository id different from repository_id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@11/app@99:pull_request"})},
		{name: "missing repository_owner_id returns ErrAuthenticationFailure", claims: claimsWithout("repository_owner_id")},
		{name: "missing repository_id returns ErrAuthenticationFailure", claims: claimsWithout("repository_id")},
		{name: "missing run_id returns ErrAuthenticationFailure", claims: claimsWithout("run_id")},
		{name: "missing run_attempt returns ErrAuthenticationFailure", claims: claimsWithout("run_attempt")},
		{name: "empty id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": ""})},
		{name: "negative id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": "-1"})},
		{name: "signed id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": "+11"})},
		{name: "leading zero id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": "011"})},
		{name: "hexadecimal id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": "0x1F"})},
		{name: "exponent id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": "1e3"})},
		{name: "full width id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": "１１"})},
		{name: "id over int64 returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": "9223372036854775808"})},
		{name: "number id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_owner_id": 11})},
		{name: "number repository_id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"repository_id": 22})},
		{name: "number run_id returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"run_id": 33})},
		{name: "number run_attempt returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"run_attempt": 2})},
		{name: "zero id matching sub returns ErrAuthenticationFailure", claims: claimsWith(map[string]any{"sub": "repo:octo@0/app@22:pull_request", "repository_owner_id": "0"})},
		{name: "upper case claim name is not read and returns ErrAuthenticationFailure", claims: map[string]any{
			"sub": "repo:octo@11/app@22:pull_request", "REPOSITORY_OWNER_ID": "11", "repository_id": "22", "run_id": "33", "run_attempt": "2",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := callerFromClaims(marshalClaims(t, tt.claims))
			if !errors.Is(err, ErrAuthenticationFailure) {
				t.Fatalf("callerFromClaims() error = %v, want %v", err, ErrAuthenticationFailure)
			}
			for name, value := range tt.claims {
				if s, ok := value.(string); ok && s != "" && strings.Contains(err.Error(), s) {
					t.Errorf("error %q contains the value of claim %s", err, name)
				}
			}
			for _, fragment := range []string{"octo", "app@", "abc"} {
				if strings.Contains(err.Error(), fragment) {
					t.Errorf("error %q contains the fragment %q of sub", err, fragment)
				}
			}
		})
	}
}

// validClaimsAnd は、正当な Claim の組の末尾に JSON のメンバーを足す。復号以外の規則で拒否されない状態で、復号の規則を確かめるため。
func validClaimsAnd(t *testing.T, members string) string {
	t.Helper()
	return strings.TrimSuffix(string(marshalClaims(t, validClaims())), "}") + "," + members + "}"
}

func TestCallerFromClaims_AttributeOutOfNumberRangeIsTreatedAsMissing(t *testing.T) {
	attributes := validAttributes()
	attributes.Environment = ""
	want := newCaller(t, 11, 22, 33, 2, attributes)
	raw := strings.Replace(string(marshalClaims(t, validClaims())), `"environment":"production"`, `"environment":1e999`, 1)
	got, err := callerFromClaims([]byte(raw))
	if err != nil {
		t.Fatalf("callerFromClaims returned error: %v", err)
	}
	if got != want {
		t.Errorf("callerFromClaims() = %+v, want %+v", got, want)
	}
}

func TestCallerFromClaims_UndecodableClaims(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "array returns ErrAuthenticationFailure", raw: `[]`},
		{name: "malformed json returns ErrAuthenticationFailure", raw: `{"sub":`},
		{name: "invalid utf-8 in an unread claim returns ErrAuthenticationFailure", raw: validClaimsAnd(t, "\"other\":\"\xff\"")},
		{name: "duplicate id claim returns ErrAuthenticationFailure", raw: validClaimsAnd(t, `"repository_owner_id":"11"`)},
		{name: "duplicate unread claim returns ErrAuthenticationFailure", raw: validClaimsAnd(t, `"secret":1,"secret":2`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := callerFromClaims([]byte(tt.raw))
			if !errors.Is(err, ErrAuthenticationFailure) {
				t.Fatalf("callerFromClaims() error = %v, want %v", err, ErrAuthenticationFailure)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error %q contains a fragment of the claims", err)
			}
		})
	}
}
