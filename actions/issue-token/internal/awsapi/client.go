package awsapi

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Option は、クライアントの接続先と HTTP クライアントを差し替える。テストが httptest のサーバーに向けるために使う。
type Option func(*options)

type options struct {
	baseEndpoint string
	httpClient   *http.Client
}

// WithBaseEndpoint は、AWS の API の接続先を endpoint に差し替える。
func WithBaseEndpoint(endpoint string) Option {
	return func(o *options) { o.baseEndpoint = endpoint }
}

// WithTransport は、要求に使う Transport を差し替える。テストが httptest の TLS サーバーの証明書を信頼するために使う。
func WithTransport(transport http.RoundTripper) Option {
	return func(o *options) { o.httpClient = &http.Client{Transport: transport} }
}

func newOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// endpoint は、Option があるときだけ接続先を返す。nil なら SDK が region から決める。
func (o options) endpoint() *string {
	if o.baseEndpoint == "" {
		return nil
	}
	return aws.String(o.baseEndpoint)
}
