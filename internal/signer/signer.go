package signer

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/tmknom/gates/internal/github"
)

// signTimeout は、KMS の署名 1 回を待つ上限である。値の理由は Lambda Design の Call Time Budget にある。
const signTimeout = 5 * time.Second

// iat と exp の生成条件は GitHub App JWT の GAJ-IAT と GAJ-EXP にある。
const (
	issuedAtSkew = 60 * time.Second
	lifetime     = 5 * time.Minute
)

// encodedHeader は、JOSE Header を Base64URL にした値である。Header は Client ID にも時刻にもよらない。
var encodedHeader = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))

// kmsClient は、KMS の Sign だけを持つ。AWS SDK の KMS のクライアントがそのまま満たし、テストではテストダブルに差し替える。
type kmsClient interface {
	Sign(ctx context.Context, params *kms.SignInput, optFns ...func(*kms.Options)) (*kms.SignOutput, error)
}

// Signer は、固定した GitHub App と KMS Key で GitHub App JWT を作る KMS Signer である。
type Signer struct {
	// encodedClientID は、Client ID を JSON の文字列にした値で、Payload の iss に置く
	encodedClientID jsontext.Value
	kmsKeyARN       string
	client          kmsClient
	now             func() time.Time
}

// NewSigner は、署名の固定設定の不変条件（Domain Model）を満たさなければ作らない。
// Client ID を作る時点で JSON の文字列にしておき、署名のたびに JSON にできない Client ID で失敗する分岐をなくす。
func NewSigner(clientID, kmsKeyARN string, client kmsClient, now func() time.Time) (*Signer, error) {
	if clientID == "" {
		return nil, fmt.Errorf("%w: must not be empty", ErrInvalidClientID)
	}
	// encoding/json/v2 は UTF-8 として正しくない文字列を JSON にしない。json の文言には Client ID の断片が入り得るため、包まない
	encodedClientID, err := json.Marshal(clientID)
	if err != nil {
		return nil, fmt.Errorf("%w: must be valid UTF-8", ErrInvalidClientID)
	}
	if kmsKeyARN == "" {
		return nil, fmt.Errorf("%w: must not be empty", ErrInvalidKMSKeyARN)
	}
	return &Signer{encodedClientID: encodedClientID, kmsKeyARN: kmsKeyARN, client: client, now: now}, nil
}

// Sign は、GitHub App JWT を作る。
func (s *Signer) Sign(ctx context.Context) (github.AppJWT, error) {
	compact, err := s.sign(ctx)
	if err != nil {
		return github.AppJWT{}, err
	}
	return github.NewAppJWT(compact), nil
}

// sign は、GitHub App JWT を JWS Compact Serialization の文字列で返す。
// Sign から分けるのは、AppJWT の値をこの package から取り出せず、署名の結果をこの関数の戻り値で検証するためである。
func (s *Signer) sign(ctx context.Context) (string, error) {
	input := signingInput(s.encodedClientID, s.now())
	// 計算済みのダイジェストを MessageType の DIGEST で渡し、KMS に署名入力をもう一度ハッシュさせない
	digest := sha256.Sum256([]byte(input))

	ctx, cancel := context.WithTimeout(ctx, signTimeout)
	defer cancel()
	out, err := s.client.Sign(ctx, &kms.SignInput{
		KeyId:            aws.String(s.kmsKeyARN),
		Message:          digest[:],
		MessageType:      types.MessageTypeDigest,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	}, withoutRetry)
	if err != nil {
		return "", classifyKMSFailure(err)
	}
	if len(out.Signature) == 0 {
		// AWS SDK は 200 の応答に署名がなくてもエラーにしない
		return "", fmt.Errorf("%w: kms returned an empty signature", ErrInternalFailure)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(out.Signature), nil
}

// withoutRetry は、AWS SDK の既定の再試行を止める。
// クライアントを作る側の設定に頼らず呼び出しごとに止めるのは、1 回の署名を待つ上限を KMS Signer だけで守るためである。
func withoutRetry(o *kms.Options) {
	o.Retryer = aws.NopRetryer{}
}

// NewKMSClient は、KMS Signer に渡す KMS のクライアントを、要求ごとに接続を張るように作る。理由は Lambda Design の Call Time Budget にある。
// 接続を使い回すかは HTTP クライアントの Transport が決めるため、withoutRetry と違い、署名のたびではなくクライアントを作るときに設定する。
// cfg の HTTPClient は、未設定か BuildableClient であることを前提にする。LoadDefaultConfig が読み込んだ設定はこれを満たす。
// kms.NewFromConfig は、optFns を適用する前に、未設定の HTTPClient を BuildableClient にし、defaults mode とサービスに応じたタイムアウトを重ねる。
// その設定を捨てないよう、新しいクライアントに置き換えずに重ねる。
func NewKMSClient(cfg aws.Config) *kms.Client {
	return kms.NewFromConfig(cfg, func(o *kms.Options) {
		o.HTTPClient = o.HTTPClient.(*awshttp.BuildableClient).WithTransportOptions(func(transport *http.Transport) {
			transport.DisableKeepAlives = true
		})
	})
}

// signingInput は、JOSE Header と Payload を Base64URL にして連結した署名入力を返す。
// encodedClientID は JSON の文字列で、時刻は整数のため、組み立てた Payload は常に JSON として正しい。
func signingInput(encodedClientID jsontext.Value, now time.Time) string {
	payload := fmt.Appendf(nil, `{"iss":%s,"iat":%d,"exp":%d}`, encodedClientID, now.Add(-issuedAtSkew).Unix(), now.Add(lifetime).Unix())
	return encodedHeader + "." + base64.RawURLEncoding.EncodeToString(payload)
}
