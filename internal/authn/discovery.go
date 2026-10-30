package authn

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
)

type discoveryDocument struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

func discoverJWKSURL(ctx context.Context, client *http.Client, issuer *url.URL) (*url.URL, error) {
	// URL.JoinPath は Path が空の URL に先頭の / のない Path を返し、要求の行が不正になる
	discoveryURL := *issuer
	discoveryURL.Path = "/.well-known/openid-configuration"

	body, err := retrieveDocument(ctx, client, &discoveryURL)
	if err != nil {
		return nil, fmt.Errorf("retrieve discovery: %w", err)
	}

	var document discoveryDocument
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("%w: discovery cannot be decoded", ErrInternalFailure)
	}
	if document.Issuer != issuer.String() {
		return nil, fmt.Errorf("%w: discovery issuer differs from the fixed issuer", ErrInternalFailure)
	}
	jwksURL, err := url.Parse(document.JWKSURI)
	if err != nil || jwksURL.Scheme != "https" || jwksURL.Host != issuer.Host {
		return nil, fmt.Errorf("%w: discovery jwks_uri is not on the issuer host", ErrInternalFailure)
	}
	return jwksURL, nil
}
