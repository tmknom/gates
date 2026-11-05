package audit_test

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/audit"
	"github.com/tmknom/gates/internal/authz"
)

func TestRecord_Write_SlogKeys(t *testing.T) {
	_, got := write(t, audit.NewRecord(requestID), audit.Succeeded())
	if got["msg"] != "audit record" {
		t.Errorf("msg = %v, want %q", got["msg"], "audit record")
	}
	if got["level"] != "INFO" {
		t.Errorf("level = %v, want %q", got["level"], "INFO")
	}
	if got["request_id"] != requestID {
		t.Errorf("request_id = %v, want %q", got["request_id"], requestID)
	}
}

func TestRecord_Write_Result(t *testing.T) {
	tests := []struct {
		name       string
		result     audit.Result
		wantResult string
	}{
		{name: "success records success", result: audit.Succeeded(), wantResult: "success"},
		{name: "failure records error code", result: audit.Failed("authorization_denied", "reason"), wantResult: "authorization_denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := write(t, audit.NewRecord(requestID), tt.result)
			if got["result"] != tt.wantResult {
				t.Errorf("result = %v, want %q", got["result"], tt.wantResult)
			}
		})
	}
}

func TestRecord_Write_Reason(t *testing.T) {
	const reason = "caller owner id does not match"
	_, got := write(t, audit.NewRecord(requestID), audit.Failed("authorization_denied", reason))
	if got["reason"] != reason {
		t.Errorf("reason = %v, want %q", got["reason"], reason)
	}
}

// TestRecord_Write_TopLevelKeys は、詰めなかった項目のキーが出ないことと、トップレベルのキーが slog のキーと Lambda Design の監査記録のキーに限られることを確かめる。
func TestRecord_Write_TopLevelKeys(t *testing.T) {
	failure := []string{"time", "level", "msg", "request_id", "result", "reason"}
	stagesPassed := func(t *testing.T) audit.Record {
		t.Helper()
		r := audit.NewRecord(requestID)
		r.SetTokenRequest(tokenRequest(t))
		r.SetCaller(caller(t, authz.WorkflowTriggerAttributes{}))
		r.SetInstallation(installation(t))
		return r
	}
	tests := []struct {
		name   string
		build  func(t *testing.T) audit.Record
		result audit.Result
		want   []string
	}{
		{
			name:   "invalid request records neither caller nor request",
			build:  func(*testing.T) audit.Record { return audit.NewRecord(requestID) },
			result: audit.Failed("invalid_request", "reason"),
			want:   failure,
		},
		{
			name: "authentication failure records request without caller",
			build: func(t *testing.T) audit.Record {
				r := audit.NewRecord(requestID)
				r.SetTokenRequest(tokenRequest(t))
				return r
			},
			result: audit.Failed("invalid_id_token", "reason"),
			want:   append(slices.Clone(failure), "request"),
		},
		{
			name: "failure before installation resolution records no installation",
			build: func(t *testing.T) audit.Record {
				r := audit.NewRecord(requestID)
				r.SetTokenRequest(tokenRequest(t))
				r.SetCaller(caller(t, authz.WorkflowTriggerAttributes{}))
				return r
			},
			result: audit.Failed("authorization_denied", "reason"),
			want:   append(slices.Clone(failure), "request", "caller"),
		},
		{
			name:   "failure after installation resolution records installation without revocation",
			build:  stagesPassed,
			result: audit.Failed("github_issuance_rejected", "reason"),
			want:   append(slices.Clone(failure), "request", "caller", "installation"),
		},
		{
			name:   "verification failure records revocation",
			build:  stagesPassed,
			result: audit.VerificationFailed("issuance_verification_failed", "reason", audit.RevocationSucceeded),
			want:   append(slices.Clone(failure), "request", "caller", "installation", "revocation"),
		},
		{
			name:   "success records neither reason nor revocation",
			build:  stagesPassed,
			result: audit.Succeeded(),
			want:   []string{"time", "level", "msg", "request_id", "result", "request", "caller", "installation"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := write(t, tt.build(t), tt.result)
			keys := slices.Sorted(maps.Keys(got))
			if want := slices.Sorted(slices.Values(tt.want)); !slices.Equal(keys, want) {
				t.Errorf("top-level keys = %v, want %v", keys, want)
			}
		})
	}
}

func TestRecord_Write_Caller(t *testing.T) {
	all := authz.WorkflowTriggerAttributes{
		Actor:          "octocat",
		ActorID:        "1001",
		EventName:      "pull_request_target",
		Ref:            "refs/heads/main",
		SHA:            "0123456789abcdef0123456789abcdef01234567",
		WorkflowRef:    "allowed-owner/repo/.github/workflows/ci.yml@refs/heads/main",
		JobWorkflowRef: "allowed-owner/shared/.github/workflows/job.yml@refs/heads/main",
		Environment:    "production",
	}
	ids := map[string]any{
		"owner_id":      float64(ownerID),
		"repository_id": float64(repositoryID),
		"run_id":        float64(runID),
		"run_attempt":   float64(runAttempt),
	}
	tests := []struct {
		name       string
		attributes authz.WorkflowTriggerAttributes
		want       map[string]any
	}{
		{
			name:       "all trigger attributes are recorded under their claim names",
			attributes: all,
			want: merge(ids, map[string]any{
				"actor":            all.Actor,
				"actor_id":         all.ActorID,
				"event_name":       all.EventName,
				"ref":              all.Ref,
				"sha":              all.SHA,
				"workflow_ref":     all.WorkflowRef,
				"job_workflow_ref": all.JobWorkflowRef,
				"environment":      all.Environment,
			}),
		},
		{
			name:       "empty trigger attributes are omitted",
			attributes: authz.WorkflowTriggerAttributes{Actor: all.Actor, EventName: all.EventName},
			want:       merge(ids, map[string]any{"actor": all.Actor, "event_name": all.EventName}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.NewRecord(requestID)
			r.SetCaller(caller(t, tt.attributes))
			_, got := write(t, r, audit.Succeeded())
			if !reflect.DeepEqual(got["caller"], tt.want) {
				t.Errorf("caller = %v, want %v", got["caller"], tt.want)
			}
		})
	}
}

func TestRecord_Write_TokenRequest(t *testing.T) {
	r := audit.NewRecord(requestID)
	r.SetTokenRequest(tokenRequest(t))
	_, got := write(t, r, audit.Succeeded())
	want := map[string]any{
		"repositories": []any{"allowed-owner/repo-b", "Allowed-Owner/Example-Repo"},
		"permissions":  map[string]any{"contents": "read"},
	}
	if !reflect.DeepEqual(got["request"], want) {
		t.Errorf("request = %v, want %v", got["request"], want)
	}
}

func TestRecord_Write_Installation(t *testing.T) {
	r := audit.NewRecord(requestID)
	r.SetInstallation(installation(t))
	_, got := write(t, r, audit.Succeeded())
	want := map[string]any{"id": float64(installationID), "owner_id": float64(installationOwnerID)}
	if !reflect.DeepEqual(got["installation"], want) {
		t.Errorf("installation = %v, want %v", got["installation"], want)
	}
}

func TestRecord_Write_RevocationOutcome(t *testing.T) {
	tests := []struct {
		name    string
		outcome audit.RevocationOutcome
		want    string
	}{
		{name: "not attempted records not_attempted", outcome: audit.RevocationNotAttempted, want: "not_attempted"},
		{name: "succeeded records succeeded", outcome: audit.RevocationSucceeded, want: "succeeded"},
		{name: "failed records failed", outcome: audit.RevocationFailed, want: "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := audit.VerificationFailed("issuance_verification_failed", "reason", tt.outcome)
			_, got := write(t, audit.NewRecord(requestID), result)
			if got["revocation"] != tt.want {
				t.Errorf("revocation = %v, want %q", got["revocation"], tt.want)
			}
		})
	}
}

// TestRecord_Write_UntrustedTriggerAttribute は、解釈も検証もしないワークフロー起動属性が、記録の行を分けたり別のキーを作ったりしないことを確かめる。
func TestRecord_Write_UntrustedTriggerAttribute(t *testing.T) {
	value := "line1\nline2\r\"quoted\"\x00\x01\x1f {\"injected\":true},\"result\":\"success\"}"
	r := audit.NewRecord(requestID)
	r.SetCaller(caller(t, authz.WorkflowTriggerAttributes{Actor: value}))
	line, got := write(t, r, audit.Failed("internal_error", "reason"))
	if n := strings.Count(line, "\n"); n != 1 || !strings.HasSuffix(line, "\n") {
		t.Errorf("record has %d newlines, want exactly one at the end: %q", n, line)
	}
	recorded, ok := got["caller"].(map[string]any)
	if !ok {
		t.Fatalf("caller = %v, want an object", got["caller"])
	}
	if recorded["actor"] != value {
		t.Errorf("caller.actor = %q, want %q", recorded["actor"], value)
	}
	if got["result"] != "internal_error" {
		t.Errorf("result = %v, want %q", got["result"], "internal_error")
	}
}

func TestRecord_Write_ExcludesIDToken(t *testing.T) {
	r := audit.NewRecord(requestID)
	r.SetTokenRequest(tokenRequest(t))
	line, _ := write(t, r, audit.Succeeded())
	if strings.Contains(line, idTokenMarker) {
		t.Errorf("record contains the ID token: %s", line)
	}
}

func TestRecord_Write_ZeroResultPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Write() with the zero Result did not panic")
		}
	}()
	write(t, audit.NewRecord(requestID), audit.Result{})
}

func merge(a, b map[string]any) map[string]any {
	m := maps.Clone(a)
	maps.Copy(m, b)
	return m
}
