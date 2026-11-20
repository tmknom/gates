package permission

import (
	"errors"
	"fmt"
	"strings"
)

// entry は、権限名と水準の組（Permission Entry）である。
// 権限名と水準を比べるだけで同じ組かが決まるため、== と map のキーに使える。
type entry struct {
	name  string
	level string
}

// parseEntry は、権限名と水準を `:` でつないだ表記（`contents:read`）を読む。
// `:` を 2 つ以上含む表記は、水準に `:` が残って文字種を満たさないため拒否される。
// 文言に入力の値を含めない。
func parseEntry(s string) (entry, error) {
	name, level, found := strings.Cut(s, ":")
	if !found {
		return entry{}, errors.New("must be a permission name and a level joined by ':'")
	}
	if !wellFormedTerm(name) {
		return entry{}, fmt.Errorf("permission name %w", errMalformedTerm)
	}
	if !wellFormedTerm(level) {
		return entry{}, fmt.Errorf("permission level %w", errMalformedTerm)
	}
	return entry{name: name, level: level}, nil
}
