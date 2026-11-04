package github

import "errors"

// ResolveInstallation の error は、authz.ErrInstallationUnavailable か ErrTransientFailure のどちらか一つを満たす。
// Installation の不変条件を満たさない応答の error は、失敗の種別ではない authz.ErrInvalidInstallation も満たす。
// Installation 利用不可は、Grant が返す Installation 利用不可と同じ失敗の種別のため、authz の sentinel で表す。
// 一時的な障害を依存先ごとの sentinel で表す理由は Lambda Design の「Failure」に記述する。
//
// 文言には、応答本文、GitHub App JWT、許可オーナーと Installation の値を含めず、応答のステータスと GitHub の request ID を含める。
// 値を伏せる理由は Lambda Design の「応答とエラー文言に出す情報」に記述する。
var (
	ErrTransientFailure = errors.New("transient failure")
)
