package permission

import "errors"

// ErrMalformedPermissions の文言に権限名と水準を含めない。理由は Lambda Design の「Validation」に記述する。
var ErrMalformedPermissions = errors.New("malformed permissions")

// ErrInvalidIssuablePermissions は、発行上限が INV-015 を満たさないか、組の表記の誤りか重複を含むことを表す。
var ErrInvalidIssuablePermissions = errors.New("invalid issuable permissions")
