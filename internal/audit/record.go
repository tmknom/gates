// Package audit は、一回の発行処理の監査記録を組み立てて出力する。
// 項目と記録する条件は Domain Model の「Audit Record」に、キー名と出力の形は Lambda Design の「Audit Record」に従う。
package audit

import (
	"log/slog"
	"maps"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/request"
)

// Record は、一回の発行処理の監査記録のうち、相関 ID と、段階を通って確定する値である。結果とそれに伴う値は Result が持つ。
// Set で始まるメソッドで詰めた項目だけを出力し、段階の順序は検査しない。
// 詰めていない項目を nil で表す。値の型のゼロ値で表すと、未設定の判定をほかの package の型に求めることになるため。
type Record struct {
	requestID    string
	caller       *authz.Caller
	tokenRequest *request.TokenRequest
	installation *authz.Installation
}

// NewRecord は、相関 ID だけを持つ監査記録を作る。
func NewRecord(requestID string) Record {
	return Record{requestID: requestID}
}

// SetCaller は、認証で確定した Caller を詰める。
func (r *Record) SetCaller(caller authz.Caller) {
	r.caller = &caller
}

// SetTokenRequest は、形式検証を通った発行要求を詰める。
func (r *Record) SetTokenRequest(tokenRequest request.TokenRequest) {
	r.tokenRequest = &tokenRequest
}

// SetInstallation は、解決した Installation を詰める。
func (r *Record) SetInstallation(installation authz.Installation) {
	r.installation = &installation
}

// Write は、監査記録を 1 レコードとして log に書く。
// 結果を引数で受け取り、結果のない監査記録を書けないようにする。
func (r *Record) Write(log *slog.Logger, result Result) {
	attrs := []any{slog.String("request_id", r.requestID)}
	attrs = append(attrs, result.attrs()...)
	if r.caller != nil {
		attrs = append(attrs, callerAttr(*r.caller))
	}
	if r.tokenRequest != nil {
		attrs = append(attrs, tokenRequestAttr(*r.tokenRequest))
	}
	if r.installation != nil {
		attrs = append(attrs, slog.Group("installation",
			slog.Int64("id", r.installation.ID()),
			slog.Int64("owner_id", r.installation.InstallationOwnerID()),
		))
	}
	log.Info("audit record", attrs...)
}

// callerAttr は、ワークフロー起動属性のうち空文字列のものを欠落として省く。
// 空文字列の扱いは Domain Model の「Request and Decision」が定める。
func callerAttr(caller authz.Caller) slog.Attr {
	attrs := []any{
		slog.Int64("owner_id", caller.OwnerID()),
		slog.Int64("repository_id", caller.RepositoryID()),
		slog.Int64("run_id", caller.RunID()),
		slog.Int64("run_attempt", caller.RunAttempt()),
	}
	trigger := caller.WorkflowTriggerAttributes()
	for _, attr := range []slog.Attr{
		slog.String("actor", trigger.Actor),
		slog.String("actor_id", trigger.ActorID),
		slog.String("event_name", trigger.EventName),
		slog.String("ref", trigger.Ref),
		slog.String("sha", trigger.SHA),
		slog.String("workflow_ref", trigger.WorkflowRef),
		slog.String("job_workflow_ref", trigger.JobWorkflowRef),
		slog.String("environment", trigger.Environment),
	} {
		if attr.Value.String() != "" {
			attrs = append(attrs, attr)
		}
	}
	return slog.Group("caller", attrs...)
}

func tokenRequestAttr(tokenRequest request.TokenRequest) slog.Attr {
	fullNames := tokenRequest.Repositories()
	repositories := make([]string, 0, len(fullNames))
	for _, fullName := range fullNames {
		repositories = append(repositories, fullName.String())
	}
	return slog.Group("request",
		slog.Any("repositories", repositories),
		slog.Any("permissions", maps.Collect(tokenRequest.Permissions().Entries())),
	)
}
