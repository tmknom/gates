package authz

import "errors"

// 認可拒否の文言に Caller、要求、許可 Owner の値を含めない。理由は Lambda Design の「Processing Flow」に記述する。
var (
	ErrAuthorizationDenied = errors.New("authorization denied")
	ErrInvalidAllowedOwner = errors.New("invalid allowed owner")
	ErrInvalidCaller       = errors.New("invalid caller")
)
