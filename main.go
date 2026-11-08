// Command gates は Runtime Entry で、固定設定を読んで部品を組み立て、Lambda Handler を Lambda Runtime に渡す。
// 責務と初期化の失敗の扱いは Lambda Design の「Runtime Entry」に従う。
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"

	"github.com/tmknom/gates/internal/authn"
	"github.com/tmknom/gates/internal/config"
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/handler"
	"github.com/tmknom/gates/internal/signer"
	"github.com/tmknom/gates/internal/tokenissuer"
)

// oidcIssuer は固定設定の Issuer である。値の理由は Lambda Design の「OIDC Validation」に記述する。
const oidcIssuer = "https://token.actions.githubusercontent.com"

// errAWSConfig は、AWS SDK の設定を読めなかったことだけを表す。SDK の文言は初期化の失敗のログに書かない。
var errAWSConfig = errors.New("failed to load the AWS SDK configuration")

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	entry, err := newRuntimeEntry(context.Background(), log)
	if err != nil {
		log.Error("initialization failed", "error", err)
		os.Exit(1)
	}
	lambda.Start(entry)
}

// runtimeEntry は、Lambda Runtime から Payload と相関 ID を受け取り、Lambda Handler に渡す。
type runtimeEntry struct {
	handler *handler.Handler
}

// lambda.Start は型で Handler を見分けて Payload をバイト列のまま渡すため、満たしていることを静的に確かめる。
var _ lambda.Handler = runtimeEntry{}

func newRuntimeEntry(ctx context.Context, log *slog.Logger) (runtimeEntry, error) {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return runtimeEntry{}, err
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return runtimeEntry{}, errAWSConfig
	}
	verifier, err := authn.NewVerifier(oidcIssuer, cfg.Audience(), time.Now, log)
	if err != nil {
		return runtimeEntry{}, err
	}
	kmsSigner, err := signer.NewSigner(cfg.ClientID(), cfg.KMSKeyARN(), kms.NewFromConfig(awsCfg), time.Now)
	if err != nil {
		return runtimeEntry{}, err
	}
	tokenIssuer := tokenissuer.NewTokenIssuer(kmsSigner, github.NewClient(), cfg.AllowedOwner(), time.Now)
	return runtimeEntry{handler: handler.NewHandler(verifier, tokenIssuer, cfg.AllowedOwner(), log)}, nil
}

// Invoke は、Lambda Handler の応答を返す。error を返すのは、相関 ID が UTF-8 として正しくなく、発行処理を始めない場合だけである。
func (e runtimeEntry) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	// lambda.Start は Invoke を呼ぶ前に必ず ctx に LambdaContext を入れるため、入っていない場合を扱わない
	lc, _ := lambdacontext.FromContext(ctx)
	requestID, err := handler.NewRequestID(lc.AwsRequestID)
	if err != nil {
		return nil, err
	}
	return e.handler.Handle(ctx, requestID, payload), nil
}
