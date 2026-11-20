package authn

import (
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"

	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/claimsset"
)

const testIssuer = "https://token.actions.githubusercontent.com"

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
		{"aws sts audience returns ErrInvalidAudience", testIssuer, "sts.amazonaws.com", ErrInvalidAudience},
		{"github host audience returns ErrInvalidAudience", testIssuer, "https://github.com", ErrInvalidAudience},
		{"github host audience in other case returns ErrInvalidAudience", testIssuer, "HTTPS://GitHub.COM", ErrInvalidAudience},
		{"github host audience with trailing slash returns ErrInvalidAudience", testIssuer, "https://github.com/", ErrInvalidAudience},
		{"github owner url audience returns ErrInvalidAudience", testIssuer, "https://github.com/octocat", ErrInvalidAudience},
		{"github repository url audience in other case returns ErrInvalidAudience", testIssuer, "HTTPS://GitHub.COM/octocat/example-repo", ErrInvalidAudience},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVerifier(tt.issuer, tt.audience, testClock, discardLog)
			if !errors.Is(err, tt.want) {
				t.Fatalf("NewVerifier error = %v, want %v", err, tt.want)
			}
			if got != nil {
				t.Errorf("NewVerifier = %+v, want nil", got)
			}
		})
	}
}

// U+017F（ſ）は Unicode の大文字小文字の畳み込みでは s と等しいが、ASCII の範囲の比較では等しくない
func TestNewVerifier_AudienceGitHubURLOnlyUnderUnicodeCaseFolding(t *testing.T) {
	for _, audience := range []string{"http\u017f://github.com", "http\u017f://github.com/octocat"} {
		if _, err := NewVerifier(testIssuer, audience, testClock, discardLog); err != nil {
			t.Errorf("NewVerifier(%q) error = %v, want nil", audience, err)
		}
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
		{"unknown claims", claimsWith(map[string]any{"repo_property_pci_compliant": "true", "unknown": map[string]any{"nested": []int{1}}})},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns Caller", func(t *testing.T) {
			got, _, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedOIDCJWT(t, signClaims(t, tt.claims)))
			if err != nil {
				t.Fatalf("Authenticate returned error: %v", err)
			}
			if want := newCaller(t, 11, 22, 33, 2, validAttributes()); got != want {
				t.Errorf("Authenticate = %+v, want %+v", got, want)
			}
		})
	}
}

// iss は go-oidc が、sub は Caller の規則が読むため、両方の読み取りでエスケープを解いた名前を確かめる。
//
//trace:checks GOJ-CLAIM-NAME-MATCH OIDCC-STRING-UNESCAPE
func TestVerifierAuthenticate_EscapedClaimName(t *testing.T) {
	for _, name := range []string{"iss", "sub"} {
		t.Run("escaped "+name+" returns Caller", func(t *testing.T) {
			value, err := json.Marshal(validClaims()[name])
			if err != nil {
				t.Fatalf("Marshal returned error: %v", err)
			}
			token := signLeadingClaims(t, escapedName(name)+":"+string(value), name)

			got, _, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedOIDCJWT(t, token))
			if err != nil {
				t.Fatalf("Authenticate returned error: %v", err)
			}
			if want := newCaller(t, 11, 22, 33, 2, validAttributes()); got != want {
				t.Errorf("Authenticate = %+v, want %+v", got, want)
			}
		})
	}
}

//trace:checks AC-AUTHN-002 AC-AUTHN-004
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
			return signPayload(t, jose.RS256, generateKey(t), marshalClaims(t, validClaims()))
		}},
		{"alg RS384", func(t *testing.T) string {
			return signPayload(t, jose.RS384, testKey(t), marshalClaims(t, validClaims()))
		}},
		{"different iss", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"iss": "https://other.example"}))
		}},
		{"iss missing", func(t *testing.T) string {
			return signClaims(t, claimsWithout("iss"))
		}},
		// 型の判定だけが拒否できるよう、寛容に読めば正しい Issuer になる値にする
		{"iss as one-element array", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"iss": []string{testIssuer}}))
		}},
		{"different aud", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"aud": "other"}))
		}},
		{"aud with another value", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"aud": []string{testAudience, "other"}}))
		}},
		{"aud as empty array", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"aud": []string{}}))
		}},
		{"aud missing", func(t *testing.T) string {
			return signClaims(t, claimsWithout("aud"))
		}},
		// 型の判定だけが拒否できるよう、寛容に読めば GATES 用 Audience になる値にする
		{"aud array with an array element", func(t *testing.T) string {
			return signClaims(t, claimsWith(map[string]any{"aud": []any{[]string{testAudience}}}))
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
			return signLeadingClaims(t, `"exp":1700000300,`+escapedName("exp")+`:1700000300`, "exp")
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
			return signLeadingClaims(t, `"aud":"other",`+escapedName("AUD")+`:"`+testAudience+`"`, "aud")
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
//
//trace:checks AC-AUTHN-004
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

// testdata/observed は、GitHub OIDC JWT の研究文書が記録した実測の Payload で、署名だけをテストの鍵で付け直す。
// 実測は Audience を指定せずに取得したため、既定の Audience を GATES 用 Audience として設定する。
func TestVerifierAuthenticate_ObservedClaims(t *testing.T) {
	for _, name := range []string{"push", "pull_request", "workflow_dispatch", "absolute_reusable_workflow", "relative_reusable_workflow", "environment"} {
		t.Run(name+" returns Caller", func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join("testdata", "observed", name+".json"))
			if err != nil {
				t.Fatalf("ReadFile returned error: %v", err)
			}
			var observed struct {
				IssuedAt       int64  `json:"iat"`
				RunID          string `json:"run_id"`
				RunAttempt     string `json:"run_attempt"`
				Actor          string `json:"actor"`
				ActorID        string `json:"actor_id"`
				EventName      string `json:"event_name"`
				Ref            string `json:"ref"`
				SHA            string `json:"sha"`
				WorkflowRef    string `json:"workflow_ref"`
				JobWorkflowRef string `json:"job_workflow_ref"`
				Environment    string `json:"environment"`
			}
			if err := json.Unmarshal(payload, &observed); err != nil {
				t.Fatalf("Unmarshal returned error: %v", err)
			}
			want := newCaller(t, 583231, 1234567890, parseObservedID(t, observed.RunID), parseObservedID(t, observed.RunAttempt), caller.WorkflowTriggerAttributes{
				Actor:          observed.Actor,
				ActorID:        observed.ActorID,
				EventName:      observed.EventName,
				Ref:            observed.Ref,
				SHA:            observed.SHA,
				WorkflowRef:    observed.WorkflowRef,
				JobWorkflowRef: observed.JobWorkflowRef,
				Environment:    observed.Environment,
			})
			keys := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{testKey(t).Public()}}
			// formal/z3 の実測の例と同じく、発行の 1 秒後に検証する
			clock := func() time.Time { return time.Unix(observed.IssuedAt+1, 0) }
			verifier := newVerifier(testIssuer, "https://github.com/octocat", keys, clock)
			got, _, err := verifier.Authenticate(t.Context(), unverifiedOIDCJWT(t, signPayload(t, jose.RS256, testKey(t), payload)))
			if err != nil {
				t.Fatalf("Authenticate returned error: %v", err)
			}
			if got != want {
				t.Errorf("Authenticate = %+v, want %+v", got, want)
			}
		})
	}
}

// 単純な Case Folding で iss と一致しない名前は、未知の Claim として受理する。
// 値を iss と異ならせて最後のメンバーに置き、デコーダが iss として読めば Issuer の不一致で拒否されるようにする。
func TestVerifierAuthenticate_NonAliasClaimName(t *testing.T) {
	tests := []struct {
		name string
	}{
		// 完全な Case Folding では ss に畳み込まれるが、単純な Case Folding では iss と一致しない
		{"iß"},
		// トルコ語用の対応でだけ i に畳み込まれ、小文字への変換でも iss になる
		{"İss"},
		// Case Folding では i に畳み込まれないが、大文字への変換では ISS になる
		{"ıss"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns Caller", func(t *testing.T) {
			payload := validClaimsAnd(t, `"`+tt.name+`":"https://evil.example"`)
			got, _, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedOIDCJWT(t, signPayload(t, jose.RS256, testKey(t), []byte(payload))))
			if err != nil {
				t.Fatalf("Authenticate returned error: %v", err)
			}
			if want := newCaller(t, 11, 22, 33, 2, validAttributes()); got != want {
				t.Errorf("Authenticate = %+v, want %+v", got, want)
			}
		})
	}
}

//trace:checks AC-AUTHN-004
func TestVerifierAuthenticate_ClaimAlias(t *testing.T) {
	for alias, name := range claimAliases(func(r rune) []rune { return claimAliasLetters[r] }) {
		t.Run(alias+" with "+name+" returns ErrAuthenticationFailure", func(t *testing.T) {
			// 別名の値を正しい名前と同じにし、別名の拒否だけが拒否できるようにする
			value, err := json.Marshal(validClaims()[name])
			if err != nil {
				t.Fatalf("Marshal returned error: %v", err)
			}
			payload := validClaimsAnd(t, `"`+alias+`":`+string(value))
			assertAuthenticationFailure(t, signPayload(t, jose.RS256, testKey(t), []byte(payload)))
		})
	}
}

// TestVerifierAuthenticate_ClaimsSet は、成功した認証が、Caller に読まない Claim を含む Payload 全体だけを署名検証済みの Claims Set として返すことを確かめる。値の一致は、署名や JWT の文字列が加わらないことも確かめる。
//
//trace:checks AC-OPS-009
func TestVerifierAuthenticate_ClaimsSet(t *testing.T) {
	claims := claimsWith(map[string]any{"head_ref": "feature", "base_ref": "main", "repo_property_tier": "gold"})
	token := signClaims(t, claims)

	_, got, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedOIDCJWT(t, token))
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}

	var want map[string]any
	if err := json.Unmarshal(marshalClaims(t, claims), &want); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if decoded := decodedClaimsSet(t, got); !reflect.DeepEqual(decoded, want) {
		t.Errorf("claims set = %v, want %v", decoded, want)
	}
}

// TestVerifierAuthenticate_ClaimsSetOfRejectedToken は、署名検証の後に拒否した JWT だけが署名検証済みの Claims Set を返すことを確かめる。
// 署名検証の後の拒否は、go-oidc の判定、Authenticate の判定、Caller の確定の判定から 1 つずつ選ぶ。
//
//trace:checks AC-OPS-009
func TestVerifierAuthenticate_ClaimsSetOfRejectedToken(t *testing.T) {
	tests := []struct {
		name   string
		claims map[string]any
		token  func(t *testing.T, payload []byte) string
		want   bool
	}{
		{
			name:   "different iss rejected by go-oidc after signature verification returns the claims set",
			claims: claimsWith(map[string]any{"iss": "https://other.example"}),
			token:  signedByTestKey,
			want:   true,
		},
		{
			name:   "exp equal to current time rejected after signature verification returns the claims set",
			claims: claimsWith(map[string]any{"exp": unixAfter(0)}),
			token:  signedByTestKey,
			want:   true,
		},
		{
			name:   "sub rejected by caller rules returns the claims set",
			claims: claimsWith(map[string]any{"sub": "repo:octo@99/app@22:ref:refs/heads/main"}),
			token:  signedByTestKey,
			want:   true,
		},
		{
			name:   "signature by another key returns no claims set",
			claims: validClaims(),
			token: func(t *testing.T, payload []byte) string {
				return signPayload(t, jose.RS256, generateKey(t), payload)
			},
		},
		{
			name:   "claim name alias rejected before signature verification returns no claims set",
			claims: claimsWith(map[string]any{"ISS": testIssuer}),
			token:  signedByTestKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := marshalClaims(t, tt.claims)

			_, got, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedOIDCJWT(t, tt.token(t, payload)))

			if !errors.Is(err, ErrAuthenticationFailure) {
				t.Fatalf("Authenticate error = %v, want ErrAuthenticationFailure", err)
			}
			if !tt.want {
				if !got.IsZero() {
					t.Errorf("claims set = %v, want zero", decodedClaimsSet(t, got))
				}
				return
			}
			var want map[string]any
			if err := json.Unmarshal(payload, &want); err != nil {
				t.Fatalf("Unmarshal returned error: %v", err)
			}
			if got.IsZero() {
				t.Fatalf("claims set is zero, want %v", want)
			}
			if decoded := decodedClaimsSet(t, got); !reflect.DeepEqual(decoded, want) {
				t.Errorf("claims set = %v, want %v", decoded, want)
			}
		})
	}
}

// payloadKeySet は、署名の検証に成功したものとして、決まった Payload を返す公開鍵の集合である。
type payloadKeySet struct {
	payload []byte
}

func (k payloadKeySet) VerifySignature(context.Context, string) ([]byte, error) {
	return k.payload, nil
}

// Authenticate は JSON オブジェクトでない Claims Set を署名検証の前に拒否するため、keySetRecorder に届く経路は公開鍵の集合を差し替えて作る。
func TestKeySetRecorder_RejectsPayloadThatIsNotClaimsSet(t *testing.T) {
	recorder := &keySetRecorder{keys: payloadKeySet{payload: []byte(`[]`)}}

	_, err := recorder.VerifySignature(t.Context(), "")

	if !errors.Is(err, ErrAuthenticationFailure) {
		t.Errorf("VerifySignature error = %v, want ErrAuthenticationFailure", err)
	}
	if !recorder.claimsSet.IsZero() {
		t.Errorf("claims set is not zero for a payload that is not a claims set")
	}
}

func newTestVerifier(t *testing.T) *Verifier {
	t.Helper()
	keys := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{testKey(t).Public()}}
	return newVerifier(testIssuer, testAudience, keys, testClock)
}

// signPayload は Claim を JSON のまま受ける。重複したキーのように、map から作れない Payload にも署名するため。
func signPayload(t *testing.T, algorithm jose.SignatureAlgorithm, key *rsa.PrivateKey, payload []byte) string {
	t.Helper()
	return signWithKeyID(t, algorithm, key, testKeyID, payload)
}

func signClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	return signPayload(t, jose.RS256, testKey(t), marshalClaims(t, claims))
}

// signLeadingClaims は leading の JSON のメンバーを先頭に固定して署名する。
// go-oidc が読む値は別名との順序で変わり、map から作る JSON では順序を決められないため。
func signLeadingClaims(t *testing.T, leading, omitted string) string {
	t.Helper()
	rest := marshalClaims(t, claimsWithout(omitted))[1:]
	return signPayload(t, jose.RS256, testKey(t), append([]byte("{"+leading+","), rest...))
}

func unixAfter(d time.Duration) int64 {
	return testNow.Add(d).Unix()
}

// parseObservedID は、実測の Payload の ID の文字列を、Caller と比べるために整数に変換する。
func parseObservedID(t *testing.T, id string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		t.Fatalf("ParseInt(%q) returned error: %v", id, err)
	}
	return n
}

func assertAuthenticationFailure(t *testing.T, token string, values ...string) {
	t.Helper()
	got, _, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedOIDCJWT(t, token))
	if !errors.Is(err, ErrAuthenticationFailure) {
		t.Fatalf("Authenticate error = %v, want ErrAuthenticationFailure", err)
	}
	if got != (caller.Caller{}) {
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

// decodedClaimsSet は、署名検証済みの Claims Set の JSON を、Payload と比べられる値に復号する。
func decodedClaimsSet(t *testing.T, claimsSet claimsset.SignatureVerified) map[string]any {
	t.Helper()
	b, err := claimsSet.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v; claims set = %s", err, b)
	}
	return got
}

func signedByTestKey(t *testing.T, payload []byte) string {
	t.Helper()
	return signPayload(t, jose.RS256, testKey(t), payload)
}
