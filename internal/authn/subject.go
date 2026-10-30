package authn

import (
	"fmt"
	"strings"
)

// 解析は復号の時点で行わない。行うと、形式の誤りが decodeClaims の文言に一本化され、ID の Claim より先に判定される。
type subject string

// 名前を照合せず、リポジトリ部の後を解釈しない理由は Lambda Design の「OIDC Validation」に記述する。
func (s subject) parse() (ownerID, repositoryID int64, err error) {
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
