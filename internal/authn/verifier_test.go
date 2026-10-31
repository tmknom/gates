package authn

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/request"
)

const (
	testIssuer   = "https://token.actions.githubusercontent.com"
	testAudience = "gates"
	testKeyID    = "test-key"
)

// testNow は validClaims の iat と同じ時刻で、実時間に依存せず時間条件の境界を確かめるために固定する。
var testNow = time.Unix(1700000000, 0)

func testClock() time.Time { return testNow }

// testKey は、鍵の生成が遅いため 1 回だけ作る。
var testKey = sync.OnceValue(generateKey)

func generateKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

func newTestVerifier() *Verifier {
	keys := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{testKey().Public()}}
	return newVerifier(testIssuer, testAudience, keys, testClock)
}

// signPayload は Claim を JSON のまま受ける。重複したキーのように、map から作れない Payload にも署名するため。
func signPayload(t *testing.T, algorithm jose.SignatureAlgorithm, key *rsa.PrivateKey, payload []byte) string {
	t.Helper()
	return signWithKeyID(t, algorithm, key, testKeyID, payload)
}

// signWithKeyID は keyID をヘッダーの kid に書く。keyID が空なら kid のない JWT になる。
func signWithKeyID(t *testing.T, algorithm jose.SignatureAlgorithm, key *rsa.PrivateKey, keyID string, payload []byte) string {
	t.Helper()
	signingKey := jose.SigningKey{Algorithm: algorithm, Key: jose.JSONWebKey{Key: key, KeyID: keyID}}
	signer, err := jose.NewSigner(signingKey, nil)
	if err != nil {
		t.Fatalf("NewSigner returned error: %v", err)
	}
	signed, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	token, err := signed.CompactSerialize()
	if err != nil {
		t.Fatalf("CompactSerialize returned error: %v", err)
	}
	return token
}

func signClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	return signPayload(t, jose.RS256, testKey(), marshalClaims(t, claims))
}

// signLeadingClaims は leading の JSON のメンバーを先頭に固定して署名する。
// go-oidc が読む値は大文字小文字だけが異なる名前の順序で変わり、map から作る JSON では順序を決められないため。
func signLeadingClaims(t *testing.T, leading, omitted string) string {
	t.Helper()
	rest := marshalClaims(t, claimsWithout(omitted))[1:]
	return signPayload(t, jose.RS256, testKey(), append([]byte("{"+leading+","), rest...))
}

// escapedClaimName は、先頭の文字を Unicode エスケープで書いた JSON の文字列を返す。エスケープを解いた後の名前で比較されることを確かめるため。
func escapedClaimName(name string) string {
	return fmt.Sprintf(`"%cu%04x%s"`, '\\', name[0], name[1:])
}

func rawIDToken(t *testing.T, token string) request.RawIDToken {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"version":      1,
		"id_token":     token,
		"repositories": []string{"octo/app"},
		"permissions":  map[string]string{"contents": "read"},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	raw, _, err := request.Validate(payload)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return raw
}

func unixAfter(d time.Duration) int64 {
	return testNow.Add(d).Unix()
}

func TestNewVerifier_InvalidConfig(t *testing.T) {
	tests := []struct {
		name     string
		issuer   string
		audience string
		want     error
	}{
		{"empty issuer returns ErrInvalidIssuer", "", testAudience, ErrInvalidIssuer},
		{"http issuer returns ErrInvalidIssuer", "http://issuer.example", testAudience, ErrInvalidIssuer},
		{"issuer without host returns ErrInvalidIssuer", "https://", testAudience, ErrInvalidIssuer},
		{"issuer with port but no host returns ErrInvalidIssuer", "https://:443", testAudience, ErrInvalidIssuer},
		{"unparsable issuer returns ErrInvalidIssuer", "https://[", testAudience, ErrInvalidIssuer},
		{"uppercase scheme returns ErrInvalidIssuer", "HTTPS://issuer.example", testAudience, ErrInvalidIssuer},
		{"issuer with path returns ErrInvalidIssuer", "https://issuer.example/path", testAudience, ErrInvalidIssuer},
		{"issuer with user returns ErrInvalidIssuer", "https://user@issuer.example", testAudience, ErrInvalidIssuer},
		{"issuer with query returns ErrInvalidIssuer", "https://issuer.example?a=b", testAudience, ErrInvalidIssuer},
		{"issuer with fragment returns ErrInvalidIssuer", "https://issuer.example#a", testAudience, ErrInvalidIssuer},
		{"issuer with empty query returns ErrInvalidIssuer", "https://issuer.example?", testAudience, ErrInvalidIssuer},
		{"issuer with empty port returns ErrInvalidIssuer", "https://issuer.example:", testAudience, ErrInvalidIssuer},
		{"empty audience returns ErrInvalidAudience", testIssuer, "", ErrInvalidAudience},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVerifier(tt.issuer, tt.audience, testClock)
			if !errors.Is(err, tt.want) {
				t.Fatalf("NewVerifier error = %v, want %v", err, tt.want)
			}
			if got != nil {
				t.Errorf("NewVerifier = %+v, want nil", got)
			}
		})
	}
}

func TestVerifierAuthenticate(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]any
	}{
		{"all claims present", validClaims()},
		{"aud as one-element array", claimsWith(map[string]any{"aud": []string{testAudience}})},
		{"exp at numeric date upper limit", claimsWith(map[string]any{"exp": 253402300799})},
		{"iat at numeric date lower limit", claimsWith(map[string]any{"iat": 0})},
		{"nbf missing", claimsWithout("nbf")},
		{"nbf at clock skew limit", claimsWith(map[string]any{"nbf": unixAfter(5 * time.Minute)})},
		{"iat at clock skew limit", claimsWith(map[string]any{"iat": unixAfter(5 * time.Minute)})},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns Caller", func(t *testing.T) {
			got, err := newTestVerifier().Authenticate(t.Context(), rawIDToken(t, signClaims(t, tt.claims)))
			if err != nil {
				t.Fatalf("Authenticate returned error: %v", err)
			}
			if want := newCaller(t, 11, 22, 33, 2, validAttributes()); got != want {
				t.Errorf("Authenticate = %+v, want %+v", got, want)
			}
		})
	}
}

func TestVerifierAuthenticate_InvalidToken(t *testing.T) {
	tests := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{"tampered payload", func(t *testing.T) string {
			s := strings.Split(signClaims(t, validClaims()), ".")
			// 署名の検証だけが拒否できるよう、ほかの規則に触れない Claim を書き換える
			tampered := marshalClaims(t, claimsWith(map[string]any{"run_id": "99"}))
			return s[0] + "." + base64.RawURLEncoding.EncodeToString(tampered) + "." + s[2]
		}},
		{"signature by another key", func(t *testing.T) string {
			return signPayload(t, jose.RS256, generateKey(), marshalClaims(t, validClaims()))
		}},
		{"alg RS384", func(t *testing.T) string {
			return signPayload(t, jose.RS384, testKey(), marshalClaims(t, validClaims()))
		}},
		{"different iss", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"iss": "https://other.example"}))
		}},
		{"different aud", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"aud": "other"}))
		}},
		{"aud with another value", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"aud": []string{testAudience, "other"}}))
		}},
		{"aud missing", func(t *testing.T) string {
			return signClaims(t, claimsWithout("aud"))
		}},
		{"exp equal to current time", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"exp": unixAfter(0)}))
		}},
		{"exp one second ago", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"exp": unixAfter(-time.Second)}))
		}},
		{"exp missing", func(t *testing.T) string {
			return signClaims(t, claimsWithout("exp"))
		}},
		{"nbf beyond clock skew", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"nbf": unixAfter(5*time.Minute + time.Second)}))
		}},
		{"iat missing", func(t *testing.T) string {
			return signClaims(t, claimsWithout("iat"))
		}},
		{"iat beyond clock skew", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"iat": unixAfter(5*time.Minute + time.Second)}))
		}},
		{"segments not Base64URL", func(*testing.T) string { return "malformed!header.malformed!payload.malformed!signature" }},
		{"claims rejected by caller rules", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"sub": "repo:octo@99/app@22:ref:refs/heads/main"}))
		}},
		// 検証ライブラリは後の値を読み、aud の検証を通る。Claim の組の復号が重複そのものを拒否する
		{"duplicate aud keys", func(t *testing.T) string {
			return signLeadingClaims(t, `"aud":"other","aud":"`+testAudience+`"`, "aud")
		}},
		{"duplicate exp keys after unescaping", func(t *testing.T) string {
			return signLeadingClaims(t, `"exp":1700000300,`+escapedClaimName("exp")+`:1700000300`, "exp")
		}},
		{"invalid UTF-8 in a claim not read for Caller", func(t *testing.T) string {
			return signLeadingClaims(t, "\"jti\":\"\xff\"", "jti")
		}},
		{"only EXP", func(t *testing.T) string {
			return signLeadingClaims(t, `"EXP":1700000300`, "exp")
		}},
		{"only IAT", func(t *testing.T) string {
			return signLeadingClaims(t, `"IAT":1700000000`, "iat")
		}},
		{"only ISS", func(t *testing.T) string {
			return signLeadingClaims(t, `"ISS":"`+testIssuer+`"`, "iss")
		}},
		{"only AUD", func(t *testing.T) string {
			return signLeadingClaims(t, `"AUD":"`+testAudience+`"`, "aud")
		}},
		{"AUD after a different aud", func(t *testing.T) string {
			return signLeadingClaims(t, `"aud":"other","AUD":"`+testAudience+`"`, "aud")
		}},
		{"escaped AUD after a different aud", func(t *testing.T) string {
			return signLeadingClaims(t, `"aud":"other",`+escapedClaimName("AUD")+`:"`+testAudience+`"`, "aud")
		}},
		// 別名の値を正しい名前と同じにし、別名の拒否だけが拒否できるようにする
		{"iss with ISS", func(t *testing.T) string {
			return signLeadingClaims(t, `"iss":"`+testIssuer+`","ISS":"`+testIssuer+`"`, "iss")
		}},
		{"exp with EXP", func(t *testing.T) string {
			return signLeadingClaims(t, `"exp":1700000300,"EXP":1700000300`, "exp")
		}},
		{"iat with IAT", func(t *testing.T) string {
			return signLeadingClaims(t, `"iat":1700000000,"IAT":1700000000`, "iat")
		}},
		{"nbf with NBF", func(t *testing.T) string {
			return signLeadingClaims(t, `"nbf":1699999700,"NBF":1699999700`, "nbf")
		}},
		// U+017F は s と同じ大文字小文字の対応に属し、encoding/json (v1) は iss として読む
		{"iss with non-ASCII alias", func(t *testing.T) string {
			return signLeadingClaims(t, `"iss":"`+testIssuer+`","iſſ":"`+testIssuer+`"`, "iss")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrAuthenticationFailure", func(t *testing.T) {
			assertAuthenticationFailure(t, tt.token(t))
		})
	}
}

// 文字列、小数、指数表記の値は、go-oidc が読むと 3 つの Claim のどれでも時間条件を通る時刻にし、型の判定だけが拒否できるようにする。
// 範囲の外の値は、Claim によっては go-oidc や iat の判定でも拒否される。
func TestVerifierAuthenticate_InvalidNumericDate(t *testing.T) {
	literals := []struct {
		name    string
		literal string
	}{
		{"numeric string", `"1700000100"`},
		{"null", `null`},
		{"true", `true`},
		{"fraction", `1700000100.5`},
		{"exponent", `1.7000001e9`},
		{"negative", `-1`},
		{"one above upper limit", `253402300800`},
		{"above int64", `9223372036854775808`},
	}
	for _, name := range []string{"exp", "iat", "nbf"} {
		for _, tt := range literals {
			t.Run(name+" "+tt.name+" returns ErrAuthenticationFailure", func(t *testing.T) {
				assertAuthenticationFailure(t, signLeadingClaims(t, `"`+name+`":`+tt.literal, name), tt.literal)
			})
		}
	}
}

func assertAuthenticationFailure(t *testing.T, token string, values ...string) {
	t.Helper()
	got, err := newTestVerifier().Authenticate(t.Context(), rawIDToken(t, token))
	if !errors.Is(err, ErrAuthenticationFailure) {
		t.Fatalf("Authenticate error = %v, want ErrAuthenticationFailure", err)
	}
	if got != (authz.Caller{}) {
		t.Errorf("Authenticate = %+v, want zero Caller", got)
	}

	// oidc: は検証ライブラリの文言の接頭辞で、値を含まない文言が混ざったときにも検出するために入れる
	forbidden := append(strings.Split(token, "."), "oidc:", testIssuer, "octo@", "other.example", "EXP", "IAT", "ISS", "AUD", "NBF", "ſ")
	for _, value := range append(forbidden, values...) {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err.Error(), value)
		}
	}
}
