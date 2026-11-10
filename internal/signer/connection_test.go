package signer

import (
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
)

func TestNewKMSClient_OpensConnectionForEachSign(t *testing.T) {
	var newConnections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		_, _ = io.WriteString(w, `{"KeyId":"`+testKMSKeyARN+`","Signature":"AQID","SigningAlgorithm":"RSASSA_PKCS1_V1_5_SHA_256"}`)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConnections.Add(1)
		}
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(server.Certificate())

	cfg := aws.Config{
		Region:       "ap-northeast-1",
		Credentials:  aws.AnonymousCredentials{},
		BaseEndpoint: aws.String(server.URL),
		// AWS SDK の既定の HTTP クライアントに、server の証明書の信頼だけを重ねる
		HTTPClient: awshttp.NewBuildableClient().WithTransportOptions(func(transport *http.Transport) {
			transport.TLSClientConfig.RootCAs = rootCAs
		}),
	}
	s := newTestSigner(t, NewKMSClient(cfg))
	for range 2 {
		if _, err := s.sign(t.Context()); err != nil {
			t.Fatalf("sign returned error: %v", err)
		}
	}

	if got := newConnections.Load(); got != 2 {
		t.Errorf("new connections = %d, want 2", got)
	}
}
