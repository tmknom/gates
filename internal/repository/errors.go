package repository

import "errors"

// エラー文言に入力の値を含めない。Domain Model の監査記録は、形式検証を通らなかった要求対象を記録しないため。
var (
	ErrInvalidFullName  = errors.New("invalid full name")
	ErrInvalidOwnerName = errors.New("invalid owner name")
)
