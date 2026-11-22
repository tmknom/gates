package audit_test

import (
	"bytes"
	"encoding/json/v2"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/audit"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/claimsset"
	"github.com/tmknom/gates/internal/errorcode"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/revocation"
)

const (
	requestID      = "c0ffee00-0000-4000-8000-000000000001"
	ownerID        = 583231
	repositoryID   = 1234567890
	runID          = 4004
	runAttempt     = 2
	installationID = 12345678
	// installationOwnerID は ownerID と異なる値にし、取り違えを検出できるようにする
	installationOwnerID = 777

	// oidcJWTMarker は、発行要求と同じ Payload に入れた GATES 用 OIDC JWT で、監査記録に現れないことを確かめる目印である。
	oidcJWTMarker = "id-token-marker.payload.signature"
	// callerRepositoryNameMarker は、Caller のリポジトリ名で、監査記録に現れないことを確かめる目印である（Lambda Design の「Audit Record」）
	callerRepositoryNameMarker = "caller-repository-name-marker"
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
		{name: "failure records error code", result: audit.Failed(errorcode.AuthorizationDenied, "reason"), wantResult: "authorization_denied"},
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
	tests := []struct {
		name   string
		reason string
	}{
		{name: "failure records its reason", reason: "caller owner id does not match"},
		{name: "failure with an empty reason records the reason key", reason: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := write(t, audit.NewRecord(requestID), audit.Failed(errorcode.AuthorizationDenied, tt.reason))
			if reason, ok := got["reason"]; !ok || reason != tt.reason {
				t.Errorf("reason = %v (present %t), want %q", reason, ok, tt.reason)
			}
		})
	}
}

// TestRecord_Write_TopLevelKeys は、詰めなかった項目のキーが出ないことと、トップレベルのキーが slog のキーと Lambda Design の監査記録のキーに限られることを確かめる。
func TestRecord_Write_TopLevelKeys(t *testing.T) {
	failure := []string{"time", "level", "msg", "request_id", "result", "reason"}
	stagesPassed := func(t *testing.T) audit.Record {
		t.Helper()
		r := audit.NewRecord(requestID)
		r.SetTokenRequest(tokenRequest(t))
		r.SetClaimsSet(signatureVerifiedClaimsSet(t, `{}`))
		r.SetCaller(verifiedCaller(t, caller.WorkflowTriggerAttributes{}))
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
			result: audit.Failed(errorcode.InvalidRequest, "reason"),
			want:   failure,
		},
		{
			name: "authentication failure before signature verification records request without claims set or caller",
			build: func(t *testing.T) audit.Record {
				r := audit.NewRecord(requestID)
				r.SetTokenRequest(tokenRequest(t))
				return r
			},
			result: audit.Failed(errorcode.InvalidIDToken, "reason"),
			want:   append(slices.Clone(failure), "request"),
		},
		{
			name: "authentication failure after signature verification records claims set without caller",
			build: func(t *testing.T) audit.Record {
				r := audit.NewRecord(requestID)
				r.SetTokenRequest(tokenRequest(t))
				r.SetClaimsSet(signatureVerifiedClaimsSet(t, `{}`))
				return r
			},
			result: audit.Failed(errorcode.InvalidIDToken, "reason"),
			want:   append(slices.Clone(failure), "request", "claims_set"),
		},
		{
			name: "failure before installation resolution records no installation",
			build: func(t *testing.T) audit.Record {
				r := audit.NewRecord(requestID)
				r.SetTokenRequest(tokenRequest(t))
				r.SetClaimsSet(signatureVerifiedClaimsSet(t, `{}`))
				r.SetCaller(verifiedCaller(t, caller.WorkflowTriggerAttributes{}))
				return r
			},
			result: audit.Failed(errorcode.AuthorizationDenied, "reason"),
			want:   append(slices.Clone(failure), "request", "claims_set", "caller"),
		},
		{
			name:   "failure after installation resolution records installation without revocation",
			build:  stagesPassed,
			result: audit.Failed(errorcode.GitHubIssuanceRejected, "reason"),
			want:   append(slices.Clone(failure), "request", "claims_set", "caller", "installation"),
		},
		{
			name:   "verification failure records revocation",
			build:  stagesPassed,
			result: audit.VerificationFailed(errorcode.IssuanceVerificationFailed, "reason", revocation.Succeeded),
			want:   append(slices.Clone(failure), "request", "claims_set", "caller", "installation", "revocation"),
		},
		{
			name:   "success records neither reason nor revocation",
			build:  stagesPassed,
			result: audit.Succeeded(),
			want:   []string{"time", "level", "msg", "request_id", "result", "request", "claims_set", "caller", "installation"},
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

//trace:checks AC-OPS-004
func TestRecord_Write_Caller(t *testing.T) {
	all := caller.WorkflowTriggerAttributes{
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
		attributes caller.WorkflowTriggerAttributes
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
			attributes: caller.WorkflowTriggerAttributes{Actor: all.Actor, EventName: all.EventName},
			want:       merge(ids, map[string]any{"actor": all.Actor, "event_name": all.EventName}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.NewRecord(requestID)
			r.SetCaller(verifiedCaller(t, tt.attributes))
			line, got := write(t, r, audit.Succeeded())
			if !reflect.DeepEqual(got["caller"], tt.want) {
				t.Errorf("caller = %v, want %v", got["caller"], tt.want)
			}
			if strings.Contains(line, callerRepositoryNameMarker) {
				t.Errorf("audit record contains the caller repository name: %s", line)
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

// TestRecord_Write_ClaimsSet は、署名検証済みの Claims Set を、未知の Claim を含めて claims_set キーのオブジェクトに書くことを確かめる。再符号化の規則は claimsset のテストに任せる。
func TestRecord_Write_ClaimsSet(t *testing.T) {
	r := audit.NewRecord(requestID)
	r.SetClaimsSet(signatureVerifiedClaimsSet(t, `{"head_ref":"feature","repo_property_tier":"gold","unknown":{"nested":[1,true,null]}}`))
	_, got := write(t, r, audit.Failed(errorcode.InvalidIDToken, "reason"))
	want := map[string]any{
		"head_ref":           "feature",
		"repo_property_tier": "gold",
		"unknown":            map[string]any{"nested": []any{float64(1), true, nil}},
	}
	if !reflect.DeepEqual(got["claims_set"], want) {
		t.Errorf("claims_set = %v, want %v", got["claims_set"], want)
	}
}

// TestRecord_Write_UntrustedClaim は、利用者が選んだ文字列を含む Claim が、記録の行を分けたり別のキーを作ったりしないことを確かめる。
//
//trace:checks AC-OPS-009
func TestRecord_Write_UntrustedClaim(t *testing.T) {
	value := "line1\nline2\r\"quoted\"\x00\x01\x1f {\"injected\":true},\"result\":\"success\"}"
	payload, err := json.Marshal(map[string]string{"head_ref": value})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	r := audit.NewRecord(requestID)
	r.SetClaimsSet(signatureVerifiedClaimsSet(t, string(payload)))
	line, got := write(t, r, audit.Failed(errorcode.InvalidIDToken, "reason"))
	if n := strings.Count(line, "\n"); n != 1 || !strings.HasSuffix(line, "\n") {
		t.Errorf("record has %d newlines, want exactly one at the end: %q", n, line)
	}
	recorded, ok := got["claims_set"].(map[string]any)
	if !ok {
		t.Fatalf("claims_set = %v, want an object", got["claims_set"])
	}
	if recorded["head_ref"] != value {
		t.Errorf("claims_set.head_ref = %q, want %q", recorded["head_ref"], value)
	}
	if got["result"] != "invalid_id_token" {
		t.Errorf("result = %v, want %q", got["result"], "invalid_id_token")
	}
}

func TestRecord_SetClaimsSet_ZeroPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("SetClaimsSet() with the zero value did not panic")
		}
	}()
	r := audit.NewRecord(requestID)
	r.SetClaimsSet(claimsset.SignatureVerified{})
}

// TestRecord_Write_RevocationOutcome は、失効の結果を revocation キーに書くことを確かめる。値ごとの表記は revocation のテストに任せる。
func TestRecord_Write_RevocationOutcome(t *testing.T) {
	result := audit.VerificationFailed(errorcode.IssuanceVerificationFailed, "reason", revocation.NotAttempted)
	_, got := write(t, audit.NewRecord(requestID), result)
	if got["revocation"] != "not_attempted" {
		t.Errorf("revocation = %v, want %q", got["revocation"], "not_attempted")
	}
}

// TestRecord_Write_UntrustedTriggerAttribute は、解釈も検証もしないワークフロー起動属性が、記録の行を分けたり別のキーを作ったりしないことを確かめる。
func TestRecord_Write_UntrustedTriggerAttribute(t *testing.T) {
	value := "line1\nline2\r\"quoted\"\x00\x01\x1f {\"injected\":true},\"result\":\"success\"}"
	r := audit.NewRecord(requestID)
	r.SetCaller(verifiedCaller(t, caller.WorkflowTriggerAttributes{Actor: value}))
	line, got := write(t, r, audit.Failed(errorcode.InternalError, "reason"))
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

//trace:checks AC-QUALITY-002
func TestRecord_Write_ExcludesOIDCJWT(t *testing.T) {
	r := audit.NewRecord(requestID)
	r.SetTokenRequest(tokenRequest(t))
	line, _ := write(t, r, audit.Succeeded())
	if strings.Contains(line, oidcJWTMarker) {
		t.Errorf("record contains the OIDC JWT: %s", line)
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

func verifiedCaller(t *testing.T, attributes caller.WorkflowTriggerAttributes) caller.Caller {
	t.Helper()
	c, err := caller.NewCaller(ownerID, repositoryID, callerRepositoryNameMarker, runID, runAttempt, attributes)
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	return c
}

// tokenRequest は、要求の表記と並びが保たれることを確かめるため、大文字小文字を混ぜ、辞書順でない並びの要求を作る。
func tokenRequest(t *testing.T) request.TokenRequest {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"version":      1,
		"id_token":     oidcJWTMarker,
		"repositories": []string{"allowed-owner/repo-b", "Allowed-Owner/Example-Repo"},
		"permissions":  map[string]string{"contents": "read"},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	_, req, err := request.Validate(payload)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return req
}

func signatureVerifiedClaimsSet(t *testing.T, payload string) claimsset.SignatureVerified {
	t.Helper()
	c, err := claimsset.NewSignatureVerified([]byte(payload))
	if err != nil {
		t.Fatalf("NewSignatureVerified returned error: %v", err)
	}
	return c
}

func installation(t *testing.T) authz.Installation {
	t.Helper()
	i, err := authz.NewInstallation(installationID, installationOwnerID, false)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	return i
}

// write は、Lambda Design の監査記録と同じく JSON Handler に書き、出力の行と、読み戻したレコードを返す。
func write(t *testing.T, record audit.Record, result audit.Result) (string, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	record.Write(slog.New(slog.NewJSONHandler(&buf, nil)), result)
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v; output = %s", err, buf.String())
	}
	return buf.String(), got
}
