package awsapi

import (
	"context"
	"errors"
	"net"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/communication"
)

// errNotSent は、要求を送る前に SDK が失敗したことを示す。
// 原因（リージョン名の誤りなど）は SDK の文言が入力の値を含み得るため示さない。
var errNotSent = errors.New("the request was not sent")

// classify は、SDK の失敗を、AWS が返したエラー応答（*APIError）、応答を得られなかった失敗（*communication.Failure）、
// 要求を送る前の失敗（errNotSent）に分ける。SDK の文言は Message と URL を含み得るため、どれにも含めない。
// err は nil であってはならない。
func classify(err error) error {
	// 接続拒否と TLS の失敗は SDK が *url.Error（net.Error）で返し、ctx の期限切れは context.DeadlineExceeded 自身が net.Error（Timeout() が true）を満たすため、
	// どちらも net.Error の判定で拾う。ctx のキャンセル（context.Canceled）は net.Error ではない。
	// 応答の本文を読む途中の失敗も、SDK は HTTP ステータスを持つ ResponseError で包むため、ResponseError より先に判定する。
	if _, ok := errors.AsType[net.Error](err); ok || errors.Is(err, context.Canceled) {
		return communication.Failed(err)
	}
	if respErr, ok := errors.AsType[*awshttp.ResponseError](err); ok {
		apiErr := &APIError{HTTPStatus: respErr.HTTPStatusCode()}
		if smithyErr, ok := errors.AsType[smithy.APIError](err); ok {
			apiErr.Code = smithyErr.ErrorCode()
		}
		return apiErr
	}
	return errNotSent
}
