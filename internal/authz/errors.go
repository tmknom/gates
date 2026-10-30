package authz

import "errors"

// ErrAuthorizationDenied と ErrInstallationUnavailable の文言に Caller、要求、許可オーナー、Installation の値を含めない。理由は Lambda Design の「Processing Flow」に記述する。
// ErrInstallationUnavailable を ErrAuthorizationDenied と分ける理由は Domain Model の「Failure Kinds」に記述する。
var (
	ErrAuthorizationDenied     = errors.New("authorization denied")
	ErrInstallationUnavailable = errors.New("installation unavailable")
	ErrInvalidAllowedOwner     = errors.New("invalid allowed owner")
	ErrInvalidCaller           = errors.New("invalid caller")
	ErrInvalidInstallation     = errors.New("invalid installation")
)
