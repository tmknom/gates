package protected

import "errors"

// ErrInvalidRepositories は、保護対象の一覧が Lambda Design の書式か INV-016 を満たさないことを表す。
// 文言に含めない値は Lambda Design の「保護対象の一覧の書式」に記述する。
var ErrInvalidRepositories = errors.New("invalid protected repositories")
