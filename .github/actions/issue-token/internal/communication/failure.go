// Package communication は、応答を得られなかった通信の失敗を表す error を提供する。
// 通信する package が同じ規則で失敗の種類を判定するため、判定をここに閉じる。
package communication

import (
	"context"
	"errors"
	"net"
)

// Failure は、応答を得られなかった通信の失敗である。
// 標準ライブラリと AWS SDK の文言は URL や要求の一部を含み得るため、文言は失敗の種類だけを示し、cause は errors.Is と errors.As のために Unwrap で返す。
// 種類を sentinel error にしないのは、使う側が種類で分岐せず、文言を注釈にするだけのため。
type Failure struct{ cause error }

// Failed は、cause を通信の失敗として包む。
func Failed(cause error) error {
	return &Failure{cause: cause}
}

// Error は、失敗の種類を固定の文言で返す。
func (e *Failure) Error() string {
	var netErr net.Error
	if errors.Is(e.cause, context.DeadlineExceeded) || (errors.As(e.cause, &netErr) && netErr.Timeout()) {
		return "the request timed out"
	}
	return "the request failed without a response"
}

// Unwrap は、包んだ cause を返す。
func (e *Failure) Unwrap() error {
	return e.cause
}
