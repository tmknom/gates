package request

import (
	"errors"
	"fmt"
)

// ErrInvalidRequest は、入力不正という失敗の種別を表す。Validate の error はどれもこれを満たす。
// 種別をエラーコードに写す Lambda Handler が、規則ごとの sentinel error を列挙せずに済ませるため、種別の sentinel error を置く。
var ErrInvalidRequest = errors.New("invalid request")

// 規則ごとの sentinel error は、フィールドには 1 つずつ、Payload 全体の不正には拒否の種類ごとに置く。
// 文言には不正なフィールドの名前と形だけを書き、Payload の断片を含めない。理由は Lambda Design の「Validation」に記述する。
var (
	ErrPayloadTooLarge  = fmt.Errorf("%w: payload is too large", ErrInvalidRequest)
	ErrMalformedPayload = fmt.Errorf("%w: malformed payload", ErrInvalidRequest)
	ErrUnknownField     = fmt.Errorf("%w: payload has an unknown field", ErrInvalidRequest)

	ErrInvalidVersion      = fmt.Errorf("%w: version must be the integer 1", ErrInvalidRequest)
	ErrInvalidIDTokenField = fmt.Errorf("%w: id_token must be a non-empty string", ErrInvalidRequest)
	ErrInvalidRepositories = fmt.Errorf("%w: invalid repositories", ErrInvalidRequest)
	ErrInvalidPermissions  = fmt.Errorf("%w: invalid permissions", ErrInvalidRequest)
)
