// Package repositorypolicy は、リポジトリポリシーの読み取りと、対象 1 件についての RULE-016 の判定である。読み取りは INV-017 を扱う。
package repositorypolicy

// FetchResult は、対象 1 件のリポジトリポリシーの取得の結果である。
// 取得できなかったことを表す値は作れず、取得できない対象を判定に渡せない。
// ゼロ値は取得の結果ではなく、Authorize は初期化の漏れとして拒否する。
type FetchResult struct {
	kind     fetchKind
	document string
}

type fetchKind int

const (
	uninitialized fetchKind = iota
	fetchedDocument
	notFound
	notFile
	tooLarge
)

// Document は、文書を取得できた結果である。文書は読み取る前の文字列で、大きさと内容は Authorize が確かめる。
func Document(document string) FetchResult {
	return FetchResult{kind: fetchedDocument, document: document}
}

// NotFound は、リポジトリポリシーのパスにファイルがない結果である。
func NotFound() FetchResult {
	return FetchResult{kind: notFound}
}

// NotFile は、リポジトリポリシーのパスにあるのがファイルでない結果である。
func NotFile() FetchResult {
	return FetchResult{kind: notFile}
}

// TooLarge は、ファイルが大きさの上限を超える結果である。
func TooLarge() FetchResult {
	return FetchResult{kind: tooLarge}
}
