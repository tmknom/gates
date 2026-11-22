package protected

import (
	"errors"
	"io"
	"regexp"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// syntaxErrorLine は、YAML のライブラリの構文の誤りの文言のうち、行番号だけを取り出す。文言の残りは入力の断片を含み得るため捨てる。
var syntaxErrorLine = regexp.MustCompile(`^yaml: line ([0-9]+): `)

// Parse は、Lambda Design の「保護対象の一覧の書式」の YAML の文書から、INV-016 を満たす保護対象の一覧を作る。
// 満たさなかった条件をすべて、位置とともに一つの誤りとして返す理由は Lambda Design の「Runtime Entry」に記述する。
func Parse(document string) (Repositories, error) {
	var c conditions
	if !scanText(document, &c) {
		return Repositories{}, c.err()
	}
	root, ok := decodeDocument(document, &c)
	if !ok {
		return Repositories{}, c.err()
	}
	repositories := readDocument(root, &c)
	if err := c.err(); err != nil {
		return Repositories{}, err
	}
	return Repositories{parsed: true, repositories: repositories}, nil
}

// decodeDocument は、文書を構文木に復号し、文書がちょうど 1 件であることを確かめる。
// 文書を読めないか 1 件もなければ false を返し、2 件目があれば条件を加えて 1 件目を返す。
// 構文木への復号はエイリアスを展開せず、重複したキーも誤りにしないため、それらは読み取りで確かめる。
func decodeDocument(document string, c *conditions) (*yaml.Node, bool) {
	decoder := yaml.NewDecoder(strings.NewReader(document))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			c.add("document must be a mapping")
		} else {
			addSyntaxError(err, c)
		}
		return nil, false
	}
	var next yaml.Node
	if err := decoder.Decode(&next); !errors.Is(err, io.EOF) {
		c.add("must be a single document")
	}
	return &root, true
}

func addSyntaxError(err error, c *conditions) {
	if m := syntaxErrorLine.FindStringSubmatch(err.Error()); m != nil {
		c.add("line %s: is not valid YAML", m[1])
		return
	}
	c.add("is not valid YAML")
}
