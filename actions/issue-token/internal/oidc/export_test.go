package oidc

import (
	"context"
	"syscall"
)

// WithDialControl は、接続の直前に呼ばれる関数を本番の Dialer に足す。テストが接続の期限を、接続せずに確かめるために使う。
func WithDialControl(control func(ctx context.Context, network, address string, conn syscall.RawConn) error) Option {
	return func(c *Client) { c.dialer.ControlContext = control }
}
