package issue_test

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/githubapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/issue"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/oidc"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/response"
)

// 資格情報の値。どれもほかの値の一部にならない、見分けられる文字列にする。
const (
	testRequestToken    = "request-token-value"
	testAWSJWT          = "aws-oidc-jwt-value"
	testGATESJWT        = "gates-oidc-jwt-value"
	testAccessKeyID     = "ASIAISSUEEXAMPLE"
	testSecretAccessKey = "secret/ISSUE+KEY="
	testSessionToken    = "session+token/ISSUE=="
)

const (
	testRepositories = "example-org/repo-b,example-org/repo-a"
	testPermissions  = "contents:read"
	testRoleARN      = "arn:aws:iam::123456789012:role/gates-role"
	testFunctionName = "gates-function"
	testRegion       = "ap-northeast-1"
	testAudience     = "gates-audience"
)

// testNow は Run に渡す時刻で、UTC でない時間帯にして、ログが UTC に直すことを確かめられるようにする。
var testNow = time.Date(2030, 1, 1, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))

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
    <Message>denied</Message>
  </Error>
  <RequestId>req-2</RequestId>
</ErrorResponse>`

// handlers は、4 つのサーバーの振る舞いである。defaultHandlers はすべての段階が成功する振る舞いを返す。
type handlers struct {
	oidc, sts, lambda, github http.HandlerFunc
}

func defaultHandlers(t *testing.T) handlers {
	t.Helper()
	return handlers{
		oidc:   oidcReturns(map[string]string{"sts.amazonaws.com": testAWSJWT, testAudience: testGATESJWT}),
		sts:    stsReturns(http.StatusOK, stsSuccessXML),
		lambda: lambdaReturns(readTestdata(t, "success.json")),
		github: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
	}
}

// oidcReturns は、audience のクエリに対応する OIDC JWT を返す。対応がなければ 400 を返す。
func oidcReturns(values map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, ok := values[r.URL.Query().Get("audience")]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body, _ := json.Marshal(map[string]string{"value": value})
		_, _ = w.Write(body)
	}
}

func stsReturns(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func lambdaReturns(payload []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", name, err)
	}
	return data
}

// receivedRequest は、サーバーが受けた要求のうち、テストが確かめる部分である。
type receivedRequest struct {
	method     string
	requestURI string
	header     http.Header
	body       string
}

// recorder は、サーバーが受けた要求を記録する。handler の goroutine から書くため、mutex で守る。
type recorder struct {
	mu       sync.Mutex
	requests []receivedRequest
}

func (r *recorder) record(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
		r.mu.Lock()
		r.requests = append(r.requests, receivedRequest{method: req.Method, requestURI: req.RequestURI, header: req.Header.Clone(), body: string(body)})
		r.mu.Unlock()
		handler(w, req)
	}
}

func (r *recorder) received() []receivedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]receivedRequest(nil), r.requests...)
}

// output は、標準出力の代わりに書かれた内容を記録する。onWrite があれば、書き込みのたびに呼び、その error で書き込みを失敗させる。
// bytes.Buffer を埋め込まないのは、io.WriteString が Buffer の WriteString を呼び、onWrite を通らなくなるため。
type output struct {
	buffer  bytes.Buffer
	onWrite func(p string) error
}

func (o *output) Write(p []byte) (int, error) {
	if o.onWrite != nil {
		if err := o.onWrite(string(p)); err != nil {
			return 0, err
		}
	}
	return o.buffer.Write(p)
}

func (o *output) String() string { return o.buffer.String() }

// fixture は、4 つの TLS のサーバーに向けた Config と、各サーバーが受けた要求と、標準出力である。
type fixture struct {
	cfg                       issue.Config
	stdout                    *output
	oidc, sts, lambda, github *recorder
}

// newFixture は、h の振る舞いをするサーバーを起動し、すべての前提を満たす Config を作る。
func newFixture(t *testing.T, h handlers) *fixture {
	t.Helper()

	f := &fixture{stdout: &output{}, oidc: &recorder{}, sts: &recorder{}, lambda: &recorder{}, github: &recorder{}}
	oidcServer := startServer(t, f.oidc.record(h.oidc))
	stsServer := startServer(t, f.sts.record(h.sts))
	lambdaServer := startServer(t, f.lambda.record(h.lambda))
	githubServer := startServer(t, f.github.record(h.github))
	githubHost, err := url.Parse(githubServer.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", githubServer.URL, err)
	}

	dir := t.TempDir()
	f.cfg = issue.Config{
		Repositories:     testRepositories,
		Permissions:      testPermissions,
		RoleARN:          testRoleARN,
		FunctionName:     testFunctionName,
		AWSRegion:        testRegion,
		Audience:         testAudience,
		OIDCRequestURL:   oidcServer.URL + "/token?api-version=2.0",
		OIDCRequestToken: testRequestToken,
		OutputPath:       createFile(t, dir, "output"),
		StatePath:        createFile(t, dir, "state"),
		Stdout:           f.stdout,
		Now:              func() time.Time { return testNow },
		OIDC:             []oidc.Option{oidc.WithTransport(oidcServer.Client().Transport)},
		STS:              []awsapi.Option{awsapi.WithBaseEndpoint(stsServer.URL), awsapi.WithTransport(stsServer.Client().Transport)},
		Lambda:           []awsapi.Option{awsapi.WithBaseEndpoint(lambdaServer.URL), awsapi.WithTransport(lambdaServer.Client().Transport)},
		GitHub:           []githubapi.Option{githubapi.WithAPIHost(githubHost), githubapi.WithTransport(githubServer.Client().Transport)},
	}
	return f
}

func startServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}

// createFile は、Runner が用意する GITHUB_OUTPUT と GITHUB_STATE の代わりの空のファイルを作る。
func createFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}

// readFileIfExists は、ファイルがなければ空を返す。書けないファイルの代わりに存在しないパスを使うため。
func readFileIfExists(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}

// expectedSuccess は、success.json から response が取り出すトークンと残りの項目の値である。
// 値の形（並び、区切り）は response の規則のため、期待値を写さずに response から得る。
func expectedSuccess(t *testing.T) (token string, items response.Items) {
	t.Helper()
	success, err := response.Judge(awsapi.InvokeResult{StatusCode: http.StatusOK, Payload: readTestdata(t, "success.json")}, nil)
	if err != nil {
		t.Fatalf("response.Judge() error = %v", err)
	}
	items, err = success.Items()
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return success.Token().Expose(), items
}
