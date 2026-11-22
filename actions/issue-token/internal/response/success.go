package response

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/tmknom/gates/actions/issue-token/internal/credential"
)

// Success は、トークンを取り出した成功応答である。
// 残りの項目は、トークンのマスクとキャンセルの確認を挟むため、Items が判定する。
type Success struct {
	token                                           credential.InstallationAccessToken
	expiresAt, repositories, permissions, requestID jsontext.Value
}

// Items は、成功応答のトークン以外の項目である。expires_at、repositories、permissions は Output に、request_id はログに出す。
type Items struct {
	ExpiresAt string
	// Repositories は、応答の順のまま , で結ぶ。
	Repositories string
	// Permissions は、権限名のバイト順の昇順に name:level を , で結ぶ。
	Permissions string
	RequestID   string
}

// Token は、成功応答から取り出したトークンを返す。
func (s Success) Token() credential.InstallationAccessToken {
	return s.token
}

// Items は、request_id、expires_at、repositories、permissions の順に判定し、最初に満たさなかった項目で失敗する。
// 拒否する値の条件は GitHub Actions Design の Inputs and Outputs に従う。
func (s Success) Items() (Items, error) {
	requestID, ok := itemString(s.requestID)
	if !ok {
		return Items{}, errors.New("the success response has no usable request_id")
	}
	unusable := func(item string) error {
		return errors.New("the success response has no usable " + item + requestIDSuffix(s.requestID))
	}

	expiresAt, ok := itemString(s.expiresAt)
	if !ok {
		return Items{}, unusable("expires_at")
	}
	repositories, ok := joinRepositories(s.repositories)
	if !ok {
		return Items{}, unusable("repositories")
	}
	permissions, ok := joinPermissions(s.permissions)
	if !ok {
		return Items{}, unusable("permissions")
	}
	return Items{
		ExpiresAt:    expiresAt,
		Repositories: repositories,
		Permissions:  permissions,
		RequestID:    requestID,
	}, nil
}

// itemString は、項目が文字列で usableItem を満たすときだけ、その値を返す。
func itemString(value jsontext.Value) (string, bool) {
	s, ok := stringOf(value)
	if !ok || !usableItem(s) {
		return "", false
	}
	return s, true
}

// usableItem は、項目の値が空でなく、改行も旧形式のワークフローコマンドの接頭辞も含まないかを返す。
// 条件は GitHub Actions Design の Inputs and Outputs に従う。
func usableItem(s string) bool {
	return s != "" && !strings.ContainsAny(s, "\r\n") && !strings.Contains(s, legacyCommandPrefix)
}

// joinRepositories は、1 件以上の配列の各要素が usableItem を満たし、, を含まないときだけ、応答の順に , で結ぶ。
func joinRepositories(value jsontext.Value) (string, bool) {
	var names []string
	if err := json.Unmarshal(value, &names); err != nil || len(names) == 0 {
		return "", false
	}
	for _, name := range names {
		if !usableItem(name) || strings.Contains(name, ",") {
			return "", false
		}
	}
	return strings.Join(names, ","), true
}

// joinPermissions は、1 件以上のオブジェクトの権限名と水準が usableItem を満たし、, と : を含まないときだけ、
// 権限名の昇順に name:level を , で結ぶ。
func joinPermissions(value jsontext.Value) (string, bool) {
	var levels map[string]string
	if err := json.Unmarshal(value, &levels); err != nil || len(levels) == 0 {
		return "", false
	}
	var pairs []string
	for _, name := range slices.Sorted(maps.Keys(levels)) {
		level := levels[name]
		if !usableItem(name) || !usableItem(level) || strings.ContainsAny(name, ",:") || strings.ContainsAny(level, ",:") {
			return "", false
		}
		pairs = append(pairs, name+":"+level)
	}
	return strings.Join(pairs, ","), true
}
