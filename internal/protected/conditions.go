package protected

import (
	"fmt"
	"slices"
	"strings"
)

// conditions は、読み取りで満たさなかった条件を、位置とともに見つけた順に集める。
type conditions struct {
	list []string
}

// add は、同じ文言の条件を一度だけ加える。未知のキーのように、同じ位置で同じ条件が重なることがあるため。
func (c *conditions) add(format string, args ...any) {
	condition := fmt.Sprintf(format, args...)
	if !slices.Contains(c.list, condition) {
		c.list = append(c.list, condition)
	}
}

// err は、集めた条件を 1 行につないだ誤りを返す。改行でつながない理由は permission.NewIssuablePermissions と同じである。
func (c *conditions) err() error {
	if len(c.list) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrInvalidRepositories, strings.Join(c.list, "; "))
}
