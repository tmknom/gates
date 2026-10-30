package repository

import "fmt"

// OwnerName は、完全名のオーナー部分と許可オーナー名に共通するオーナーの名前である。
// フィールドを非公開にし、ゼロ値以外は ParseOwnerName でしか作れないようにする。
type OwnerName struct {
	// == と map のキーは大文字小文字を区別して比べるため、比較できない型にして Equal だけを使わせる
	_     doNotCompare
	value string
}

// doNotCompare をフィールドに持つ struct は、== と map のキーに使えない。
// 比較できない func 型を長さ 0 の配列で持たせ、メモリを使わずに比較を禁じる。
// google.golang.org/protobuf の DoNotCompare で知られた技法である。
type doNotCompare [0]func()

// ParseOwnerName は、GitHub のオーナー名の文字種に従って検証する。
func ParseOwnerName(s string) (OwnerName, error) {
	if s == "" {
		return OwnerName{}, fmt.Errorf("%w: must not be empty", ErrInvalidOwnerName)
	}
	for i := range len(s) {
		if !isASCIIAlphanumeric(s[i]) && s[i] != '-' {
			return OwnerName{}, fmt.Errorf("%w: must consist of ASCII letters, digits, or '-'", ErrInvalidOwnerName)
		}
	}
	return OwnerName{value: s}, nil
}

// Equal は、ASCII の範囲で大文字小文字を区別せずに比べる。
func (o OwnerName) Equal(other OwnerName) bool {
	return equalFoldASCII(o.value, other.value)
}

// String は、ParseOwnerName に渡された表記をそのまま返す。
func (o OwnerName) String() string {
	return o.value
}
