package authn

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
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
)

// testNow は validClaims の iat と同じ時刻で、実時間に依存せず時間条件の境界を確かめるために固定する。
var testNow = time.Unix(1700000000, 0)

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
	return NewVerifier(testIssuer, testAudience, keys, func() time.Time { return testNow })
}

// signPayload は Claim を JSON のまま受ける。重複したキーのように、map から作れない Payload にも署名するため。
func signPayload(t *testing.T, algorithm jose.SignatureAlgorithm, key *rsa.PrivateKey, payload []byte) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: algorithm, Key: key}, nil)
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

func TestVerifierAuthenticate(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]any
	}{
		{"all claims present", validClaims()},
		{"aud as one-element array", claimsWith(map[string]any{"aud": []string{testAudience}})},
		{"exp equal to current time", claimsWith(map[string]any{"exp": unixAfter(0)})},
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
		// 検証ライブラリは後の値を読み、aud の検証を通る。Caller を確定する復号が重複そのものを拒否する
		{"duplicate aud keys", func(t *testing.T) string {
			rest := marshalClaims(t, claimsWithout("aud"))[1:]
			payload := append([]byte(`{"aud":"other","aud":"`+testAudience+`",`), rest...)
			return signPayload(t, jose.RS256, testKey(), payload)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrAuthenticationFailure", func(t *testing.T) {
			token := tt.token(t)

			got, err := newTestVerifier().Authenticate(t.Context(), rawIDToken(t, token))
			if !errors.Is(err, ErrAuthenticationFailure) {
				t.Fatalf("Authenticate error = %v, want ErrAuthenticationFailure", err)
			}
			if got != (authz.Caller{}) {
				t.Errorf("Authenticate = %+v, want zero Caller", got)
			}

			// oidc: は検証ライブラリの文言の接頭辞で、値を含まない文言が混ざったときにも検出するために入れる
			forbidden := append(strings.Split(token, "."), "oidc:", testIssuer, "octo@", "other.example")
			for _, value := range forbidden {
				if strings.Contains(err.Error(), value) {
					t.Errorf("error message %q contains %q", err.Error(), value)
				}
			}
		})
	}
}
