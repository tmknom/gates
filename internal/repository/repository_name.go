package repository

import (
	"errors"
	"fmt"
)

const maxRepositoryNameLength = 100

// ValidateRepositoryName は、オーナー名を含まないリポジトリ名が、GitHub の新規リポジトリ名の文字種と長さを満たすかを検証する。
// 文言は、呼び出し元がどの名前かを前置して読める形にし、入力の値を含めない。
func ValidateRepositoryName(s string) error {
	for i := range len(s) {
		c := s[i]
		if !isASCIIAlphanumeric(c) && c != '-' && c != '_' && c != '.' {
			return errors.New("must consist of ASCII letters, digits, '-', '_', or '.'")
		}
	}
	// 文字種を ASCII に限った後なので、バイト数が文字数に等しい
	if s == "" || len(s) > maxRepositoryNameLength {
		return fmt.Errorf("must be 1 to %d characters", maxRepositoryNameLength)
	}
	return nil
}
