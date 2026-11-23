package protected

import (
	"fmt"
	"strings"

	"github.com/tmknom/gates/internal/strictyaml"
)

// Parse は、Lambda Design の「保護対象の一覧の書式」の YAML の文書から、INV-016 を満たす保護対象の一覧を作る。
// 満たさなかった条件をすべて、位置とともに一つの誤りとして返す理由は Lambda Design の「Runtime Entry」に記述する。
func Parse(document string) (Repositories, error) {
	var c strictyaml.Conditions
	root, ok := strictyaml.Decode(document, &c)
	if !ok {
		return Repositories{}, invalid(&c)
	}
	repositories := readDocument(root, &c)
	if len(c.List()) > 0 {
		return Repositories{}, invalid(&c)
	}
	return Repositories{parsed: true, repositories: repositories}, nil
}

// invalid は、集めた条件を 1 行につないだ誤りを返す。改行でつながない理由は permission.NewIssuablePermissions と同じである。
func invalid(c *strictyaml.Conditions) error {
	return fmt.Errorf("%w: %s", ErrInvalidRepositories, strings.Join(c.List(), "; "))
}
