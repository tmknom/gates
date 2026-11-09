package awsapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

const testRegion = "ap-northeast-1"

const (
	testAccessKeyID     = "ASIAEXAMPLE"
	testSecretAccessKey = "secret/EXAMPLE+KEY="
	testSessionToken    = "session+token/EXAMPLE=="
)

// 応答の Message に入れる目印。エラー文言に現れてはならない。
const messageMarker = "SECRET-MESSAGE-MARKER"

// AWS STS の成功応答。3 つの値は testAccessKeyID などと一致する。
const stsSuccessXML = `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleWithWebIdentityResult>
    <Credentials>
      <AccessKeyId>` + testAccessKeyID + `</AccessKeyId>
      <SecretAccessKey>` + testSecretAccessKey + `</SecretAccessKey>
      <SessionToken>` + testSessionToken + `</SessionToken>
      <Expiration>2030-01-01T00:00:00Z</Expiration>
    </Credentials>
  </AssumeRoleWithWebIdentityResult>
  <ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata>
</AssumeRoleWithWebIdentityResponse>`

const stsAccessDeniedXML = `<ErrorResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <Error>
    <Type>Sender</Type>
    <Code>AccessDenied</Code>
    <Message>` + messageMarker + `</Message>
  </Error>
  <RequestId>req-2</RequestId>
</ErrorResponse>`

// capturedRequest は、サーバーが受けた要求のうち、テストが確かめる部分である。
type capturedRequest struct {
	method string
	path   string
	header http.Header
	body   string
}

// capture は、要求の内容を読み取る。handler の goroutine から呼ぶ。
func capture(r *http.Request) capturedRequest {
	body, _ := io.ReadAll(r.Body)
	return capturedRequest{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: string(body)}
}

// received は、呼び出しが返った後に、サーバーが受けた要求を取り出す。要求がなければ失敗させる。
func received(t *testing.T, got <-chan capturedRequest) capturedRequest {
	t.Helper()
	select {
	case req := <-got:
		return req
	default:
		t.Fatal("the server received no request")
		return capturedRequest{}
	}
}

func testCredentials() credential.AWSCredentials {
	return credential.NewAWSCredentials(testAccessKeyID, testSecretAccessKey, testSessionToken)
}

// newSTSClient は、handler を持つ TLS のサーバーに向けた STSClient を返す。
func newSTSClient(t *testing.T, handler http.HandlerFunc) *awsapi.STSClient {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return awsapi.NewSTSClient(testRegion, awsapi.WithBaseEndpoint(server.URL), awsapi.WithTransport(server.Client().Transport))
}

// newLambdaClient は、handler を持つ TLS のサーバーに向けた LambdaClient を返す。
func newLambdaClient(t *testing.T, handler http.HandlerFunc, creds credential.AWSCredentials) *awsapi.LambdaClient {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return awsapi.NewLambdaClient(testRegion, creds, awsapi.WithBaseEndpoint(server.URL), awsapi.WithTransport(server.Client().Transport))
}

// closedServerOptions は、すでに閉じたサーバーに向ける Option を返す。接続が拒否される。
func closedServerOptions() []awsapi.Option {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url, transport := server.URL, server.Client().Transport
	server.Close()
	return []awsapi.Option{awsapi.WithBaseEndpoint(url), awsapi.WithTransport(transport)}
}

// blockUntilCanceled は、要求の ctx が終わるまで応答を返さない。サーバーの Close を詰まらせないため。
// サーバーは本文を読み終えるまで接続の切断を検知しないため、先に本文を読む。
func blockUntilCanceled(_ http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	<-r.Context().Done()
}
