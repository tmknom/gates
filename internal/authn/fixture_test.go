package authn

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"

	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/request"
)

const (
	testAudience = "gates"
	testKeyID    = "test-key"
)

// testNow は validClaims の iat と同じ時刻で、実時間に依存せず時間条件の境界を確かめるために固定する。
var testNow = time.Unix(1700000000, 0)

// discardLog は、ログを確かめないテストが Verifier に渡す logger である。
var discardLog = slog.New(slog.DiscardHandler)

// sharedKey は、鍵の生成が遅いため 1 回だけ作る。
var sharedKey = sync.OnceValues(func() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
})

// claimAliasLetters は、GOJ-CLAIM-NAME-ALIAS が別名の範囲として定める、別名を持つ 5 つの Claim 名の各文字と同じ文字に畳み込まれる文字の組である。
var claimAliasLetters = map[rune][]rune{
	'a': []rune("aA"), 'b': []rune("bB"), 'd': []rune("dD"), 'e': []rune("eE"),
	'f': []rune("fF"), 'i': []rune("iI"), 'n': []rune("nN"), 'p': []rune("pP"),
	's': []rune("sSſ"), 't': []rune("tT"), 'u': []rune("uU"), 'x': []rune("xX"),
}

// validClaims は、実 JWT と同じく Caller の確定に読まない Claim も含む、正当な Claim の組を返す。
func validClaims() map[string]any {
	return map[string]any{
		"iss":                 "https://token.actions.githubusercontent.com",
		"aud":                 "gates",
		"exp":                 1700000300,
		"iat":                 1700000000,
		"nbf":                 1699999700,
		"jti":                 "example-jti",
		"sub":                 "repo:octo@11/app@22:ref:refs/heads/main",
		"repository_owner_id": "11",
		"repository_id":       "22",
		"run_id":              "33",
		"run_attempt":         "2",
		"actor":               "octocat",
		"actor_id":            "583231",
		"event_name":          "push",
		"ref":                 "refs/heads/main",
		"sha":                 "ffac537e6cbbf934b08745a378932722df287a53",
		"workflow_ref":        "octo/app/.github/workflows/ci.yml@refs/heads/main",
		"job_workflow_ref":    "octo/app/.github/workflows/job.yml@refs/heads/main",
		"environment":         "production",
	}
}

func validAttributes() caller.WorkflowTriggerAttributes {
	return caller.WorkflowTriggerAttributes{
		Actor:          "octocat",
		ActorID:        "583231",
		EventName:      "push",
		Ref:            "refs/heads/main",
		SHA:            "ffac537e6cbbf934b08745a378932722df287a53",
		WorkflowRef:    "octo/app/.github/workflows/ci.yml@refs/heads/main",
		JobWorkflowRef: "octo/app/.github/workflows/job.yml@refs/heads/main",
		Environment:    "production",
	}
}

func marshalClaims(t *testing.T, claims map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return b
}

func claimsWith(overrides map[string]any) map[string]any {
	claims := validClaims()
	maps.Copy(claims, overrides)
	return claims
}

func claimsWithout(names ...string) map[string]any {
	claims := validClaims()
	for _, name := range names {
		delete(claims, name)
	}
	return claims
}

func newCaller(t *testing.T, ownerID, repositoryID, runID, runAttempt int64, attributes caller.WorkflowTriggerAttributes) caller.Caller {
	t.Helper()
	c, err := caller.NewCaller(ownerID, repositoryID, runID, runAttempt, attributes)
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	return c
}

// validClaimsAnd は、正当な Claim の組の末尾に JSON のメンバーを足す。復号以外の規則で拒否されない状態で、復号の規則を確かめるため。
func validClaimsAnd(t *testing.T, members string) string {
	t.Helper()
	return strings.TrimSuffix(string(marshalClaims(t, validClaims())), "}") + "," + members + "}"
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

func testClock() time.Time { return testNow }

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

// escapedName は、先頭の文字を Unicode エスケープで書いた JSON の文字列を返す。エスケープを解いた後の名前で比較されることを確かめるため。
func escapedName(name string) string {
	return fmt.Sprintf(`"%cu%04x%s"`, '\\', name[0], name[1:])
}

func unverifiedOIDCJWT(t *testing.T, token string) request.UnverifiedOIDCJWT {
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
