package permission

import "errors"

// ErrMalformedPermissions の文言に権限名と水準を含めない。理由は Lambda Design の「Validation」に記述する。
var ErrMalformedPermissions = errors.New("malformed permissions")
