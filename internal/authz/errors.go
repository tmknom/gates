package authz

import "errors"

// ErrAuthorizationDenied と ErrUnusableInstallation の文言に Caller、要求、許可 Owner、Installation の値を含めない。理由は Lambda Design の「Processing Flow」に記述する。
// ErrUnusableInstallation を ErrAuthorizationDenied と分ける理由は Domain Model の「Lifecycle of an Issuance」に記述する。
var (
	ErrAuthorizationDenied  = errors.New("authorization denied")
	ErrInvalidAllowedOwner  = errors.New("invalid allowed owner")
	ErrInvalidCaller        = errors.New("invalid caller")
	ErrInvalidInstallation  = errors.New("invalid installation")
	ErrUnusableInstallation = errors.New("unusable installation")
)
