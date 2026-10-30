package authz

import "fmt"

// WorkflowTriggerAttributes は、ワークフロー実行の起動と、GATES 用 OIDC JWT を取得したジョブを示す Claim の組である。
// 解釈しない理由は Domain Model の「Request and Decision」に記述する。actor_id を含めて Claim の文字列のまま持ち、空文字列で欠落を表す。
type WorkflowTriggerAttributes struct {
	Actor          string
	ActorID        string
	EventName      string
	Ref            string
	SHA            string
	WorkflowRef    string
	JobWorkflowRef string
	Environment    string
}

// Caller は、検証済みの GATES 用 OIDC JWT から確定した呼び出し元である。
// フィールドを非公開にし、ゼロ値以外は NewCaller でしか作れないようにする。
type Caller struct {
	ownerID           int64
	repositoryID      int64
	runID             int64
	runAttempt        int64
	triggerAttributes WorkflowTriggerAttributes
}

// NewCaller は、Domain Model の「Invariants」が定める Caller の条件のうち、値だけで判定できるものを検証する。
// 欠落した Claim を 0 として受け取っても Caller にしないため。ワークフロー起動属性はベストエフォートのため検証しない。
func NewCaller(ownerID, repositoryID, runID, runAttempt int64, triggerAttributes WorkflowTriggerAttributes) (Caller, error) {
	if ownerID <= 0 {
		return Caller{}, fmt.Errorf("%w: owner id must be a positive integer", ErrInvalidCaller)
	}
	if repositoryID <= 0 {
		return Caller{}, fmt.Errorf("%w: repository id must be a positive integer", ErrInvalidCaller)
	}
	if runID <= 0 {
		return Caller{}, fmt.Errorf("%w: run_id must be a positive integer", ErrInvalidCaller)
	}
	if runAttempt <= 0 {
		return Caller{}, fmt.Errorf("%w: run_attempt must be a positive integer", ErrInvalidCaller)
	}
	return Caller{
		ownerID:           ownerID,
		repositoryID:      repositoryID,
		runID:             runID,
		runAttempt:        runAttempt,
		triggerAttributes: triggerAttributes,
	}, nil
}

// OwnerID は、不変 Owner ID を返す。
func (c Caller) OwnerID() int64 {
	return c.ownerID
}

// RepositoryID は、不変 Repository ID を返す。
func (c Caller) RepositoryID() int64 {
	return c.repositoryID
}

// RunID は、run_id を返す。
func (c Caller) RunID() int64 {
	return c.runID
}

// RunAttempt は、run_attempt を返す。
func (c Caller) RunAttempt() int64 {
	return c.runAttempt
}

// WorkflowTriggerAttributes は、ワークフロー起動属性を返す。
func (c Caller) WorkflowTriggerAttributes() WorkflowTriggerAttributes {
	return c.triggerAttributes
}
