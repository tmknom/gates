package communication_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/communication"
)

// timeoutError は、Timeout() が true の net.Error である。
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout on https://example.com/secret" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return false }

func TestFailed_ErrorTellsTheKindOfFailure(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cause error
		want  string
	}{
		"context.DeadlineExceeded is a timeout": {
			cause: context.DeadlineExceeded,
			want:  "the request timed out",
		},
		"net.Error with Timeout() is a timeout": {
			cause: &net.OpError{Op: "dial", Net: "tcp", Err: timeoutError{}},
			want:  "the request timed out",
		},
		"context.Canceled is not a timeout": {
			cause: context.Canceled,
			want:  "the request failed without a response",
		},
		"other error is not a timeout": {
			cause: errors.New("connection refused"),
			want:  "the request failed without a response",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := communication.Failed(tt.cause).Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFailed_UnwrapReturnsTheCause(t *testing.T) {
	t.Parallel()

	err := communication.Failed(context.DeadlineExceeded)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(%v, context.DeadlineExceeded) = false, want true", err)
	}
}
