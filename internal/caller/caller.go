// Package caller は、認証 Context が確定し、発行 Context に渡す Caller である。
// 認証から発行へ渡る値のため、作る側と読む側のどちらの package にも置かない。
package caller

import (
	"errors"
	"fmt"

	"github.com/tmknom/gates/internal/githubid"
)

// ErrInvalidCaller は、Caller の不変条件を満たさない値から Caller を作ろうとしたことを表す。
var ErrInvalidCaller = errors.New("invalid caller")

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
	ownerID           githubid.OwnerID
	repositoryID      githubid.RepositoryID
	runID             githubid.RunID
	runAttempt        githubid.RunAttempt
	triggerAttributes WorkflowTriggerAttributes
}

// NewCaller は、Domain Model の「Invariants」が定める Caller の条件のうち、値だけで判定できるものを検証する。
// 欠落した Claim を 0 として受け取っても Caller にしないため。ワークフロー起動属性はベストエフォートのため検証しない。
func NewCaller(ownerID, repositoryID, runID, runAttempt int64, triggerAttributes WorkflowTriggerAttributes) (Caller, error) {
	validOwnerID, err := githubid.NewOwnerID(ownerID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: owner id %w", ErrInvalidCaller, err)
	}
	validRepositoryID, err := githubid.NewRepositoryID(repositoryID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: repository id %w", ErrInvalidCaller, err)
	}
	validRunID, err := githubid.NewRunID(runID)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: run_id %w", ErrInvalidCaller, err)
	}
	validRunAttempt, err := githubid.NewRunAttempt(runAttempt)
	if err != nil {
		return Caller{}, fmt.Errorf("%w: run_attempt %w", ErrInvalidCaller, err)
	}
	return Caller{
		ownerID:           validOwnerID,
		repositoryID:      validRepositoryID,
		runID:             validRunID,
		runAttempt:        validRunAttempt,
		triggerAttributes: triggerAttributes,
	}, nil
}

// OwnerID は、不変 Owner ID を返す。
func (c Caller) OwnerID() githubid.OwnerID {
	return c.ownerID
}

// RepositoryID は、不変 Repository ID を返す。
func (c Caller) RepositoryID() githubid.RepositoryID {
	return c.repositoryID
}

// RunID は、ワークフロー実行 ID を返す。
func (c Caller) RunID() githubid.RunID {
	return c.runID
}

// RunAttempt は、試行番号を返す。
func (c Caller) RunAttempt() githubid.RunAttempt {
	return c.runAttempt
}

// WorkflowTriggerAttributes は、ワークフロー起動属性を返す。
func (c Caller) WorkflowTriggerAttributes() WorkflowTriggerAttributes {
	return c.triggerAttributes
}
