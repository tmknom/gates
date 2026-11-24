package tokenissuer

import (
	"context"

	"github.com/tmknom/gates/internal/github"
)

// signerStub は KMS Signer のテストダブルである。
// KMS Signer の作った GitHub App JWT を GitHub Client が送ることは、KMS のテストダブルを持つ KMS Signer の package のテストで確かめる。
type signerStub struct {
	err   error
	calls int
}

// appJWTMarker は、KMS Signer のテストダブルが返す GitHub App JWT で、要求とエラー文言での扱いを確かめる目印である。
const appJWTMarker = "app-jwt-marker"

func (s *signerStub) Sign(context.Context) (github.AppJWT, error) {
	s.calls++
	if s.err != nil {
		return github.AppJWT{}, s.err
	}
	return github.NewAppJWT(appJWTMarker), nil
}
