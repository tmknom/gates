package request

import "errors"

// sentinel error は、フィールドには 1 つずつ、Payload 全体の不正には拒否の種類ごとに置く。
// 文言には不正なフィールドの名前と形だけを書き、Payload の断片を含めない。理由は Lambda Design の「Validation」に記述する。
var (
	ErrPayloadTooLarge  = errors.New("payload is too large")
	ErrMalformedPayload = errors.New("malformed payload")
	ErrUnknownField     = errors.New("payload has an unknown field")

	ErrInvalidVersion      = errors.New("version must be the integer 1")
	ErrInvalidIDTokenField = errors.New("id_token must be a non-empty string")
	ErrInvalidRepositories = errors.New("invalid repositories")
	ErrInvalidPermissions  = errors.New("invalid permissions")
)
