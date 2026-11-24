package github

import "errors"

// ResolveInstallation の error は、authz.ErrInstallationUnavailable か ErrTransientFailure のどちらか一つを満たす。
// Installation の不変条件を満たさない応答の error は、失敗の種別ではない authz.ErrInvalidInstallation も満たす。
// Issue と IssuePolicyReadToken の error は、ErrIssuanceRejected、ErrTransientFailure、authz.ErrInstallationUnavailable のどれか一つを満たす。
// FetchRepositoryPolicy の error は、ErrTransientFailure か ErrInternalFailure のどちらか一つを満たす。
// Installation 利用不可は、Grant が返す Installation 利用不可と同じ失敗の種別のため、authz の sentinel で表す。
// 一時的な障害を依存先ごとの sentinel で表す理由は Lambda Design の「Failure」に記述する。
//
// 文言に含める情報と伏せる値は、Lambda Design の「応答とエラー文言に出す情報」に従う。
var (
	ErrTransientFailure = errors.New("transient failure")
	// ErrIssuanceRejected は、GitHub 発行拒否を表す。
	ErrIssuanceRejected = errors.New("github issuance rejected")
	// ErrInternalFailure は、リポジトリポリシーの取得が、再試行では解消せず、ファイルの有無も示さない形で終わったことを表す。
	ErrInternalFailure = errors.New("internal failure")
)
