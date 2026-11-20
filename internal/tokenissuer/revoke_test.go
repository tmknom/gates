package tokenissuer

import (
	"context"
	"testing"
	"time"
)

// 失効は要求の ctx の中断を引き継がない。入口から通すと、発行応答を受け取った後で要求の ctx を中断させる準備が要るため、段階の関数で確かめる。
func TestRevocationContext_IgnoresCancellationOfRequestContext(t *testing.T) {
	withDeadline := func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, 30*time.Second)
	}
	tests := []struct {
		name         string
		newContext   func(context.Context) (context.Context, context.CancelFunc)
		withDeadline bool
	}{
		{name: "request context without deadline", newContext: context.WithCancel, withDeadline: false},
		{name: "request context with deadline", newContext: withDeadline, withDeadline: true},
	}
	for _, tt := range tests {
		t.Run(tt.name+" is not done after cancellation", func(t *testing.T) {
			ctx, cancel := tt.newContext(t.Context())
			cancel()

			got, cancelRevocation := revocationContext(ctx)
			defer cancelRevocation()

			if err := got.Err(); err != nil {
				t.Errorf("revocation context Err() = %v, want nil", err)
			}
			if _, ok := got.Deadline(); ok != tt.withDeadline {
				t.Errorf("revocation context has deadline = %v, want %v", ok, tt.withDeadline)
			}
		})
	}
}
