package signer

import (
	"context"
	"errors"
	"fmt"
	"net"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// throttlingErrorCode は、スロットリングのエラーコードである。名前で分類する理由は Lambda Design の Failure にある。
const throttlingErrorCode = "ThrottlingException"

// classifyKMSFailure は、AWS SDK が返した KMS の署名の失敗を、一時的な障害か内部障害に分ける。
// 分類の規則は Lambda Design の Failure にある。
// AWS SDK の error は文言に要求の内容と KMS の message を含み得るため、包まずに文言を組み立て直す。
func classifyKMSFailure(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: kms sign was canceled or timed out", ErrTransientFailure)
	}
	// AWS SDK は、接続できない失敗と、送った後に応答を受け取れない失敗を、どちらも RequestSendError で返す
	if _, ok := errors.AsType[*smithyhttp.RequestSendError](err); ok {
		return fmt.Errorf("%w: kms sign failed without a response", ErrTransientFailure)
	}
	// 応答を受け取った失敗は ResponseError に包まれる。包まれないのは、リージョンや認証情報の解決のように要求を送る前の失敗である
	respErr, ok := errors.AsType[*awshttp.ResponseError](err)
	if !ok {
		return fmt.Errorf("%w: kms sign request was not sent", ErrInternalFailure)
	}

	// 応答の本文を受信する途中で切れた失敗も、応答を受け取れない通信の失敗である。
	// AWS SDK は本文の読み取りの失敗を復号の失敗として ResponseError に包むが、読み取りの error を連鎖に残す。
	// 完全に受信して解釈できない本文と分けるため、読み取りの error の形で判定する
	if isBodyReadFailure(err) {
		return fmt.Errorf("%w: kms sign response was cut off: status %d, request id %q", ErrTransientFailure, respErr.HTTPStatusCode(), respErr.ServiceRequestID())
	}

	// 本文を解釈できない応答には、エラーコードがない
	code := ""
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		code = apiErr.ErrorCode()
	}
	status := respErr.HTTPStatusCode()
	kind := ErrInternalFailure
	if status >= 500 || code == throttlingErrorCode {
		kind = ErrTransientFailure
	}
	return fmt.Errorf("%w: kms sign failed: code %q, status %d, request id %q", kind, code, status, respErr.ServiceRequestID())
}

// isBodyReadFailure は、本文の受信が接続の失敗で途中で切れたかを判定する。
// io.ErrUnexpectedEOF は手がかりにしない。AWS SDK の JSON の復号は、完全に受信した壊れた本文にも同じ error を返すため。
// そのため、Content-Length に満たない切断や HTTP/2 のストリームのリセットのように net.Error を満たさない切断は内部障害になる
func isBodyReadFailure(err error) bool {
	_, ok := errors.AsType[net.Error](err)
	return ok
}
