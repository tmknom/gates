package repository

import "errors"

// エラー文言に入力の値を含めない。形式検証を通らなかった要求対象は、ログに残さないため。
var (
	ErrInvalidFullName  = errors.New("invalid full name")
	ErrInvalidOwnerName = errors.New("invalid owner name")
)
