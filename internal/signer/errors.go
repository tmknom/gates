package signer

import "errors"

// Sign の error は、次の 2 つの失敗の種別のどちらか一つを満たす。
//
// 文言に KMS の応答本文と AWS SDK の文言を含めない。AWS SDK の文言には、要求の内容と KMS が返した message が入り得るため。
// AWS のエラーコード、HTTP ステータス、request ID は、調査のために含める。
var (
	ErrTransientFailure = errors.New("transient failure")
	ErrInternalFailure  = errors.New("internal failure")
)

// 署名の固定設定の不変条件の違反で、Sign の失敗の種別ではない。
var (
	ErrInvalidClientID  = errors.New("invalid client id")
	ErrInvalidKMSKeyARN = errors.New("invalid kms key arn")
)
