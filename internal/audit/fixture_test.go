package audit_test

import (
	"bytes"
	"encoding/json/v2"
	"log/slog"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/audit"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/request"
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

	// idTokenMarker は、発行要求と同じ Payload に入れた GATES 用 OIDC JWT で、監査記録に現れないことを確かめる目印である。
	idTokenMarker = "id-token-marker.payload.signature"
)

func caller(t *testing.T, attributes authz.WorkflowTriggerAttributes) authz.Caller {
	t.Helper()
	c, err := authz.NewCaller(ownerID, repositoryID, runID, runAttempt, attributes)
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
		"id_token":     idTokenMarker,
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

func installation(t *testing.T) authz.Installation {
	t.Helper()
	i, err := authz.NewInstallation(installationID, installationOwnerID, time.Time{})
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
