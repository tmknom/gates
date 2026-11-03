package authn

import (
	"fmt"
	"strings"
	"unicode"
)

// 解析は復号の時点で行わない。行うと、形式の誤りが decodeClaims の文言に一本化され、ID の Claim より先に判定される。
type subject string

// maxSubjectLength の値の理由は JWT Design の「`sub`」に記述する。
const maxSubjectLength = 255

// 名前を照合せず、リポジトリ部の後を解釈しない理由は JWT Design の「`sub`」に記述する。
func (s subject) parse() (ownerID, repositoryID int64, err error) {
	// ASCII に限ったあとはバイト数が文字数に等しいため、len で文字数の上限を判定できる
	if strings.ContainsFunc(string(s), func(r rune) bool { return r > unicode.MaxASCII }) {
		return 0, 0, fmt.Errorf("%w: sub must consist of ASCII characters", ErrAuthenticationFailure)
	}
	if len(s) > maxSubjectLength {
		return 0, 0, fmt.Errorf("%w: sub exceeds the maximum length", ErrAuthenticationFailure)
	}
	rest, ok := strings.CutPrefix(string(s), "repo:")
	if !ok {
		return 0, 0, fmt.Errorf("%w: sub must start with the repo prefix", ErrAuthenticationFailure)
	}
	ownerPart, rest, ok := strings.Cut(rest, "/")
	if !ok {
		return 0, 0, fmt.Errorf("%w: sub must separate owner and repository with a slash", ErrAuthenticationFailure)
	}
	repositoryPart, _, ok := strings.Cut(rest, ":")
	if !ok {
		return 0, 0, fmt.Errorf("%w: sub must have a colon after the repository", ErrAuthenticationFailure)
	}

	ownerID, err = parseNamedID("sub owner", ownerPart)
	if err != nil {
		return 0, 0, err
	}
	repositoryID, err = parseNamedID("sub repository", repositoryPart)
	if err != nil {
		return 0, 0, err
	}
	return ownerID, repositoryID, nil
}

func parseNamedID(partName, part string) (int64, error) {
	if strings.Count(part, "@") != 1 {
		return 0, fmt.Errorf("%w: %s must have exactly one at sign", ErrAuthenticationFailure, partName)
	}
	name, id, _ := strings.Cut(part, "@")
	if name == "" {
		return 0, fmt.Errorf("%w: %s must have a name", ErrAuthenticationFailure, partName)
	}
	return parseID(partName+" id", id)
}
