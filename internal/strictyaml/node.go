package strictyaml

import (
	"slices"

	yaml "go.yaml.in/yaml/v3"
)

// Node は、文書の構文木の節である。ゼロ値は欠落した値を表す。
// 構文木を包んで公開しないのは、使う側が節の値を直接読み、この package の拒否の規則を通らずに値を取り出せないようにするため。
type Node struct {
	node *yaml.Node
}

// keyValue は、対応の中の、文字列のキーとその値の節の組である。
type keyValue struct {
	key   string
	value Node
}

// ReadFields は、keys だけをキーに持つ対応を読み、キーごとの値の節を返す。対応でなければ nil を返す。
// 未知のキーの名前を文言に書かない理由は Lambda Design の「保護対象の一覧の書式」に記述する。
// 欠落したキーの値はゼロ値になり、値を読む関数はゼロ値を条件を加えずに読めなかったものとして扱う。
func ReadFields(n Node, path string, keys []string, c *Conditions) map[string]Node {
	pairs, _, ok := readMapping(n, path, c)
	if !ok {
		return nil
	}
	fields := make(map[string]Node, len(keys))
	for _, pair := range pairs {
		_, duplicate := fields[pair.key]
		switch {
		case !slices.Contains(keys, pair.key):
			c.Add("%s has an unknown key", label(path))
		case duplicate:
			c.Add("%s is a duplicate key", child(path, pair.key))
		default:
			fields[pair.key] = pair.value
		}
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			c.Add("%s is required", child(path, key))
		}
	}
	return fields
}

// ReadSequence は、列の要素の節を返す。列でなければ空を返す。
func ReadSequence(n Node, path string, c *Conditions) []Node {
	if !accept(n, c) {
		return nil
	}
	if n.node.Kind != yaml.SequenceNode {
		c.Add("%s must be a sequence", label(path))
		return nil
	}
	items := make([]Node, 0, len(n.node.Content))
	for _, item := range n.node.Content {
		items = append(items, Node{node: item})
	}
	return items
}

// readMapping は、対応のうち、キーが文字列のスカラーである組を書かれた順に返す。対応でなければ ok に false を返す。
// 読めないキーの組は条件を加えて除き、除いた組があれば complete に false を返す。
// 引用符のない << はマージキーとして拒否し、引用符で囲んだ "<<" は文字列のキーとして読む。
// 重複したキーは、キーの意味によって扱いが変わるため、呼び出し元が判定する。
func readMapping(n Node, path string, c *Conditions) (pairs []keyValue, complete, ok bool) {
	if !accept(n, c) {
		return nil, false, false
	}
	if n.node.Kind != yaml.MappingNode {
		c.Add("%s must be a mapping", label(path))
		return nil, false, false
	}
	complete = true
	for i := 0; i < len(n.node.Content); i += 2 {
		key, value := Node{node: n.node.Content[i]}, Node{node: n.node.Content[i+1]}
		switch {
		case !accept(key, c):
			complete = false
		case isPlain(key) && key.node.Kind == yaml.ScalarNode && key.node.Value == "<<":
			c.Add("line %d: must not have a merge key", key.node.Line)
			complete = false
		case !isString(key):
			c.Add("%s has a key that is not a string", label(path))
			complete = false
		default:
			pairs = append(pairs, keyValue{key: key.node.Value, value: value})
		}
	}
	return pairs, complete, true
}

// readString は、文字列のスカラーの値を返す。引用符のないスカラーは、Core Schema で文字列に解決されるものに限る。
func readString(n Node, path string, c *Conditions) (string, bool) {
	if !accept(n, c) {
		return "", false
	}
	if !isString(n) {
		c.Add("%s must be a string", label(path))
		return "", false
	}
	return n.node.Value, true
}

// readQuoted は、引用符（" か '）で囲んだスカラーの値を、エスケープを展開して返す。ブロックスカラーは引用符で囲んだ値として扱わない。
func readQuoted(n Node, path string, c *Conditions) (string, bool) {
	if !accept(n, c) {
		return "", false
	}
	if n.node.Kind != yaml.ScalarNode || n.node.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) == 0 {
		c.Add("%s must be a quoted string", label(path))
		return "", false
	}
	return n.node.Value, true
}

// accept は、節を読んでよいかを返す。欠落した値（ゼロ値）とエイリアスは読まず、エイリアスとアンカーを拒否する。
// エイリアスを展開しないため、展開先の種類の誤りを重ねて報告しない。
func accept(n Node, c *Conditions) bool {
	if n.node == nil {
		return false
	}
	if n.node.Kind == yaml.AliasNode {
		c.Add("line %d: must not be an alias", n.node.Line)
		return false
	}
	if n.node.Anchor != "" {
		c.Add("line %d: must not have an anchor", n.node.Line)
	}
	return true
}

func isString(n Node) bool {
	return n.node.Kind == yaml.ScalarNode && (!isPlain(n) || !resolvesToNonString(n.node.Value))
}

// isPlain は、引用符もブロックスカラーの記号も持たないスカラーかを返す。明示的なタグは解析の前の文字列で拒否するため、ここでは見ない。
func isPlain(n Node) bool {
	return n.node.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0
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
