package authn

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"

	"github.com/tmknom/gates/internal/caller"
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

// discardLog は、ログを確かめないテストが Verifier に渡す logger である。
var discardLog = slog.New(slog.DiscardHandler)

// sharedKey は、鍵の生成が遅いため 1 回だけ作る。
var sharedKey = sync.OnceValues(func() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
})

// testKey は、テストが共有する鍵を返す。
func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := sharedKey()
	if err != nil {
		t.Fatalf("rsa.GenerateKey returned error: %v", err)
	}
	return key
}

// generateKey は、共有する鍵と異なる鍵を作る。
func generateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey returned error: %v", err)
	}
	return key
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
	return signPayload(t, jose.RS256, testKey(t), marshalClaims(t, claims))
}

// signLeadingClaims は leading の JSON のメンバーを先頭に固定して署名する。
// go-oidc が読む値は別名との順序で変わり、map から作る JSON では順序を決められないため。
func signLeadingClaims(t *testing.T, leading, omitted string) string {
	t.Helper()
	rest := marshalClaims(t, claimsWithout(omitted))[1:]
	return signPayload(t, jose.RS256, testKey(t), append([]byte("{"+leading+","), rest...))
}

// escapedName は、先頭の文字を Unicode エスケープで書いた JSON の文字列を返す。エスケープを解いた後の名前で比較されることを確かめるため。
func escapedName(name string) string {
	return fmt.Sprintf(`"%cu%04x%s"`, '\\', name[0], name[1:])
}

func unverifiedIDToken(t *testing.T, token string) request.UnverifiedIDToken {
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
			got, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedIDToken(t, signClaims(t, tt.claims)))
			if err != nil {
				t.Fatalf("Authenticate returned error: %v", err)
			}
			if want := newCaller(t, 11, 22, 33, 2, validAttributes()); got != want {
				t.Errorf("Authenticate = %+v, want %+v", got, want)
			}
		})
	}
}

//trace:checks AC-03 AC-27
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
//trace:checks AC-27
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
			got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, signPayload(t, jose.RS256, testKey(t), payload)))
			if err != nil {
				t.Fatalf("Authenticate returned error: %v", err)
			}
			if got != want {
				t.Errorf("Authenticate = %+v, want %+v", got, want)
			}
		})
	}
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
			got, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedIDToken(t, signPayload(t, jose.RS256, testKey(t), []byte(payload))))
			if err != nil {
				t.Fatalf("Authenticate returned error: %v", err)
			}
			if want := newCaller(t, 11, 22, 33, 2, validAttributes()); got != want {
				t.Errorf("Authenticate = %+v, want %+v", got, want)
			}
		})
	}
}

// claimAliasLetters は、GOJ-CLAIM-NAME が別名の範囲として定める、別名を持つ 5 つの Claim 名の各文字と同じ文字に畳み込まれる文字の組である。
var claimAliasLetters = map[rune][]rune{
	'a': []rune("aA"), 'b': []rune("bB"), 'd': []rune("dD"), 'e': []rune("eE"),
	'f': []rune("fF"), 'i': []rune("iI"), 'n': []rune("nN"), 'p': []rune("pP"),
	's': []rune("sSſ"), 't': []rune("tT"), 'u': []rune("uU"), 'x': []rune("xX"),
}

// claimAliases は、各文字を letters の返す文字に置き換えて作れる名前のうち元の名前と異なるものを、元の名前を値とする map で返す。
func claimAliases(letters func(rune) []rune) map[string]string {
	aliases := map[string]string{}
	for _, name := range []string{"iss", "aud", "exp", "iat", "nbf"} {
		spellings := []string{""}
		for _, r := range name {
			var next []string
			for _, prefix := range spellings {
				for _, letter := range letters(r) {
					next = append(next, prefix+string(letter))
				}
			}
			spellings = next
		}
		for _, spelling := range spellings {
			if spelling != name {
				aliases[spelling] = name
			}
		}
	}
	return aliases
}

// simpleFoldOrbit は、r と Unicode の単純な Case Folding で同じ文字に畳み込まれる文字を返す。
func simpleFoldOrbit(r rune) []rune {
	orbit := []rune{r}
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		orbit = append(orbit, f)
	}
	return orbit
}

// claimNameCandidates は、5 つの Claim 名と、その各文字を単純な Case Folding で同じ文字に畳み込まれる文字に置き換えた名前を返す。
// i には、トルコ語用の対応でだけ i と同じになる İ と、i に似るがどの Case Folding でも i にならない ı も充てる。
// 完全な Case Folding でだけ ss と同じになる iß も加える。
func claimNameCandidates() []string {
	letters := func(r rune) []rune {
		if r == 'i' {
			return append(simpleFoldOrbit(r), 'İ', 'ı')
		}
		return simpleFoldOrbit(r)
	}
	names := []string{"iss", "aud", "exp", "iat", "nbf", "iß"}
	for name := range claimAliases(letters) {
		names = append(names, name)
	}
	return names
}

// decodedClaimName は、go-oidc が Claim を読む encoding/json (v1) が、name を 5 つの Claim 名のどれとして読むかを返す。どれとしても読まなければ空文字列を返す。
func decodedClaimName(t *testing.T, name string) string {
	t.Helper()
	var claims struct {
		Iss jsonv1.RawMessage `json:"iss"`
		Aud jsonv1.RawMessage `json:"aud"`
		Exp jsonv1.RawMessage `json:"exp"`
		Iat jsonv1.RawMessage `json:"iat"`
		Nbf jsonv1.RawMessage `json:"nbf"`
	}
	if err := jsonv1.Unmarshal([]byte(`{"`+name+`":0}`), &claims); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	for claimName, value := range map[string]jsonv1.RawMessage{"iss": claims.Iss, "aud": claims.Aud, "exp": claims.Exp, "iat": claims.Iat, "nbf": claims.Nbf} {
		if value != nil {
			return claimName
		}
	}
	return ""
}

// GOJ-CLAIM-NAME が定める別名が、go-oidc のデコーダが 5 つの Claim 名として読む名前とも、isClaimAlias が拒否する名前とも一致することを確かめる。
// デコーダの照合は Go の Unicode の版で広がり得るため、単純な Case Folding で畳み込まれる文字はすべて候補に入れる。
//
//trace:checks AC-27
func TestClaimAliases_MatchDecoderAndRejection(t *testing.T) {
	aliases := claimAliases(func(r rune) []rune { return claimAliasLetters[r] })
	for _, name := range claimNameCandidates() {
		claimName, isAlias := aliases[name]
		if !isAlias && slices.Contains([]string{"iss", "aud", "exp", "iat", "nbf"}, name) {
			claimName = name
		}
		if got := decodedClaimName(t, name); got != claimName {
			t.Errorf("decoder under unicode %s reads %q as %q, want %q", unicode.Version, name, got, claimName)
		}
		if got := isClaimAlias(name); got != isAlias {
			t.Errorf("isClaimAlias(%q) = %v, want %v", name, got, isAlias)
		}
	}
}

//trace:checks AC-27
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

func assertAuthenticationFailure(t *testing.T, token string, values ...string) {
	t.Helper()
	got, err := newTestVerifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))
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
