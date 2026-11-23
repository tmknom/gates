package strictyaml

import "fmt"

// Conditions は、読み取りで満たさなかった条件を、位置とともに見つけた順に集める。
// 加えた条件を集合でも持つのは、大きさの上限に収まるリポジトリポリシーでも条件を数万件作れ、列だけで重複を探すと条件の数の 2 乗の時間がかかるため。
type Conditions struct {
	list []string
	seen map[string]bool
}

// Add は、同じ文言の条件を一度だけ加える。未知のキーのように、同じ位置で同じ条件が重なることがあるため。
func (c *Conditions) Add(format string, args ...any) {
	condition := fmt.Sprintf(format, args...)
	if c.seen[condition] {
		return
	}
	if c.seen == nil {
		c.seen = make(map[string]bool)
	}
	c.seen[condition] = true
	c.list = append(c.list, condition)
}

// List は、集めた条件を見つけた順に返す。
func (c *Conditions) List() []string {
	return c.list
}
