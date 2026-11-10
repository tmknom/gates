package authn

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"maps"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"

	"github.com/tmknom/gates/internal/caller"
)

const (
	discoveryPath = "/.well-known/openid-configuration"
	jwksPath      = "/.well-known/jwks"

	// responseMarker は、応答本文がエラー文言に漏れていないことを確かめる目印である。
	responseMarker = "response-body-marker"
)

// issuerStub は Discovery と JWKS を返す Issuer で、応答の差し替えと取得回数の記録ができる。
type issuerStub struct {
	server            *httptest.Server
	discoveryRequests atomic.Int32
	jwksRequests      atomic.Int32
	newConnections    atomic.Int32
	discovery         atomic.Pointer[http.HandlerFunc]
	jwks              atomic.Pointer[http.HandlerFunc]
	// logs は verifier が作る Verifier のログの出力先である。slog の Handler が書き込みを直列にする
	logs bytes.Buffer
}

func newIssuerStub(t *testing.T, key *rsa.PrivateKey, keyID string) *issuerStub {
	t.Helper()
	stub := &issuerStub{}
	mux := http.NewServeMux()
	mux.HandleFunc(discoveryPath, func(w http.ResponseWriter, r *http.Request) {
		stub.discoveryRequests.Add(1)
		(*stub.discovery.Load())(w, r)
	})
	mux.HandleFunc(jwksPath, func(w http.ResponseWriter, r *http.Request) {
		stub.jwksRequests.Add(1)
		(*stub.jwks.Load())(w, r)
	})
	stub.server = httptest.NewUnstartedServer(mux)
	stub.server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			stub.newConnections.Add(1)
		}
	}
	// 証明書を信頼しないクライアントのテストで、サーバーのログがテストの出力に混ざらないようにする
	stub.server.Config.ErrorLog = log.New(io.Discard, "", 0)
	stub.server.StartTLS()
	t.Cleanup(stub.server.Close)

	stub.respondDiscovery(respondBody(stub.discoveryBody()))
	stub.respondJWKS(respondBody(jwksBody(t, key, keyID)))
	return stub
}

func (s *issuerStub) respondDiscovery(handler http.HandlerFunc) { s.discovery.Store(&handler) }

func (s *issuerStub) respondJWKS(handler http.HandlerFunc) { s.jwks.Store(&handler) }

func (s *issuerStub) issuer() string { return s.server.URL }

func (s *issuerStub) discoveryBody() string {
	return discoveryBodyOf(s.issuer(), s.issuer()+jwksPath)
}

func (s *issuerStub) verifier(t *testing.T) *Verifier {
	t.Helper()
	return s.verifierWithClock(t, testClock)
}

func (s *issuerStub) verifierWithClock(t *testing.T, now func() time.Time) *Verifier {
	t.Helper()
	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(s.server.Certificate())
	verifier, err := NewVerifier(s.issuer(), testAudience, now, slog.New(slog.NewJSONHandler(&s.logs, nil)), WithRootCAs(rootCAs))
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}
	return verifier
}

func (s *issuerStub) documentRequests() int32 {
	return s.discoveryRequests.Load() + s.jwksRequests.Load()
}

// movableClock は testNow から進められる時計で、取得の最中に stub の handler からも進めるため atomic に持つ。
type movableClock struct {
	elapsed atomic.Int64
}

func (c *movableClock) now() time.Time { return testNow.Add(time.Duration(c.elapsed.Load())) }

func (c *movableClock) advance(d time.Duration) { c.elapsed.Add(int64(d)) }

// token は stub を Issuer とする JWT を、keyID を添えて key で署名する。
func (s *issuerStub) token(t *testing.T, key *rsa.PrivateKey, keyID string) string {
	t.Helper()
	claims := marshalClaims(t, claimsWith(map[string]any{"iss": s.issuer()}))
	return signWithKeyID(t, jose.RS256, key, keyID, claims)
}

// documents は、Discovery と JWKS で同じ規則になることを文書ごとに確かめるための表である。
var documents = []struct {
	name    string
	respond func(*issuerStub, http.HandlerFunc)
	current func(*issuerStub) http.HandlerFunc
}{
	{"discovery", (*issuerStub).respondDiscovery, func(s *issuerStub) http.HandlerFunc { return *s.discovery.Load() }},
	{"jwks", (*issuerStub).respondJWKS, func(s *issuerStub) http.HandlerFunc { return *s.jwks.Load() }},
}

func discoveryBodyOf(issuer, jwksURI string) string {
	return fmt.Sprintf(`{"issuer":%q,"jwks_uri":%q}`, issuer, jwksURI)
}

func jwksBody(t *testing.T, key *rsa.PrivateKey, keyID string) string {
	t.Helper()
	return jwksBodyOf(t, publicJWK(key, keyID))
}

func jwksBodyOf(t *testing.T, key jose.JSONWebKey) string {
	t.Helper()
	body, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return string(body)
}

func publicJWK(key *rsa.PrivateKey, keyID string) jose.JSONWebKey {
	return jose.JSONWebKey{Key: key.Public(), KeyID: keyID, Algorithm: string(jose.RS256), Use: "sig"}
}

func respondBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) }
}

func respondStatus(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, responseMarker)
	}
}

// padded は JSON の後ろに空白を足し、復号できる本文のまま大きさだけを size に揃える。
func padded(body string, size int) string {
	return body + strings.Repeat(" ", size-len(body))
}

// assertRejected は失敗の種別と、どの拒否でも JWT の断片、応答本文、ライブラリの文言がエラー文言に入らないことを確かめる。
func assertRejected(t *testing.T, got caller.Caller, err error, want error, token string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("Authenticate error = %v, want %v", err, want)
	}
	if got != (caller.Caller{}) {
		t.Errorf("Authenticate = %+v, want zero Caller", got)
	}
	forbidden := append(strings.Split(token, "."), responseMarker, "oidc:", "go-jose")
	for _, value := range forbidden {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err.Error(), value)
		}
	}
}

func assertAuthenticated(t *testing.T, verifier *Verifier, token string) {
	t.Helper()
	got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if want := newCaller(t, 11, 22, 33, 2, validAttributes()); got != want {
		t.Errorf("Authenticate = %+v, want %+v", got, want)
	}
}

func TestAuthenticate_RetrievesKeysFromIssuer(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)

	assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(t), testKeyID))
}

func TestAuthenticate_OpensConnectionForEachRetrieval(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)

	assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(t), testKeyID))

	if got := stub.newConnections.Load(); got != 2 {
		t.Errorf("new connections = %d, want 2", got)
	}
}

func TestAuthenticate_RetrievalFailure(t *testing.T) {
	closeConnection := func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack returned error: %v", err)
			return
		}
		_ = conn.Close()
	}
	truncateBody := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = io.WriteString(w, `{"issuer":`)
	}
	redirectToDocument := func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path, http.StatusFound) //nolint:gosec // G710: テスト内の Issuer が自身の同じパスへ転送するだけで、外部へは転送しない
	}
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{"connection closed returns ErrTransientFailure", closeConnection, ErrTransientFailure},
		{"status 500 returns ErrTransientFailure", respondStatus(500), ErrTransientFailure},
		{"status 403 returns ErrTransientFailure", respondStatus(403), ErrTransientFailure},
		{"status 429 returns ErrTransientFailure", respondStatus(429), ErrTransientFailure},
		{"body cut off returns ErrTransientFailure", truncateBody, ErrTransientFailure},
		{"status 404 returns ErrInternalFailure", respondStatus(404), ErrInternalFailure},
		{"redirect returns ErrInternalFailure", redirectToDocument, ErrInternalFailure},
		{"non JSON body returns ErrInternalFailure", respondBody("<html>" + responseMarker), ErrInternalFailure},
	}
	for _, document := range documents {
		for _, tt := range tests {
			t.Run(document.name+" "+tt.name, func(t *testing.T) {
				stub := newIssuerStub(t, testKey(t), testKeyID)
				document.respond(stub, tt.handler)
				token := stub.token(t, testKey(t), testKeyID)

				got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

				assertRejected(t, got, err, tt.want, token)
			})
		}
	}
}

func TestAuthenticate_UnacceptableDiscovery(t *testing.T) {
	tests := []struct {
		name string
		body func(stub *issuerStub) string
	}{
		{"different issuer", func(stub *issuerStub) string {
			return discoveryBodyOf("https://other.example/"+responseMarker, stub.issuer()+jwksPath)
		}},
		{"jwks_uri over http", func(stub *issuerStub) string {
			return discoveryBodyOf(stub.issuer(), strings.Replace(stub.issuer(), "https://", "http://", 1)+"/"+responseMarker)
		}},
		{"jwks_uri on another host", func(stub *issuerStub) string {
			return discoveryBodyOf(stub.issuer(), "https://other.example/"+responseMarker)
		}},
		{"jwks_uri on another port", func(stub *issuerStub) string {
			// Issuer の末尾はポートのため、数字を足すとホストが同じままポートの値だけが変わる
			return discoveryBodyOf(stub.issuer(), stub.issuer()+"0/"+responseMarker)
		}},
		{"jwks_uri unparsable", func(stub *issuerStub) string {
			return discoveryBodyOf(stub.issuer(), "https://["+responseMarker)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrInternalFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			stub.respondDiscovery(respondBody(tt.body(stub)))
			token := stub.token(t, testKey(t), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

			assertRejected(t, got, err, ErrInternalFailure, token)
			if requests := stub.jwksRequests.Load(); requests != 0 {
				t.Errorf("JWKS requests = %d, want 0", requests)
			}
		})
	}
}

// 集合として解釈できない JWKS は使用可能な鍵が含まれていても、使用可能な鍵のない JWKS も、集合全体を拒否する。
func TestAuthenticate_UnacceptableJWKS(t *testing.T) {
	usable := jwkWith(t, testKey(t), testKeyID, nil)
	tests := []struct {
		name string
		body string
	}{
		{"keys not an array", `{"keys":{"` + responseMarker + `":` + usable + `}}`},
		{"duplicate keys at the root", `{"keys":[` + usable + `],"keys":[` + usable + `]}`},
		{"duplicate unknown name at the root", `{"keys":[` + usable + `],"unknown":1,"unknown":2}`},
		{"invalid UTF-8 in a key", jwksOf(usable, "{\"kty\":\"RSA\",\"kid\":\"\xff\"}")},
		{"only an excluded key", jwksOf(`{"kty":"EC","kid":"` + testKeyID + `","crv":"P-256"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrInternalFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			stub.respondJWKS(respondBody(tt.body))
			token := stub.token(t, testKey(t), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

			assertRejected(t, got, err, ErrInternalFailure, token)
		})
	}
}

// 秘密鍵か対称鍵の値を含む JWK は、使用可能な鍵の後にあっても集合全体を拒否させ、ほかの JWK も除外として記録しない。
func TestAuthenticate_JWKSWithSecretKeyMaterial(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{"RSA private key", strings.TrimSuffix(strings.TrimPrefix(jwksBodyOf(t, jose.JSONWebKey{Key: generateKey(t), KeyID: "secret"}), `{"keys":[`), `]}`)},
		{"RSA public key with a prime", jwkWith(t, generateKey(t), "secret", map[string]any{"p": "AQ"})},
		{"symmetric key", `{"kty":"oct","kid":"secret","k":"` + responseMarker + `"}`},
		{"EC private key", `{"kty":"EC","kid":"secret","crv":"P-256","d":"` + responseMarker + `"}`},
		{"RSA key with a symmetric key value", jwkWith(t, generateKey(t), "secret", map[string]any{"k": responseMarker})},
		// 名前の重複で JWK として読めなくても、秘密鍵の値があれば除外ではなく集合全体を拒否する
		{"private key with a duplicate member name", `{"kid":"secret",` + jwkWith(t, generateKey(t), "secret", map[string]any{"d": responseMarker})[1:]},
		// 重複した kty のどれかが oct なら、並び順によらず集合全体を拒否する
		{"oct before a duplicate kty without a key value", `{"kty":"oct","kty":"RSA","kid":"secret"}`},
		{"oct after a duplicate kty without a key value", `{"kty":"RSA","kty":"oct","kid":"secret"}`},
	}
	// 秘密鍵の値の前に除外される JWK を置き、鍵ごとの判定より先に集合全体を確かめることを確かめる
	excluded := `{"kty":"EC","kid":"excluded","crv":"P-256"}`
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrInternalFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			stub.respondJWKS(respondBody(jwksOf(excluded, jwkWith(t, testKey(t), testKeyID, nil), tt.secret)))
			token := stub.token(t, testKey(t), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

			assertRejected(t, got, err, ErrInternalFailure, token)
			if records := excludedRecords(t, &stub.logs); len(records) != 0 {
				t.Errorf("excluded records = %v, want none", records)
			}
			if strings.Contains(stub.logs.String(), responseMarker) {
				t.Errorf("logs %q contain the secret value", stub.logs.String())
			}
		})
	}
}

// excludedRecords は、ログから除外した JWK の記録だけを取り出す。
func excludedRecords(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for line := range strings.Lines(logs.String()) {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("Unmarshal returned error: %v", err)
		}
		if record["msg"] == "jwk excluded" {
			records = append(records, record)
		}
	}
	return records
}

// jwkWith は key の公開鍵の JWK を、members のメンバーで上書きした JSON で返す。
func jwkWith(t *testing.T, key *rsa.PrivateKey, keyID string, members map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(jose.JSONWebKey{Key: key.Public(), KeyID: keyID})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var jwk map[string]any
	if err := json.Unmarshal(encoded, &jwk); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	maps.Copy(jwk, members)
	overridden, err := json.Marshal(jwk)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return string(overridden)
}

func jwksOf(keys ...string) string {
	return `{"keys":[` + strings.Join(keys, ",") + `]}`
}

// selfSignedCertificate は subject の公開鍵の自己署名の証明書を作る。
func selfSignedCertificate(t *testing.T, subject *rsa.PrivateKey, keyUsage x509.KeyUsage) *x509.Certificate {
	t.Helper()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), KeyUsage: keyUsage}
	return createCertificate(t, template, template, subject, subject)
}

// issuedCertificate は subject の公開鍵の証明書を、発行者の証明書 parent の鍵 signer で署名して作る。
func issuedCertificate(t *testing.T, subject *rsa.PrivateKey, parent *x509.Certificate, signer *rsa.PrivateKey) *x509.Certificate {
	t.Helper()
	return createCertificate(t, &x509.Certificate{SerialNumber: big.NewInt(1)}, parent, subject, signer)
}

func createCertificate(t *testing.T, template, parent *x509.Certificate, subject, signer *rsa.PrivateKey) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &subject.PublicKey, signer)
	if err != nil {
		t.Fatalf("CreateCertificate returned error: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate returned error: %v", err)
	}
	return cert
}

// x5cOf は証明書を JWK の x5c の値にする。
func x5cOf(certs ...*x509.Certificate) []string {
	var encoded []string
	for _, cert := range certs {
		encoded = append(encoded, base64.StdEncoding.EncodeToString(cert.Raw))
	}
	return encoded
}

// 除外される JWK のうち鍵の値を使うものは、2047 bit の法を除き key の公開鍵から作り、keyID は除外の記録に残る kid である。
func TestAuthenticate_ExcludedJWK(t *testing.T) {
	const keyID = "excluded"
	key := generateKey(t)
	shortKey, err := rsa.GenerateKey(rand.Reader, 2047) //nolint:gosec // G403: 使用可能な鍵の下限のすぐ下で JWK を除外することを確かめるため、短い鍵を意図して作る
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	issuer := generateKey(t)
	issuerCert := selfSignedCertificate(t, issuer, x509.KeyUsageCertSign)
	unrelatedCert := selfSignedCertificate(t, generateKey(t), x509.KeyUsageCertSign)
	jwk := func(members map[string]any) string { return jwkWith(t, key, keyID, members) }
	tests := []struct {
		name  string
		jwk   string
		keyID string
	}{
		{"key not an object", `"` + responseMarker + `"`, ""},
		{"duplicate member name", `{"kid":"` + keyID + `",` + jwk(nil)[1:], ""},
		{"non RSA key", `{"kty":"EC","kid":"` + keyID + `","crv":"P-256"}`, keyID},
		{"kty that is not a string", jwk(map[string]any{"kty": 1}), keyID},
		{"no kid", jwkWith(t, key, "", nil), ""},
		{"modulus that is not Base64urlUInt", jwk(map[string]any{"n": "!" + responseMarker}), keyID},
		{"modulus with a leading zero octet", jwk(map[string]any{"n": base64.RawURLEncoding.EncodeToString(append([]byte{0}, key.N.Bytes()...))}), keyID},
		{"exponent with a leading zero octet", jwk(map[string]any{"e": "AAEAAQ"}), keyID},
		{"modulus of 2047 bits", jwkWith(t, shortKey, keyID, nil), keyID},
		{"even modulus", jwk(map[string]any{"n": base64.RawURLEncoding.EncodeToString(new(big.Int).Lsh(key.N, 1).Bytes())}), keyID},
		{"exponent 1", jwk(map[string]any{"e": "AQ"}), keyID},
		{"even exponent", jwk(map[string]any{"e": "AQAA"}), keyID},
		// 2^31 + 1 は奇数のため、上限だけが除外できる
		{"exponent above 2^31 - 1", jwk(map[string]any{"e": "gAAAAQ"}), keyID},
		{"use for encryption", jwk(map[string]any{"use": "enc"}), keyID},
		{"empty use", jwk(map[string]any{"use": ""}), keyID},
		{"empty alg", jwk(map[string]any{"alg": ""}), keyID},
		{"key_ops without verify", jwk(map[string]any{"key_ops": []string{"sign"}}), keyID},
		{"empty key_ops", jwk(map[string]any{"key_ops": []string{}}), keyID},
		{"key_ops that is null", jwk(map[string]any{"key_ops": nil}), keyID},
		{"key_ops with a non string element", jwk(map[string]any{"key_ops": []any{1}}), keyID},
		{"key_ops with a duplicate", jwk(map[string]any{"key_ops": []string{"verify", "verify"}}), keyID},
		{"alg other than RS256", jwk(map[string]any{"alg": "RS384"}), keyID},
		{"use and key_ops that disagree", jwk(map[string]any{"use": "sig", "key_ops": []string{"verify", "encrypt"}}), keyID},
		{"empty x5c", jwk(map[string]any{"x5c": []string{}}), keyID},
		{"x5c that is not a certificate", jwk(map[string]any{"x5c": []string{base64.StdEncoding.EncodeToString([]byte(responseMarker))}}), keyID},
		{"x5c of another key", jwk(map[string]any{"x5c": x5cOf(issuerCert)}), keyID},
		{"x5c whose key usage excludes digital signatures", jwk(map[string]any{"x5c": x5cOf(selfSignedCertificate(t, key, x509.KeyUsageKeyEncipherment))}), keyID},
		{"x5c whose second certificate is not the issuer", jwk(map[string]any{"x5c": x5cOf(issuedCertificate(t, key, issuerCert, issuer), unrelatedCert)}), keyID},
	}
	for _, tt := range tests {
		t.Run(tt.name+" next to a usable key is excluded and recorded", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			stub.respondJWKS(respondBody(jwksOf(tt.jwk, jwkWith(t, testKey(t), testKeyID, nil))))
			verifier := stub.verifier(t)

			assertAuthenticated(t, verifier, stub.token(t, testKey(t), testKeyID))
			// 除外した JWK の kid は保持しないため、その kid の JWT は鍵の値によらず拒否される
			token := stub.token(t, key, keyID)
			got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))
			assertRejected(t, got, err, ErrAuthenticationFailure, token)

			records := excludedRecords(t, &stub.logs)
			if len(records) == 0 {
				t.Fatalf("logs %q have no excluded record", stub.logs.String())
			}
			record := records[0]
			if record["level"] != "WARN" || record["index"] != float64(0) || record["kid"] != tt.keyID || record["reason"] == "" {
				t.Errorf("excluded record = %v, want WARN with index 0, kid %q, and a reason", record, tt.keyID)
			}
			modulus := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
			for _, value := range []string{responseMarker, "go-jose", modulus} {
				if strings.Contains(stub.logs.String(), value) {
					t.Errorf("logs %q contain %q", stub.logs.String(), value)
				}
			}
		})
	}
}

func TestAuthenticate_UsableJWK(t *testing.T) {
	issuer := generateKey(t)
	issuerCert := selfSignedCertificate(t, issuer, x509.KeyUsageCertSign)
	tests := []struct {
		name    string
		members map[string]any
	}{
		{"no use, key_ops, or alg", nil},
		{"key_ops with verify and an operation outside signatures but no use", map[string]any{"key_ops": []string{"verify", "encrypt"}}},
		{"use and key_ops limited to signature operations", map[string]any{"use": "sig", "key_ops": []string{"sign", "verify"}}},
		{"x5c of a self-signed certificate without key usage", map[string]any{"x5c": x5cOf(selfSignedCertificate(t, testKey(t), 0))}},
		{"x5c of a certificate for digital signatures", map[string]any{"x5c": x5cOf(selfSignedCertificate(t, testKey(t), x509.KeyUsageDigitalSignature))}},
		{"x5c with its issuer", map[string]any{"x5c": x5cOf(issuedCertificate(t, testKey(t), issuerCert, issuer), issuerCert)}},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns Caller", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			stub.respondJWKS(respondBody(jwksOf(jwkWith(t, testKey(t), testKeyID, tt.members))))

			assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(t), testKeyID))
		})
	}
}

// 指数の境界の鍵を JWT の署名に使う鍵として作らずに、保持されたことを確かめる。
// 保持された鍵で署名が一致しなければ認証失敗になり、除外されれば使用可能な鍵がなく内部障害になる。
func TestAuthenticate_ExponentAtBoundaryIsKept(t *testing.T) {
	tests := []struct {
		name     string
		exponent string
	}{
		{"exponent 3", "Aw"},
		{"exponent 2^31 - 1", "f____w"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrAuthenticationFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			stub.respondJWKS(respondBody(jwksOf(jwkWith(t, testKey(t), testKeyID, map[string]any{"e": tt.exponent}))))
			token := stub.token(t, testKey(t), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

			assertRejected(t, got, err, ErrAuthenticationFailure, token)
		})
	}
}

func TestAuthenticate_SameKeyID(t *testing.T) {
	t.Run("two usable keys under another key ID return Caller", func(t *testing.T) {
		stub := newIssuerStub(t, testKey(t), testKeyID)
		stub.respondJWKS(respondBody(jwksOf(
			jwkWith(t, generateKey(t), "shared", nil),
			jwkWith(t, generateKey(t), "shared", nil),
			jwkWith(t, testKey(t), testKeyID, nil),
		)))

		assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(t), testKeyID))
	})
	t.Run("two usable keys are rejected, retrieved again, and the key left after rotation returns Caller", func(t *testing.T) {
		stub := newIssuerStub(t, testKey(t), testKeyID)
		clock := &movableClock{}
		verifier := stub.verifierWithClock(t, clock.now)
		// 先頭の鍵で署名する。一意性を判定しなければ go-jose が先頭の鍵を選んで受理する
		stub.respondJWKS(respondBody(jwksOf(jwkWith(t, testKey(t), testKeyID, nil), jwkWith(t, generateKey(t), testKeyID, nil))))
		token := stub.token(t, testKey(t), testKeyID)
		got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))
		assertRejected(t, got, err, ErrAuthenticationFailure, token)

		stub.respondJWKS(respondBody(jwksOf(jwkWith(t, testKey(t), testKeyID, nil))))
		clock.advance(keyRetrievalInterval)
		assertAuthenticated(t, verifier, token)
		if requests := stub.jwksRequests.Load(); requests != 2 {
			t.Errorf("JWKS requests = %d, want 2", requests)
		}
	})
	t.Run("an excluded JWK before a usable key returns Caller", func(t *testing.T) {
		stub := newIssuerStub(t, testKey(t), testKeyID)
		excluded := jwkWith(t, generateKey(t), testKeyID, map[string]any{"use": "enc"})
		stub.respondJWKS(respondBody(jwksOf(excluded, jwkWith(t, testKey(t), testKeyID, nil))))

		assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(t), testKeyID))
	})
}

// signWithHeader は Protected Header を JSON のまま受ける。重複した名前のように、署名器から作れない Header にも署名するため。
func signWithHeader(t *testing.T, key *rsa.PrivateKey, header string, payload []byte) string {
	t.Helper()
	signingInput := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15 returned error: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (s *issuerStub) tokenWithHeader(t *testing.T, key *rsa.PrivateKey, header string) string {
	t.Helper()
	return signWithHeader(t, key, header, marshalClaims(t, claimsWith(map[string]any{"iss": s.issuer()})))
}

func TestAuthenticate_UnacceptableHeaderIsRejectedWithoutRetrieval(t *testing.T) {
	const base = `"alg":"RS256","kid":"` + testKeyID + `"`
	tests := []struct {
		name   string
		header string
	}{
		{"crit with an unsupported extension", `{` + base + `,"crit":["exp"],"exp":1}`},
		{"empty crit", `{` + base + `,"crit":[]}`},
		{"crit with a registered name", `{` + base + `,"crit":["alg"]}`},
		{"crit naming b64", `{` + base + `,"crit":["b64"],"b64":true}`},
		{"crit that is null", `{` + base + `,"crit":null}`},
		{"b64 false", `{` + base + `,"b64":false}`},
		{"b64 true", `{` + base + `,"b64":true}`},
		{"b64 that is not a boolean", `{` + base + `,"b64":"false"}`},
		{"duplicate name", `{` + base + `,"kid":"` + testKeyID + `"}`},
		{"duplicate name after unescaping", `{` + base + `,` + escapedName("kid") + `:"` + testKeyID + `"}`},
		{"invalid UTF-8", `{` + base + `,"unknown":"` + "\xff" + `"}`},
		// cty の拒否は値によらないため、Nested JWT を示す値で確かめる
		{"cty", `{` + base + `,"cty":"JWT"}`},
		{"alg none", `{"alg":"none","kid":"` + testKeyID + `"}`},
		{"no kid", `{"alg":"RS256"}`},
		{"empty kid", `{"alg":"RS256","kid":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrAuthenticationFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			token := stub.tokenWithHeader(t, testKey(t), tt.header)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

			assertRejected(t, got, err, ErrAuthenticationFailure, token)
			if requests := stub.documentRequests(); requests != 0 {
				t.Errorf("retrieved documents %d times, want 0", requests)
			}
		})
	}
}

func TestAuthenticate_UnknownHeaderIsIgnored(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	token := stub.tokenWithHeader(t, testKey(t), `{"alg":"RS256","kid":"`+testKeyID+`","unknown":{"nested":1}}`)

	assertAuthenticated(t, stub.verifier(t), token)
}

func attackerKeyHeader(t *testing.T, attacker *rsa.PrivateKey) string {
	t.Helper()
	return `{"alg":"RS256","kid":"` + testKeyID + `","jwk":` + jwkWith(t, attacker, testKeyID, nil) +
		`,"jku":"https://attacker.example/jwks","x5u":"https://attacker.example/cert"}`
}

func TestAuthenticate_KeyHeadersDoNotChangeRetrievalOrVerificationKey(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	var requested []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		return stub.server.Client().Transport.RoundTrip(req)
	})
	verifier, err := NewVerifier(stub.issuer(), testAudience, testClock, discardLog, WithTransport(transport))
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}

	assertAuthenticated(t, verifier, stub.tokenWithHeader(t, testKey(t), attackerKeyHeader(t, generateKey(t))))

	if want := []string{stub.issuer() + discoveryPath, stub.issuer() + jwksPath}; !slices.Equal(requested, want) {
		t.Errorf("requested = %v, want %v", requested, want)
	}
}

func TestAuthenticate_SignatureByKeyInHeaderReturnsErrAuthenticationFailure(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	attacker := generateKey(t)
	token := stub.tokenWithHeader(t, attacker, attackerKeyHeader(t, attacker))

	got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

	assertRejected(t, got, err, ErrAuthenticationFailure, token)
}

func TestAuthenticate_ResponseSizeLimit(t *testing.T) {
	validBodies := []struct {
		name    string
		body    func(stub *issuerStub) string
		respond func(*issuerStub, http.HandlerFunc)
	}{
		{"discovery", (*issuerStub).discoveryBody, (*issuerStub).respondDiscovery},
		{"jwks", func(*issuerStub) string { return jwksBody(t, testKey(t), testKeyID) }, (*issuerStub).respondJWKS},
	}
	for _, document := range validBodies {
		t.Run(document.name+" at the limit returns Caller", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			document.respond(stub, respondBody(padded(document.body(stub), keyRetrievalSizeLimit)))

			assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(t), testKeyID))
		})
		t.Run(document.name+" over the limit returns ErrInternalFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			document.respond(stub, respondBody(padded(document.body(stub), keyRetrievalSizeLimit+1)))
			token := stub.token(t, testKey(t), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

			assertRejected(t, got, err, ErrInternalFailure, token)
		})
	}
}

func TestAuthenticate_DeadlineExceededReturnsErrTransientFailure(t *testing.T) {
	waitForDisconnect := func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }
	authenticateWithDeadline := func(t *testing.T, verifier *Verifier, token string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		got, err := verifier.Authenticate(ctx, unverifiedIDToken(t, token))

		assertRejected(t, got, err, ErrTransientFailure, token)
	}
	t.Run("discovery deadline exceeded returns ErrTransientFailure", func(t *testing.T) {
		stub := newIssuerStub(t, testKey(t), testKeyID)
		stub.respondDiscovery(waitForDisconnect)

		authenticateWithDeadline(t, stub.verifier(t), stub.token(t, testKey(t), testKeyID))
	})
	// 先に Discovery を保持し、期限が JWKS の取得の間に切れるようにする
	t.Run("jwks deadline exceeded returns ErrTransientFailure", func(t *testing.T) {
		stub := newIssuerStub(t, testKey(t), testKeyID)
		clock := &movableClock{}
		verifier := stub.verifierWithClock(t, clock.now)
		assertAuthenticated(t, verifier, stub.token(t, testKey(t), testKeyID))
		stub.respondJWKS(waitForDisconnect)
		clock.advance(keyRetrievalInterval)

		authenticateWithDeadline(t, verifier, stub.token(t, generateKey(t), "unknown"))
	})
}

// deadlineRecorder は、各取得の要求に付いた期限とそこまでの時間を記録し、実際の通信は stub に任せる。
type deadlineRecorder struct {
	transport http.RoundTripper
	deadlines []time.Time
	remaining []time.Duration
}

func (d *deadlineRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline, ok := req.Context().Deadline()
	if !ok {
		return nil, errors.New("request has no deadline")
	}
	d.deadlines = append(d.deadlines, deadline)
	d.remaining = append(d.remaining, time.Until(deadline))
	return d.transport.RoundTrip(req)
}

func TestAuthenticate_EachRetrievalHasItsOwnTimeout(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	recorder := &deadlineRecorder{transport: stub.server.Client().Transport}
	verifier, err := NewVerifier(stub.issuer(), testAudience, testClock, discardLog, WithTransport(recorder))
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}

	assertAuthenticated(t, verifier, stub.token(t, testKey(t), testKeyID))

	if len(recorder.remaining) != 2 {
		t.Fatalf("recorded %d requests, want 2", len(recorder.remaining))
	}
	for i, remaining := range recorder.remaining {
		// 実行の遅れで短くなる分を見込む
		if remaining > keyRetrievalTimeout || remaining < keyRetrievalTimeout/2 {
			t.Errorf("request %d remaining = %v, want about %v", i, remaining, keyRetrievalTimeout)
		}
	}
	// 1 つの期限を共有すると同じ時刻になる
	if !recorder.deadlines[1].After(recorder.deadlines[0]) {
		t.Errorf("deadlines = %v, want the second later than the first", recorder.deadlines)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestAuthenticate_CommunicationFailureKeepsResponseValuesOutOfError(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	stub.respondDiscovery(respondBody(discoveryBodyOf(stub.issuer(), stub.issuer()+"/"+responseMarker)))
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == discoveryPath {
			return stub.server.Client().Transport.RoundTrip(req)
		}
		return nil, errors.New("malformed HTTP response " + responseMarker)
	})
	verifier, err := NewVerifier(stub.issuer(), testAudience, testClock, discardLog, WithTransport(transport))
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}
	token := stub.token(t, testKey(t), testKeyID)

	got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))

	assertRejected(t, got, err, ErrTransientFailure, token)
}

func TestAuthenticate_RequestsDocumentsWithoutCache(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	recordHeader := func(next http.HandlerFunc, got *atomic.Value) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			got.Store(r.Header.Get("Cache-Control"))
			next(w, r)
		}
	}
	var discoveryHeader, jwksHeader atomic.Value
	stub.respondDiscovery(recordHeader(*stub.discovery.Load(), &discoveryHeader))
	stub.respondJWKS(recordHeader(*stub.jwks.Load(), &jwksHeader))

	assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(t), testKeyID))

	if got := discoveryHeader.Load(); got != "no-cache" {
		t.Errorf("discovery Cache-Control = %v, want no-cache", got)
	}
	if got := jwksHeader.Load(); got != "no-cache" {
		t.Errorf("JWKS Cache-Control = %v, want no-cache", got)
	}
}

func TestAuthenticate_DefaultClientRejectsUntrustedCertificate(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	verifier, err := NewVerifier(stub.issuer(), testAudience, testClock, discardLog)
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}
	token := stub.token(t, testKey(t), testKeyID)

	got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))

	assertRejected(t, got, err, ErrTransientFailure, token)
	if requests := stub.discoveryRequests.Load(); requests != 0 {
		t.Errorf("discovery requests = %d, want 0", requests)
	}
}

func TestAuthenticate_JWKSFailureKeepsDiscovery(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	clock := &movableClock{}
	verifier := stub.verifierWithClock(t, clock.now)
	token := stub.token(t, testKey(t), testKeyID)
	valid := *stub.jwks.Load()
	stub.respondJWKS(respondStatus(500))

	got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))
	assertRejected(t, got, err, ErrTransientFailure, token)

	stub.respondJWKS(valid)
	clock.advance(keyRetrievalInterval)
	assertAuthenticated(t, verifier, token)
	if requests := stub.discoveryRequests.Load(); requests != 1 {
		t.Errorf("discovery requests = %d, want 1", requests)
	}
}

func TestAuthenticate_ReusesRetrievedDiscoveryAndKeys(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	verifier := stub.verifier(t)
	token := stub.token(t, testKey(t), testKeyID)

	assertAuthenticated(t, verifier, token)
	assertAuthenticated(t, verifier, token)

	if requests := stub.discoveryRequests.Load(); requests != 1 {
		t.Errorf("discovery requests = %d, want 1", requests)
	}
	if requests := stub.jwksRequests.Load(); requests != 1 {
		t.Errorf("JWKS requests = %d, want 1", requests)
	}
}

func TestAuthenticate_KeyRotationRetrievesJWKSAgain(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	clock := &movableClock{}
	verifier := stub.verifierWithClock(t, clock.now)
	assertAuthenticated(t, verifier, stub.token(t, testKey(t), testKeyID))

	rotated := generateKey(t)
	stub.respondJWKS(respondBody(jwksBody(t, rotated, "rotated")))
	clock.advance(keyRetrievalInterval)
	assertAuthenticated(t, verifier, stub.token(t, rotated, "rotated"))

	if requests := stub.discoveryRequests.Load(); requests != 1 {
		t.Errorf("discovery requests = %d, want 1", requests)
	}
}

func TestAuthenticate_RetrievalIntervalAfterSuccess(t *testing.T) {
	tests := []struct {
		name     string
		elapsed  time.Duration
		wantJWKS int32
	}{
		{"59 seconds rejects without retrieval", 59 * time.Second, 1},
		{"one minute retrieves again", time.Minute, 2},
		{"clock moved back retrieves again", -time.Second, 2},
	}
	tokens := []struct {
		name  string
		keyID string
	}{
		{"unknown key ID", "unknown"},
		{"known key ID with a mismatched signature", testKeyID},
	}
	for _, tt := range tests {
		for _, tokenCase := range tokens {
			t.Run(tokenCase.name+" after "+tt.name, func(t *testing.T) {
				stub := newIssuerStub(t, testKey(t), testKeyID)
				clock := &movableClock{}
				verifier := stub.verifierWithClock(t, clock.now)
				assertAuthenticated(t, verifier, stub.token(t, testKey(t), testKeyID))
				token := stub.token(t, generateKey(t), tokenCase.keyID)

				clock.advance(tt.elapsed)
				got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))

				assertRejected(t, got, err, ErrAuthenticationFailure, token)
				if requests := stub.jwksRequests.Load(); requests != tt.wantJWKS {
					t.Errorf("JWKS requests = %d, want %d", requests, tt.wantJWKS)
				}
			})
		}
	}
}

// 取得の完了から数えると、取得にかかった時間の分だけ再取得が遅れる。
func TestAuthenticate_RetrievalIntervalCountsFromRetrievalStart(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	clock := &movableClock{}
	verifier := stub.verifierWithClock(t, clock.now)
	valid := *stub.jwks.Load()
	const retrievalDuration = 10 * time.Second
	stub.respondJWKS(func(w http.ResponseWriter, r *http.Request) {
		clock.advance(retrievalDuration)
		valid(w, r)
	})
	assertAuthenticated(t, verifier, stub.token(t, testKey(t), testKeyID))
	stub.respondJWKS(valid)
	token := stub.token(t, generateKey(t), "unknown")

	clock.advance(time.Minute - retrievalDuration)
	got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))

	assertRejected(t, got, err, ErrAuthenticationFailure, token)
	if requests := stub.jwksRequests.Load(); requests != 2 {
		t.Errorf("JWKS requests = %d, want 2", requests)
	}
}

// assertFailureKindOnly は、上限の間の失敗に前回の取得の応答の内容が持ち越されていないことを確かめる。
func assertFailureKindOnly(t *testing.T, err error) {
	t.Helper()
	if strings.Contains(err.Error(), "status") {
		t.Errorf("error message %q contains the previous response status", err.Error())
	}
}

// retrievalFailures は、上限の間に前回と同じ種別を返すことを種別ごとに確かめるための表である。
var retrievalFailures = []struct {
	name   string
	status int
	want   error
}{
	{"transient failure", 500, ErrTransientFailure},
	{"internal failure", 404, ErrInternalFailure},
}

func TestAuthenticate_RetrievalIntervalAfterFailureWithKeys(t *testing.T) {
	for _, failure := range retrievalFailures {
		t.Run(failure.name+" is returned without retrieval and retrieved again after the interval", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			clock := &movableClock{}
			verifier := stub.verifierWithClock(t, clock.now)
			token := stub.token(t, testKey(t), testKeyID)
			assertAuthenticated(t, verifier, token)
			rotated := generateKey(t)
			rotatedToken := stub.token(t, rotated, "rotated")

			stub.respondJWKS(respondStatus(failure.status))
			clock.advance(keyRetrievalInterval)
			got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, rotatedToken))
			assertRejected(t, got, err, failure.want, rotatedToken)
			requests := stub.documentRequests()

			stub.respondJWKS(respondBody(jwksBody(t, rotated, "rotated")))
			clock.advance(keyRetrievalInterval - time.Second)
			got, err = verifier.Authenticate(t.Context(), unverifiedIDToken(t, rotatedToken))
			assertRejected(t, got, err, failure.want, rotatedToken)
			assertFailureKindOnly(t, err)
			// 上限の間でも、手元の鍵で検証できる JWT の検証は妨げない
			assertAuthenticated(t, verifier, token)
			if got := stub.documentRequests(); got != requests {
				t.Errorf("document requests = %d, want %d", got, requests)
			}

			clock.advance(time.Second)
			assertAuthenticated(t, verifier, rotatedToken)
		})
	}
}

func TestAuthenticate_RetrievalIntervalAfterFailureWithoutKeys(t *testing.T) {
	for _, document := range documents {
		for _, failure := range retrievalFailures {
			t.Run(document.name+" "+failure.name+" is returned without retrieval and retrieved again after the interval", func(t *testing.T) {
				stub := newIssuerStub(t, testKey(t), testKeyID)
				clock := &movableClock{}
				verifier := stub.verifierWithClock(t, clock.now)
				token := stub.token(t, testKey(t), testKeyID)
				valid := document.current(stub)

				document.respond(stub, respondStatus(failure.status))
				got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))
				assertRejected(t, got, err, failure.want, token)
				requests := stub.documentRequests()

				document.respond(stub, valid)
				clock.advance(keyRetrievalInterval - time.Second)
				got, err = verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))
				assertRejected(t, got, err, failure.want, token)
				assertFailureKindOnly(t, err)
				if got := stub.documentRequests(); got != requests {
					t.Errorf("document requests = %d, want %d", got, requests)
				}

				clock.advance(time.Second)
				assertAuthenticated(t, verifier, token)
			})
		}
	}
}

// 失敗を残すと、前回取得した JWKS の鍵で検証できないと分かった後も取得の失敗を返す。
func TestAuthenticate_SuccessfulRetrievalClearsPreviousFailure(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	clock := &movableClock{}
	verifier := stub.verifierWithClock(t, clock.now)
	token := stub.token(t, testKey(t), testKeyID)
	valid := *stub.jwks.Load()

	stub.respondJWKS(respondStatus(500))
	got, err := verifier.Authenticate(t.Context(), unverifiedIDToken(t, token))
	assertRejected(t, got, err, ErrTransientFailure, token)

	stub.respondJWKS(valid)
	clock.advance(keyRetrievalInterval)
	assertAuthenticated(t, verifier, token)

	unknownKeyToken := stub.token(t, generateKey(t), "unknown")
	got, err = verifier.Authenticate(t.Context(), unverifiedIDToken(t, unknownKeyToken))
	assertRejected(t, got, err, ErrAuthenticationFailure, unknownKeyToken)
}

func TestAuthenticate_RetrievedKeysRejectInvalidToken(t *testing.T) {
	tests := []struct {
		name  string
		token func(t *testing.T, stub *issuerStub) string
	}{
		{"signature by another key under the same key ID", func(t *testing.T, stub *issuerStub) string {
			return stub.token(t, generateKey(t), testKeyID)
		}},
		{"unknown key ID", func(t *testing.T, stub *issuerStub) string {
			return stub.token(t, generateKey(t), "unknown")
		}},
		{"issuer differing from the fixed one", func(t *testing.T, _ *issuerStub) string {
			return signWithKeyID(t, jose.RS256, testKey(t), testKeyID, marshalClaims(t, claimsWith(map[string]any{"iss": "https://other.example"})))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrAuthenticationFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			token := tt.token(t, stub)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

			assertRejected(t, got, err, ErrAuthenticationFailure, token)
			if errors.Is(err, ErrTransientFailure) || errors.Is(err, ErrInternalFailure) {
				t.Errorf("Authenticate error = %v, want only ErrAuthenticationFailure", err)
			}
		})
	}
}

func TestAuthenticate_ConcurrentCallsShareRetrieval(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	verifier := stub.verifier(t)
	token := unverifiedIDToken(t, stub.token(t, testKey(t), testKeyID))

	var calls sync.WaitGroup
	for range 8 {
		calls.Go(func() {
			if _, err := verifier.Authenticate(t.Context(), token); err != nil {
				t.Errorf("Authenticate returned error: %v", err)
			}
		})
	}
	calls.Wait()

	if requests := stub.discoveryRequests.Load(); requests != 1 {
		t.Errorf("discovery requests = %d, want 1", requests)
	}
	if requests := stub.jwksRequests.Load(); requests != 1 {
		t.Errorf("JWKS requests = %d, want 1", requests)
	}
}

func TestAuthenticate_ConcurrentUnknownKeyIDsStayWithinRetrievalInterval(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	verifier := stub.verifier(t)
	unknownKeyToken := unverifiedIDToken(t, stub.token(t, generateKey(t), "unknown"))

	var calls sync.WaitGroup
	for range 8 {
		calls.Go(func() {
			if _, err := verifier.Authenticate(t.Context(), unknownKeyToken); !errors.Is(err, ErrAuthenticationFailure) {
				t.Errorf("Authenticate error = %v, want ErrAuthenticationFailure", err)
			}
		})
	}
	calls.Wait()

	if requests := stub.jwksRequests.Load(); requests != 1 {
		t.Errorf("JWKS requests = %d, want 1", requests)
	}
}

// retrievalRecorder を呼び出しごとに分けないと、取得に至らずに拒否された呼び出しが、別の呼び出しの取得の失敗を受け取る。
func TestAuthenticate_ConcurrentRetrievalFailureDoesNotReachCallsRejectedWithoutRetrieval(t *testing.T) {
	stub := newIssuerStub(t, testKey(t), testKeyID)
	clock := &movableClock{}
	verifier := stub.verifierWithClock(t, clock.now)
	assertAuthenticated(t, verifier, stub.token(t, testKey(t), testKeyID))
	stub.respondJWKS(respondStatus(500))
	clock.advance(keyRetrievalInterval)

	unknownKeyToken := unverifiedIDToken(t, stub.token(t, generateKey(t), "unknown"))
	otherAudience := marshalClaims(t, claimsWith(map[string]any{"iss": stub.issuer(), "aud": "other"}))
	rejectedToken := unverifiedIDToken(t, signWithKeyID(t, jose.RS256, testKey(t), testKeyID, otherAudience))

	var calls sync.WaitGroup
	for range 8 {
		calls.Go(func() {
			if _, err := verifier.Authenticate(t.Context(), unknownKeyToken); !errors.Is(err, ErrTransientFailure) {
				t.Errorf("unknown key error = %v, want ErrTransientFailure", err)
			}
		})
		calls.Go(func() {
			if _, err := verifier.Authenticate(t.Context(), rejectedToken); !errors.Is(err, ErrAuthenticationFailure) {
				t.Errorf("other audience error = %v, want ErrAuthenticationFailure", err)
			}
		})
	}
	calls.Wait()
}

// 名前の判定と NumericDate の判定は、鍵情報に依存しないため取得の前に行う。
func TestAuthenticate_UnacceptableClaimSetIsRejectedWithoutRetrieval(t *testing.T) {
	tests := []struct {
		name    string
		leading string
		omitted string
	}{
		{"duplicate claim name", `"jti":"a","jti":"b"`, "jti"},
		{"claim name differing only by case", `"EXP":1700000300`, "exp"},
		{"numeric date as a string", `"exp":"1700000300"`, "exp"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrAuthenticationFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			claims := claimsWith(map[string]any{"iss": stub.issuer()})
			delete(claims, tt.omitted)
			payload := append([]byte("{"+tt.leading+","), marshalClaims(t, claims)[1:]...)
			token := signWithKeyID(t, jose.RS256, testKey(t), testKeyID, payload)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, token))

			assertRejected(t, got, err, ErrAuthenticationFailure, token)
			if requests := stub.documentRequests(); requests != 0 {
				t.Errorf("retrieved documents %d times, want 0", requests)
			}
		})
	}
}

// go-oidc の解析は JSON 形式と空白を含む JWT を通し、go-jose の解析は CR と LF を読み飛ばすため、正当な署名でも拒否することを確かめる。
func TestAuthenticate_NonCompactSerializationIsRejectedWithoutRetrieval(t *testing.T) {
	tests := []struct {
		name      string
		serialize func(t *testing.T, compact string) string
	}{
		{"JSON serialization returns ErrAuthenticationFailure", func(t *testing.T, compact string) string {
			t.Helper()
			signature, err := jose.ParseSignedCompact(compact, []jose.SignatureAlgorithm{jose.RS256})
			if err != nil {
				t.Fatalf("ParseSignedCompact returned error: %v", err)
			}
			return signature.FullSerialize()
		}},
		{"compact with whitespace returns ErrAuthenticationFailure", func(_ *testing.T, compact string) string {
			return strings.Replace(compact, ".", ". ", 1)
		}},
		{"compact with CR returns ErrAuthenticationFailure", func(_ *testing.T, compact string) string {
			return strings.Replace(compact, ".", ".\r", 1)
		}},
		{"compact with LF returns ErrAuthenticationFailure", func(_ *testing.T, compact string) string {
			return strings.Replace(compact, ".", ".\n", 1)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := newIssuerStub(t, testKey(t), testKeyID)
			compact := stub.token(t, testKey(t), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), unverifiedIDToken(t, tt.serialize(t, compact)))

			assertRejected(t, got, err, ErrAuthenticationFailure, compact)
			if requests := stub.documentRequests(); requests != 0 {
				t.Errorf("retrieved documents %d times, want 0", requests)
			}
		})
	}
}
