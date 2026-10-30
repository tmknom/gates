package authn

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/tmknom/gates/internal/authz"
)

// claims は、GATES 用 OIDC JWT の Claim のうち、Caller の確定に読むものを復号する入れ物である。
// ID の Claim は、JSON の数値を受け付けないよう string で受ける。
// ワークフロー起動属性は、文字列でない値を欠落として扱うよう jsontext.Value で受ける。
type claims struct {
	Sub               string         `json:"sub"`
	RepositoryOwnerID string         `json:"repository_owner_id"`
	RepositoryID      string         `json:"repository_id"`
	RunID             string         `json:"run_id"`
	RunAttempt        string         `json:"run_attempt"`
	Actor             jsontext.Value `json:"actor"`
	ActorID           jsontext.Value `json:"actor_id"`
	EventName         jsontext.Value `json:"event_name"`
	Ref               jsontext.Value `json:"ref"`
	SHA               jsontext.Value `json:"sha"`
	WorkflowRef       jsontext.Value `json:"workflow_ref"`
	JobWorkflowRef    jsontext.Value `json:"job_workflow_ref"`
	Environment       jsontext.Value `json:"environment"`
}

// decodeClaims は、重複したキーと不正な UTF-8 を拒否し、Claim 名の大文字小文字を区別する encoding/json/v2 の既定で復号する。
// encoding/json/v2 のエラー文言は入力の断片を含むため包まない。
func decodeClaims(rawClaims []byte) (claims, error) {
	var c claims
	if err := json.Unmarshal(rawClaims, &c); err != nil {
		return claims{}, fmt.Errorf("%w: claims cannot be decoded", ErrAuthenticationFailure)
	}
	return c, nil
}

func (c claims) triggerAttributes() authz.WorkflowTriggerAttributes {
	return authz.WorkflowTriggerAttributes{
		Actor:          attributeValue(c.Actor),
		ActorID:        attributeValue(c.ActorID),
		EventName:      attributeValue(c.EventName),
		Ref:            attributeValue(c.Ref),
		SHA:            attributeValue(c.SHA),
		WorkflowRef:    attributeValue(c.WorkflowRef),
		JobWorkflowRef: attributeValue(c.JobWorkflowRef),
		Environment:    attributeValue(c.Environment),
	}
}

func attributeValue(claim jsontext.Value) string {
	var s string
	if err := json.Unmarshal(claim, &s); err != nil {
		return ""
	}
	return s
}
