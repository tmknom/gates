package authz

import "fmt"

// RepositoryPolicyEnforcement は、デプロイがリポジトリポリシーを適用するかの固定設定である。
// フィールドを非公開の struct にし、2 つの値とゼロ値のほかを作れないようにする。
// 真偽値を取り出す口を置かないのは、ゼロ値を「適用しない」と読み違えて分岐させないため（INV-018）。
type RepositoryPolicyEnforcement struct {
	value string
}

var (
	// RepositoryPolicyEnforced は、リポジトリポリシーを適用することを表す。
	RepositoryPolicyEnforced = RepositoryPolicyEnforcement{value: "true"}
	// RepositoryPolicyNotEnforced は、リポジトリポリシーを適用しないことを表す。
	RepositoryPolicyNotEnforced = RepositoryPolicyEnforcement{value: "false"}
)

// ParseRepositoryPolicyEnforcement は、設定の値を大文字小文字と前後の空白を区別して検証する。
// strconv.ParseBool を使わないのは、1 や T のような書き損じも受け付けてしまうため（INV-018）。
// 誤りの文言に値を含めないのは、設定の誤りを値を記録せずに残すため（AC-AUTHZ-017）。
func ParseRepositoryPolicyEnforcement(s string) (RepositoryPolicyEnforcement, error) {
	switch s {
	case RepositoryPolicyEnforced.value:
		return RepositoryPolicyEnforced, nil
	case RepositoryPolicyNotEnforced.value:
		return RepositoryPolicyNotEnforced, nil
	default:
		return RepositoryPolicyEnforcement{}, fmt.Errorf("%w: must be true or false", ErrInvalidRepositoryPolicyEnforcement)
	}
}

// IsZero は、2 つの値のどれでもないゼロ値かを返す。
// ゼロ値をどちらの動作にも当てないのは、設定の読み込みを経ていない値で、運用者が決めていない動作をしないためである。
func (e RepositoryPolicyEnforcement) IsZero() bool {
	return e.value == ""
}
