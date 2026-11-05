package handler

import (
	"errors"
	"unicode/utf8"
)

// ErrInvalidRequestID は、相関 ID が UTF-8 として正しくないことを表す。
// この相関 ID では応答を符号化できないため、発行処理を始めない。理由は Lambda Design の「Failure」に記述する。
var ErrInvalidRequestID = errors.New("request ID is not valid UTF-8")

// RequestID は、UTF-8 として正しいことを確かめた相関 ID である。
// Handle が受け取る相関 ID をこの型に限り、応答を書けない相関 ID で発行処理を始めないようにする。
type RequestID struct {
	value string
}

// NewRequestID は、UTF-8 として正しい相関 ID だけから RequestID を作る。
func NewRequestID(value string) (RequestID, error) {
	if !utf8.ValidString(value) {
		return RequestID{}, ErrInvalidRequestID
	}
	return RequestID{value: value}, nil
}

// String は、相関 ID の値を返す。
func (id RequestID) String() string {
	return id.value
}
