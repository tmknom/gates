package config

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/repository"
)

// 環境変数の名前は、AWS Infrastructure Design が定める Lambda と Terraform の間の契約である。
const (
	envAllowedOwnerID   = "GATES_ALLOWED_OWNER_ID"
	envAllowedOwnerName = "GATES_ALLOWED_OWNER_NAME"
	envAllowedOwnerType = "GATES_ALLOWED_OWNER_TYPE"
	envClientID         = "GATES_GITHUB_APP_CLIENT_ID"
	envAudience         = "GATES_AUDIENCE"
	envKMSKeyARN        = "GATES_KMS_KEY_ARN"
)

// Config は、固定設定のうち GATES 運用者がデプロイ時に決める値である。
// Client ID、Audience、KMS Key ARN を文字列のまま持つのは、それらの条件を検証器と KMS Signer が作る時点の不変条件として持ち、規則を 2 か所に分けないため。
// フィールドを非公開にし、ゼロ値以外は Load でしか作れないようにする。
type Config struct {
	allowedOwner authz.AllowedOwner
	clientID     string
	audience     string
	kmsKeyARN    string
}

// Load は、環境変数から固定設定を読み、Domain Model の「Invariants」が定める許可オーナーの条件を検証する。
// 欠落と不正な値をすべて集めて返す理由は Lambda Design の「Runtime Entry」に記述する。
func Load(lookupEnv func(key string) (string, bool)) (Config, error) {
	l := loader{lookupEnv: lookupEnv}
	ownerID := l.read(envAllowedOwnerID)
	ownerName := l.read(envAllowedOwnerName)
	ownerType := l.read(envAllowedOwnerType)
	clientID := l.read(envClientID)
	audience := l.read(envAudience)
	kmsKeyARN := l.read(envKMSKeyARN)

	allowedOwner := l.allowedOwner(ownerID, ownerName, ownerType)

	if len(l.errs) > 0 {
		return Config{}, fmt.Errorf("%w: %w", ErrInvalidConfig, errors.Join(l.errs...))
	}
	return Config{allowedOwner: allowedOwner, clientID: clientID.value, audience: audience.value, kmsKeyARN: kmsKeyARN.value}, nil
}

// AllowedOwner は、許可オーナーを返す。
func (c Config) AllowedOwner() authz.AllowedOwner {
	return c.allowedOwner
}

// ClientID は、GitHub App の Client ID を返す。
func (c Config) ClientID() string {
	return c.clientID
}

// Audience は、GATES 用 Audience を返す。
func (c Config) Audience() string {
	return c.audience
}

// KMSKeyARN は、署名鍵の KMS Key ARN を返す。
func (c Config) KMSKeyARN() string {
	return c.kmsKeyARN
}

// envValue は、読んだ環境変数の値である。
// ok が偽の値は、欠落か空として報告済みで、以降の条件の判定に使わない。
type envValue struct {
	key   string
	value string
	ok    bool
}

// loader は、環境変数を読んで検証し、満たさなかった条件を環境変数の名前とともに集める。
type loader struct {
	lookupEnv func(key string) (string, bool)
	errs      []error
}

// read は、欠落と空を分けて報告する。欠落は Terraform との契約の名前の不一致、空は値の誤りと、直す箇所が異なるため。
func (l *loader) read(key string) envValue {
	value, ok := l.lookupEnv(key)
	switch {
	case !ok:
		l.fail(key, errors.New("is not set"))
		return envValue{key: key}
	case value == "":
		l.fail(key, errors.New("must not be empty"))
		return envValue{key: key}
	}
	return envValue{key: key, value: value, ok: true}
}

func (l *loader) fail(key string, err error) {
	l.errs = append(l.errs, fmt.Errorf("%s: %w", key, err))
}

// allowedOwner は、Owner ID、オーナー名、種別の誤りを、ほかの 2 つの成否によらず報告する。
// NewAllowedOwner は最初に見つけた誤りだけを返すため、Owner ID と種別を先に検証し、どちらも正しいときだけ NewAllowedOwner にオーナー名を検証させる。
// どちらかが誤っていれば NewAllowedOwner を呼べないため、オーナー名は ParseOwnerName で検証する。
func (l *loader) allowedOwner(rawID, name, rawType envValue) authz.AllowedOwner {
	id, idOK := l.ownerID(rawID)
	ownerType, typeOK := l.ownerType(rawType)
	switch {
	case !name.ok:
		return authz.AllowedOwner{}
	case !idOK || !typeOK:
		if _, err := repository.ParseOwnerName(name.value); err != nil {
			l.fail(name.key, err)
		}
		return authz.AllowedOwner{}
	}
	owner, err := authz.NewAllowedOwner(id, name.value, ownerType)
	if err != nil {
		l.fail(name.key, err)
	}
	return owner
}

func (l *loader) ownerID(raw envValue) (int64, bool) {
	if !raw.ok {
		return 0, false
	}
	id, err := parseOwnerID(raw.value)
	if err != nil {
		l.fail(raw.key, err)
		return 0, false
	}
	return id, true
}

func (l *loader) ownerType(raw envValue) (authz.OwnerType, bool) {
	if !raw.ok {
		return "", false
	}
	ownerType, err := authz.ParseOwnerType(raw.value)
	if err != nil {
		l.fail(raw.key, err)
		return "", false
	}
	return ownerType, true
}

// parseOwnerID は、符号と先頭の 0 を持たない 10 進表記だけを受け付ける。
// strconv.ParseUint を 63 ビットで使うのは、strconv.ParseInt と違って符号を受理せず、値が int64 に収まるため。
// strconv の error は値を含むため、包まずに条件だけを返す。
func parseOwnerID(s string) (int64, error) {
	id, err := strconv.ParseUint(s, 10, 63)
	if err != nil || s[0] == '0' {
		return 0, errors.New("must be a positive decimal integer without a sign or leading zeros that fits in int64")
	}
	return int64(id), nil
}
