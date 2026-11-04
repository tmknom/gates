package signer

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/go-jose/go-jose/v4"
)

const (
	testClientID  = "Iv23liTestClientID"
	testKMSKeyARN = "arn:aws:kms:ap-northeast-1:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"
	testRequestID = "kms-request-id"
	// kmsMessageMarker は、KMS の応答本文の message に置く目印で、エラー文言に応答本文と AWS SDK の文言が入らないことを確かめるために使う
	kmsMessageMarker = "kms-message-marker"
)

// testNow は、実時間に依存せず iat と exp の値を確かめるために固定する。
var testNow = time.Unix(1700000000, 0)

func testClock() time.Time { return testNow }

// kmsStub は、テストの中で生成した RSA 鍵で、受け取ったダイジェストにそのまま署名する KMS のテストダブルである。
// ダイジェストをそのまま署名するため、MessageType を誤って二重ハッシュになる誤りは検出できない。
type kmsStub struct {
	key *rsa.PrivateKey
	got *kms.SignInput
}

func (s *kmsStub) Sign(_ context.Context, in *kms.SignInput, _ ...func(*kms.Options)) (*kms.SignOutput, error) {
	s.got = in
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, in.Message)
	if err != nil {
		return nil, err
	}
	return &kms.SignOutput{Signature: signature}, nil
}

func newKMSStub(t *testing.T) *kmsStub {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	return &kmsStub{key: key}
}

func newTestSigner(t *testing.T, client kmsClient) *Signer {
	t.Helper()
	s, err := NewSigner(testClientID, testKMSKeyARN, client, testClock)
	if err != nil {
		t.Fatalf("NewSigner returned error: %v", err)
	}
	return s
}

// doFunc は、AWS SDK の KMS のクライアントに差す HTTP クライアントで、AWS に接続せずに応答を返す。
type doFunc func(*http.Request) (*http.Response, error)

func (f doFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, header http.Header, body string) doFunc {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}

func respondKMSError(status int, code string) doFunc {
	header := http.Header{"X-Amzn-Errortype": {code}, "X-Amzn-Requestid": {testRequestID}, "Content-Type": {"application/x-amz-json-1.1"}}
	return respond(status, header, `{"__type":"`+code+`","message":"`+kmsMessageMarker+`"}`)
}

// cutOffBody は、本文の先頭だけを返した後に読み取りの error を返し、受信の途中で切れた本文を表す。
type cutOffBody struct {
	r   io.Reader
	err error
}

func (b *cutOffBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if errors.Is(err, io.EOF) {
		return n, b.err
	}
	return n, err
}

func respondCutOff(status int, readErr error) doFunc {
	return func(*http.Request) (*http.Response, error) {
		header := http.Header{"X-Amzn-Requestid": {testRequestID}, "Content-Type": {"application/x-amz-json-1.1"}}
		body := &cutOffBody{r: strings.NewReader(`{"Signature":"` + kmsMessageMarker), err: readErr}
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(body)}, nil
	}
}

func respondSignature() doFunc {
	return respond(http.StatusOK, http.Header{"X-Amzn-Requestid": {testRequestID}}, `{"KeyId":"`+testKMSKeyARN+`","Signature":"AQID","SigningAlgorithm":"RSASSA_PKCS1_V1_5_SHA_256"}`)
}

// newKMSClient は、AWS SDK の KMS のクライアントを、認証情報なしで、HTTP の送信先を差し替えて作る。
// 再試行の設定は SDK の既定のままにし、KMS Signer が再試行を止めることを確かめられるようにする。
func newKMSClient(do doFunc) *kms.Client {
	return kms.New(kms.Options{Region: "ap-northeast-1", Credentials: aws.AnonymousCredentials{}, HTTPClient: do})
}

func TestNewSigner_RejectsInvalidFixedConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		clientID  string
		kmsKeyARN string
		want      error
	}{
		{name: "empty client ID returns ErrInvalidClientID", clientID: "", kmsKeyARN: testKMSKeyARN, want: ErrInvalidClientID},
		{name: "client ID that is not valid UTF-8 returns ErrInvalidClientID", clientID: "\xff", kmsKeyARN: testKMSKeyARN, want: ErrInvalidClientID},
		{name: "empty KMS Key ARN returns ErrInvalidKMSKeyARN", clientID: testClientID, kmsKeyARN: "", want: ErrInvalidKMSKeyARN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSigner(tt.clientID, tt.kmsKeyARN, nil, testClock)
			if !errors.Is(err, tt.want) {
				t.Errorf("NewSigner error = %v, want %v", err, tt.want)
			}
			if s != nil {
				t.Errorf("NewSigner returned a Signer with error %v", err)
			}
		})
	}
}

func TestSigner_Sign_ProducesVerifiableAppJWT(t *testing.T) {
	stub := newKMSStub(t)
	compact, err := newTestSigner(t, stub).sign(t.Context())
	if err != nil {
		t.Fatalf("sign returned error: %v", err)
	}

	jws, err := jose.ParseSignedCompact(compact, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatalf("ParseSignedCompact returned error: %v", err)
	}
	payload, err := jws.Verify(&stub.key.PublicKey)
	if err != nil {
		t.Fatalf("Verify with the public key returned error: %v", err)
	}

	t.Run("header has only alg RS256 and typ JWT", func(t *testing.T) {
		segment, _, _ := strings.Cut(compact, ".")
		var header map[string]string
		if err := json.Unmarshal(decodeSegment(t, segment), &header); err != nil {
			t.Fatalf("Unmarshal header returned error: %v", err)
		}
		if want := map[string]string{"alg": "RS256", "typ": "JWT"}; !maps.Equal(header, want) {
			t.Errorf("header = %v, want %v", header, want)
		}
	})

	t.Run("payload has iss, and iat and exp as JSON integers", func(t *testing.T) {
		var claims map[string]jsontext.Value
		if err := json.Unmarshal(payload, &claims); err != nil {
			t.Fatalf("Unmarshal payload returned error: %v", err)
		}
		want := map[string]string{
			"iss": `"` + testClientID + `"`,
			"iat": "1699999940", // 生成時刻の 60 秒前
			"exp": "1700000300", // 生成時刻の 5 分後
		}
		if got := slices.Sorted(maps.Keys(claims)); !slices.Equal(got, slices.Sorted(maps.Keys(want))) {
			t.Fatalf("claim names = %v, want %v", got, slices.Sorted(maps.Keys(want)))
		}
		for name, value := range want {
			if got := string(claims[name]); got != value {
				t.Errorf("claim %s = %s, want %s", name, got, value)
			}
		}
	})
}

func TestSigner_Sign_SendsDigestOfSigningInputToKMS(t *testing.T) {
	stub := newKMSStub(t)
	compact, err := newTestSigner(t, stub).sign(t.Context())
	if err != nil {
		t.Fatalf("sign returned error: %v", err)
	}

	got := stub.got
	if got.KeyId == nil || *got.KeyId != testKMSKeyARN {
		t.Errorf("KeyId = %v, want %q", got.KeyId, testKMSKeyARN)
	}
	if got.SigningAlgorithm != types.SigningAlgorithmSpecRsassaPkcs1V15Sha256 {
		t.Errorf("SigningAlgorithm = %q, want %q", got.SigningAlgorithm, types.SigningAlgorithmSpecRsassaPkcs1V15Sha256)
	}
	if got.MessageType != types.MessageTypeDigest {
		t.Errorf("MessageType = %q, want %q", got.MessageType, types.MessageTypeDigest)
	}
	signingInput := compact[:strings.LastIndex(compact, ".")]
	if digest := sha256.Sum256([]byte(signingInput)); string(got.Message) != string(digest[:]) {
		t.Errorf("Message = %x, want SHA-256 of the signing input %x", got.Message, digest)
	}
}

func TestSigner_Sign_ClassifiesKMSFailure(t *testing.T) {
	tests := []struct {
		name   string
		client *kms.Client
		ctx    func(t *testing.T) context.Context
		want   error
		// wantInMessage は、調査のためにエラー文言に含める AWS のエラーコード、HTTP ステータス、request ID である
		wantInMessage []string
	}{
		{name: "ThrottlingException returns ErrTransientFailure", client: newKMSClient(respondKMSError(http.StatusBadRequest, "ThrottlingException")), want: ErrTransientFailure, wantInMessage: []string{"ThrottlingException", testRequestID}},
		{name: "KMSInternalException with 500 returns ErrTransientFailure", client: newKMSClient(respondKMSError(http.StatusInternalServerError, "KMSInternalException")), want: ErrTransientFailure, wantInMessage: []string{"KMSInternalException", testRequestID}},
		{name: "503 with a non-JSON body returns ErrTransientFailure", client: newKMSClient(respond(http.StatusServiceUnavailable, http.Header{"Content-Type": {"text/html"}}, "<html>"+kmsMessageMarker+"</html>")), want: ErrTransientFailure, wantInMessage: []string{"503"}},
		{name: "502 with an empty body returns ErrTransientFailure", client: newKMSClient(respond(http.StatusBadGateway, http.Header{}, "")), want: ErrTransientFailure, wantInMessage: []string{"502"}},
		{name: "connection reset while receiving the body returns ErrTransientFailure", client: newKMSClient(respondCutOff(http.StatusOK, syscall.ECONNRESET)), want: ErrTransientFailure, wantInMessage: []string{"200", testRequestID}},
		{name: "connection reset while receiving an error body returns ErrTransientFailure", client: newKMSClient(respondCutOff(http.StatusBadRequest, syscall.ECONNRESET)), want: ErrTransientFailure, wantInMessage: []string{"400", testRequestID}},
		{name: "fully received but malformed error body returns ErrInternalFailure", client: newKMSClient(respond(http.StatusBadRequest, http.Header{"X-Amzn-Requestid": {testRequestID}, "Content-Type": {"application/x-amz-json-1.1"}}, `{"__type":"DisabledException","message":"`+kmsMessageMarker)), want: ErrInternalFailure, wantInMessage: []string{"400", testRequestID}},
		{name: "connection failure returns ErrTransientFailure", client: newKMSClient(func(*http.Request) (*http.Response, error) {
			return nil, errors.New(kmsMessageMarker)
		}), want: ErrTransientFailure},
		{name: "deadline exceeded while waiting for KMS returns ErrTransientFailure", client: newKMSClient(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}), ctx: func(t *testing.T) context.Context {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
			t.Cleanup(cancel)
			return ctx
		}, want: ErrTransientFailure},
		{name: "context canceled before the call returns ErrTransientFailure", client: newKMSClient(respondSignature()), ctx: func(t *testing.T) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}, want: ErrTransientFailure},
		{name: "DisabledException returns ErrInternalFailure", client: newKMSClient(respondKMSError(http.StatusBadRequest, "DisabledException")), want: ErrInternalFailure, wantInMessage: []string{"DisabledException", testRequestID}},
		{name: "KMSInvalidStateException returns ErrInternalFailure", client: newKMSClient(respondKMSError(http.StatusBadRequest, "KMSInvalidStateException")), want: ErrInternalFailure, wantInMessage: []string{"KMSInvalidStateException", testRequestID}},
		{name: "NotFoundException returns ErrInternalFailure", client: newKMSClient(respondKMSError(http.StatusBadRequest, "NotFoundException")), want: ErrInternalFailure, wantInMessage: []string{"NotFoundException", testRequestID}},
		{name: "AccessDeniedException with 403 returns ErrInternalFailure", client: newKMSClient(respondKMSError(http.StatusForbidden, "AccessDeniedException")), want: ErrInternalFailure, wantInMessage: []string{"AccessDeniedException", testRequestID}},
		{name: "200 with a malformed body returns ErrInternalFailure", client: newKMSClient(respond(http.StatusOK, http.Header{"X-Amzn-Requestid": {testRequestID}}, `{"Signature": "`+kmsMessageMarker)), want: ErrInternalFailure, wantInMessage: []string{testRequestID}},
		{name: "200 without a signature returns ErrInternalFailure", client: newKMSClient(respond(http.StatusOK, http.Header{}, "")), want: ErrInternalFailure},
		{name: "failure before sending the request returns ErrInternalFailure", client: kms.New(kms.Options{Credentials: aws.AnonymousCredentials{}, HTTPClient: respondSignature()}), want: ErrInternalFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.ctx != nil {
				ctx = tt.ctx(t)
			}
			_, err := newTestSigner(t, tt.client).Sign(ctx)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Sign error = %v, want %v", err, tt.want)
			}
			if errors.Is(err, ErrTransientFailure) && errors.Is(err, ErrInternalFailure) {
				t.Errorf("Sign error %v matches both failure kinds", err)
			}
			msg := err.Error()
			// 応答本文と AWS SDK の文言は kmsMessageMarker を含む。eyJ は JSON の Base64URL 表記の先頭で、署名入力の断片を含まないことを確かめる
			for _, secret := range []string{kmsMessageMarker, "eyJ"} {
				if strings.Contains(msg, secret) {
					t.Errorf("Sign error %q contains %q", msg, secret)
				}
			}
			for _, want := range tt.wantInMessage {
				if !strings.Contains(msg, want) {
					t.Errorf("Sign error %q does not contain %q", msg, want)
				}
			}
		})
	}
}

func TestSigner_Sign_DoesNotRetry(t *testing.T) {
	attempts := 0
	client := newKMSClient(func(r *http.Request) (*http.Response, error) {
		attempts++
		return respondKMSError(http.StatusInternalServerError, "KMSInternalException")(r)
	})
	if _, err := newTestSigner(t, client).Sign(t.Context()); !errors.Is(err, ErrTransientFailure) {
		t.Fatalf("Sign error = %v, want %v", err, ErrTransientFailure)
	}
	if attempts != 1 {
		t.Errorf("KMS was called %d times, want 1", attempts)
	}
}

func TestSigner_Sign_BoundsKMSCallTo5Seconds(t *testing.T) {
	var remaining time.Duration
	client := newKMSClient(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Errorf("request to KMS has no deadline")
		}
		remaining = time.Until(deadline)
		return respondSignature()(r)
	})
	if _, err := newTestSigner(t, client).Sign(t.Context()); err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if remaining <= 0 || remaining > 5*time.Second {
		t.Errorf("remaining time of the request to KMS = %v, want in (0, 5s]", remaining)
	}
}

func decodeSegment(t *testing.T, segment string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	return b
}
