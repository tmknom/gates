// Package strictyaml は、保護対象の一覧とリポジトリポリシーの YAML の文書を、認可に要らない機能を拒否しながら構文の上で読む。
// 受け付ける範囲と、ID、名前、権限の上限の値の書式は Lambda Design の「保護対象の一覧の書式」に従い、満たさなかった条件を位置とともに集める。
package strictyaml

import (
	"errors"
	"io"
	"regexp"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// syntaxErrorLine は、YAML のライブラリの構文の誤りの文言のうち、行番号だけを取り出す。文言の残りは入力の断片を含み得るため捨てる。
var syntaxErrorLine = regexp.MustCompile(`^yaml: line ([0-9]+): `)

// Decode は、文書の文字列を確かめてから構文木に復号し、トップレベルの節を返す。
// 文字の符号化が正しくないか、文書を読めないか 1 件もなければ、条件を加えて false とゼロ値の節を返す。ほかの条件は集めて true を返す。
func Decode(document string, c *Conditions) (Node, bool) {
	if !scanText(document, c) {
		return Node{}, false
	}
	root, ok := decodeDocument(document, c)
	if !ok {
		return Node{}, false
	}
	return Node{node: root.Content[0]}, true
}

// decodeDocument は、文書を構文木に復号し、文書がちょうど 1 件であることを確かめる。
// 文書を読めないか 1 件もなければ false を返し、2 件目があれば条件を加えて 1 件目を返す。
// 構文木への復号はエイリアスを展開せず、重複したキーも誤りにしないため、それらは読み取りで確かめる。
func decodeDocument(document string, c *Conditions) (*yaml.Node, bool) {
	decoder := yaml.NewDecoder(strings.NewReader(document))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			c.Add("document must be a mapping")
		} else {
			addSyntaxError(err, c)
		}
		return nil, false
	}
	var next yaml.Node
	if err := decoder.Decode(&next); !errors.Is(err, io.EOF) {
		c.Add("must be a single document")
	}
	return &root, true
}

func addSyntaxError(err error, c *Conditions) {
	if m := syntaxErrorLine.FindStringSubmatch(err.Error()); m != nil {
		c.Add("line %s: is not valid YAML", m[1])
		return
	}
	c.Add("is not valid YAML")
}
