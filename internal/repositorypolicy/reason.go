package repositorypolicy

import "errors"

// リポジトリポリシーの条件の拒否の理由である。どれを返すかの区別は Lambda Design の「Audit Record」に記述する。
// 文言は固定で、どの許可 Caller が当たったかも示さない。理由は Lambda Design の「応答とエラー文言に出す情報」に記述する。
var (
	// errUninitialized は、構築関数を経ていないゼロ値の取得の結果で判定したことを表す
	errUninitialized             = errors.New("repository policy fetch result is not initialized")
	errNotFound                  = errors.New("repository policy is not found")
	errNotFile                   = errors.New("repository policy is not a file")
	errTooLarge                  = errors.New("repository policy is too large")
	errInvalid                   = errors.New("repository policy is invalid")
	errNoAllowedCallers          = errors.New("repository policy lists no allowed callers")
	errNoMatchingAllowedCaller   = errors.New("repository policy has no allowed caller matching both the caller repository id and name")
	errAllowedCallerNameMismatch = errors.New("repository policy has an allowed caller matching the caller repository id but not the name")
	errAllowedCallerIDMismatch   = errors.New("repository policy has an allowed caller matching the caller repository name but not the id")
	errExceedsPermissionLimit    = errors.New("permissions are not within the permission limit of the repository policy")
)
