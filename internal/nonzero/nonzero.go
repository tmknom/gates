// Package nonzero は、GATES のコードで panic してよい唯一の場所である。
// panic してよい場合は Lambda Design の「Failure」に記述する。
package nonzero

import "fmt"

// Require は、value がゼロ値なら panic する。
// panic の値には型名だけを入れ、値を入れない。資格情報を持つ型も渡るためである。
func Require[T interface{ IsZero() bool }](value T) {
	if value.IsZero() {
		panic(fmt.Sprintf("zero value of %T", value))
	}
}
