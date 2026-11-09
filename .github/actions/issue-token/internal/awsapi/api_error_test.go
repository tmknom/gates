package awsapi_test

import (
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
)

func TestAPIError_ErrorShowsCodeAndStatus(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  *awsapi.APIError
		want string
	}{
		"with an AWS error code": {
			err:  &awsapi.APIError{Code: "AccessDeniedException", HTTPStatus: 403},
			want: `AWS error code "AccessDeniedException", HTTP status 403`,
		},
		"without an AWS error code shows only the status": {
			err:  &awsapi.APIError{HTTPStatus: 502},
			want: "HTTP status 502 without an AWS error code",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
