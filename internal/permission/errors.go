package permission

import "errors"

// エラー文言に権限名と水準を含めない。理由は Lambda Design の「Validation」に記述する。
var ErrInvalidAppPermissions = errors.New("invalid app permissions")
