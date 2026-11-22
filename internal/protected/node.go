package protected

import (
	"slices"

	yaml "go.yaml.in/yaml/v3"
)

// keyValue は、対応の中の、文字列のキーとその値のノードの組である。
type keyValue struct {
	key   string
	value *yaml.Node
}

// readFields は、keys だけをキーに持つ対応を読み、キーごとの値のノードを返す。対応でなければ nil を返す。
// 未知のキーの名前を文言に書かない理由は Lambda Design の「保護対象の一覧の書式」に記述する。
// 欠落したキーの値は nil になり、値を読む関数は nil を条件を加えずに読めなかったものとして扱う。
func readFields(n *yaml.Node, path string, keys []string, c *conditions) map[string]*yaml.Node {
	pairs, _, ok := readMapping(n, path, c)
	if !ok {
		return nil
	}
	fields := make(map[string]*yaml.Node, len(keys))
	for _, pair := range pairs {
		switch {
		case !slices.Contains(keys, pair.key):
			c.add("%s has an unknown key", label(path))
		case fields[pair.key] != nil:
			c.add("%s is a duplicate key", child(path, pair.key))
		default:
			fields[pair.key] = pair.value
		}
	}
	for _, key := range keys {
		if fields[key] == nil {
			c.add("%s is required", child(path, key))
		}
	}
	return fields
}

// readMapping は、対応のうち、キーが文字列のスカラーである組を書かれた順に返す。対応でなければ ok に false を返す。
// 読めないキーの組は条件を加えて除き、除いた組があれば complete に false を返す。
// 引用符のない << はマージキーとして拒否し、引用符で囲んだ "<<" は文字列のキーとして読む。
// 重複したキーは、キーの意味によって扱いが変わるため、呼び出し元が判定する。
func readMapping(n *yaml.Node, path string, c *conditions) (pairs []keyValue, complete, ok bool) {
	if !accept(n, c) {
		return nil, false, false
	}
	if n.Kind != yaml.MappingNode {
		c.add("%s must be a mapping", label(path))
		return nil, false, false
	}
	complete = true
	for i := 0; i < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		switch {
		case !accept(key, c):
			complete = false
		case isPlain(key) && key.Kind == yaml.ScalarNode && key.Value == "<<":
			c.add("line %d: must not have a merge key", key.Line)
			complete = false
		case !isString(key):
			c.add("%s has a key that is not a string", label(path))
			complete = false
		default:
			pairs = append(pairs, keyValue{key: key.Value, value: value})
		}
	}
	return pairs, complete, true
}

// readSequence は、列の要素のノードを返す。列でなければ空を返す。
func readSequence(n *yaml.Node, path string, c *conditions) []*yaml.Node {
	if !accept(n, c) {
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		c.add("%s must be a sequence", label(path))
		return nil
	}
	return n.Content
}

// readString は、文字列のスカラーの値を返す。引用符のないスカラーは、Core Schema で文字列に解決されるものに限る。
func readString(n *yaml.Node, path string, c *conditions) (string, bool) {
	if !accept(n, c) {
		return "", false
	}
	if !isString(n) {
		c.add("%s must be a string", label(path))
		return "", false
	}
	return n.Value, true
}

// readQuoted は、引用符（" か '）で囲んだスカラーの値を、エスケープを展開して返す。ブロックスカラーは引用符で囲んだ値として扱わない。
func readQuoted(n *yaml.Node, path string, c *conditions) (string, bool) {
	if !accept(n, c) {
		return "", false
	}
	if n.Kind != yaml.ScalarNode || n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) == 0 {
		c.add("%s must be a quoted string", label(path))
		return "", false
	}
	return n.Value, true
}

// accept は、ノードを読んでよいかを返す。欠落した値（nil）とエイリアスは読まず、エイリアスとアンカーを拒否する。
// エイリアスを展開しないため、展開先の種類の誤りを重ねて報告しない。
func accept(n *yaml.Node, c *conditions) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.AliasNode {
		c.add("line %d: must not be an alias", n.Line)
		return false
	}
	if n.Anchor != "" {
		c.add("line %d: must not have an anchor", n.Line)
	}
	return true
}

func isString(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && (!isPlain(n) || !resolvesToNonString(n.Value))
}

// isPlain は、引用符もブロックスカラーの記号も持たないスカラーかを返す。明示的なタグは解析の前の文字列で拒否するため、ここでは見ない。
func isPlain(n *yaml.Node) bool {
	return n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0
}

func child(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// label は、誤りの文言で path を示す語を返す。文書そのものの位置は空の path で表す。
func label(path string) string {
	if path == "" {
		return "document"
	}
	return path
}
