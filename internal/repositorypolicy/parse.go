package repositorypolicy

import (
	"github.com/tmknom/gates/internal/allowedcaller"
	"github.com/tmknom/gates/internal/strictyaml"
)

// maxDocumentBytes は、リポジトリポリシーの大きさの上限である。値の根拠は Lambda Design の「Repository Policy」に記述する。
const maxDocumentBytes = 64 * 1024

var documentKeys = []string{"callers"}

// parse は、文書から INV-017 を満たす許可 Caller の集合を作る。
// 満たさなかった条件は位置と値を含み得るため捨て、一つの誤りだけを返す。理由は Lambda Design の「応答とエラー文言に出す情報」に記述する。
func parse(document string) (allowedcaller.Set, error) {
	if len(document) > maxDocumentBytes {
		return allowedcaller.Set{}, errTooLarge
	}
	var c strictyaml.Conditions
	// 読めない文書は Decode が条件を加えるため、下の条件の確認で拒否する
	root, _ := strictyaml.Decode(document, &c)
	fields := strictyaml.ReadFields(root, "", documentKeys, &c)
	set := allowedcaller.Read(fields["callers"], "callers", &c)
	if len(c.List()) > 0 {
		return allowedcaller.Set{}, errInvalid
	}
	return set, nil
}
