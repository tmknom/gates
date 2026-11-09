// Package revocation は、発行したトークンの失効を試みる。
package revocation

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/workflow"
)

// revoker は失効の要求を送る。
type revoker interface {
	Revoke(ctx context.Context, token credential.InstallationAccessToken) (int, error)
}

// Attempt は、失効を要求し、結果を w に書く。
// 失効の失敗は、ワークフローを失敗させず警告にとどめる（GitHub Actions Design の Revocation）。
// そのため返す error は、w への書き込みの失敗だけである。
func Attempt(ctx context.Context, w io.Writer, client revoker, token credential.InstallationAccessToken) error {
	status, err := client.Revoke(ctx, token)
	if err != nil {
		return workflow.Warning(w, "revocation failed: "+err.Error())
	}
	if status != http.StatusNoContent {
		return workflow.Warning(w, fmt.Sprintf("revocation failed: HTTP status %d", status))
	}
	_, err = fmt.Fprintf(w, "revocation succeeded: HTTP status %d\n", status)
	return err
}
