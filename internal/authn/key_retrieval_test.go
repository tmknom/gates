package authn

import (
	"context"
	"crypto/rsa"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"

	"github.com/tmknom/gates/internal/authz"
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
	discovery         atomic.Pointer[http.HandlerFunc]
	jwks              atomic.Pointer[http.HandlerFunc]
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
	verifier, err := NewVerifier(s.issuer(), testAudience, testClock, WithTransport(s.server.Client().Transport))
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}
	return verifier
}

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
}{
	{"discovery", (*issuerStub).respondDiscovery},
	{"jwks", (*issuerStub).respondJWKS},
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
func assertRejected(t *testing.T, got authz.Caller, err error, want error, token string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("Authenticate error = %v, want %v", err, want)
	}
	if got != (authz.Caller{}) {
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
	got, err := verifier.Authenticate(t.Context(), rawIDToken(t, token))
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if want := newCaller(t, 11, 22, 33, 2, validAttributes()); got != want {
		t.Errorf("Authenticate = %+v, want %+v", got, want)
	}
}

func TestAuthenticate_RetrievesKeysFromIssuer(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)

	assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(), testKeyID))
}

func TestAuthenticate_RetrievalFailure(t *testing.T) {
	closeConnection := func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			panic(err)
		}
		_ = conn.Close()
	}
	truncateBody := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = io.WriteString(w, `{"issuer":`)
	}
	redirectToDocument := func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
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
				stub := newIssuerStub(t, testKey(), testKeyID)
				document.respond(stub, tt.handler)
				token := stub.token(t, testKey(), testKeyID)

				got, err := stub.verifier(t).Authenticate(t.Context(), rawIDToken(t, token))

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
			stub := newIssuerStub(t, testKey(), testKeyID)
			stub.respondDiscovery(respondBody(tt.body(stub)))
			token := stub.token(t, testKey(), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), rawIDToken(t, token))

			assertRejected(t, got, err, ErrInternalFailure, token)
			if requests := stub.jwksRequests.Load(); requests != 0 {
				t.Errorf("JWKS requests = %d, want 0", requests)
			}
		})
	}
}

func TestAuthenticate_UnacceptableJWKS(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"keys empty", `{"keys":[]}`},
		{"only non RSA keys", `{"keys":[{"kty":"EC","kid":"ec"}]}`},
		{"key not an object", `{"keys":["` + responseMarker + `"]}`},
		{"RSA key with invalid modulus", `{"keys":[{"kty":"RSA","kid":"` + testKeyID + `","n":"!` + responseMarker + `","e":"AQAB"}]}`},
		{"RSA key without key ID", jwksBody(t, testKey(), "")},
		{"RSA key that is not public", jwksBodyOf(t, jose.JSONWebKey{Key: testKey(), KeyID: testKeyID})},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrInternalFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(), testKeyID)
			stub.respondJWKS(respondBody(tt.body))
			token := stub.token(t, testKey(), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), rawIDToken(t, token))

			assertRejected(t, got, err, ErrInternalFailure, token)
		})
	}
}

func TestAuthenticate_JWKSSkipsUnusableKeys(t *testing.T) {
	singleKey := func(body string) string {
		return strings.TrimSuffix(strings.TrimPrefix(body, `{"keys":[`), `]}`)
	}
	tests := []struct {
		name     string
		unusable string
	}{
		{"non RSA key", `{"kty":"EC","kid":"ec","crv":"unknown"}`},
		{"RSA key without key ID", singleKey(jwksBody(t, generateKey(), ""))},
		{"RSA key that is not public", singleKey(jwksBodyOf(t, jose.JSONWebKey{Key: generateKey(), KeyID: "private"}))},
	}
	for _, tt := range tests {
		t.Run(tt.name+" next to a usable key returns Caller", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(), testKeyID)
			usable := singleKey(jwksBody(t, testKey(), testKeyID))
			stub.respondJWKS(respondBody(`{"keys":[` + tt.unusable + `,` + usable + `]}`))

			assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(), testKeyID))
		})
	}
}

func TestAuthenticate_ResponseSizeLimit(t *testing.T) {
	validBodies := []struct {
		name    string
		body    func(stub *issuerStub) string
		respond func(*issuerStub, http.HandlerFunc)
	}{
		{"discovery", (*issuerStub).discoveryBody, (*issuerStub).respondDiscovery},
		{"jwks", func(*issuerStub) string { return jwksBody(t, testKey(), testKeyID) }, (*issuerStub).respondJWKS},
	}
	for _, document := range validBodies {
		t.Run(document.name+" at the limit returns Caller", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(), testKeyID)
			document.respond(stub, respondBody(padded(document.body(stub), keyRetrievalSizeLimit)))

			assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(), testKeyID))
		})
		t.Run(document.name+" over the limit returns ErrInternalFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(), testKeyID)
			document.respond(stub, respondBody(padded(document.body(stub), keyRetrievalSizeLimit+1)))
			token := stub.token(t, testKey(), testKeyID)

			got, err := stub.verifier(t).Authenticate(t.Context(), rawIDToken(t, token))

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

		got, err := verifier.Authenticate(ctx, rawIDToken(t, token))

		assertRejected(t, got, err, ErrTransientFailure, token)
	}
	t.Run("discovery deadline exceeded returns ErrTransientFailure", func(t *testing.T) {
		stub := newIssuerStub(t, testKey(), testKeyID)
		stub.respondDiscovery(waitForDisconnect)

		authenticateWithDeadline(t, stub.verifier(t), stub.token(t, testKey(), testKeyID))
	})
	// 先に Discovery を保持し、期限が JWKS の取得の間に切れるようにする
	t.Run("jwks deadline exceeded returns ErrTransientFailure", func(t *testing.T) {
		stub := newIssuerStub(t, testKey(), testKeyID)
		verifier := stub.verifier(t)
		assertAuthenticated(t, verifier, stub.token(t, testKey(), testKeyID))
		stub.respondJWKS(waitForDisconnect)

		authenticateWithDeadline(t, verifier, stub.token(t, generateKey(), "unknown"))
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
	stub := newIssuerStub(t, testKey(), testKeyID)
	recorder := &deadlineRecorder{transport: stub.server.Client().Transport}
	verifier, err := NewVerifier(stub.issuer(), testAudience, testClock, WithTransport(recorder))
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}

	assertAuthenticated(t, verifier, stub.token(t, testKey(), testKeyID))

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
	stub := newIssuerStub(t, testKey(), testKeyID)
	stub.respondDiscovery(respondBody(discoveryBodyOf(stub.issuer(), stub.issuer()+"/"+responseMarker)))
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == discoveryPath {
			return stub.server.Client().Transport.RoundTrip(req)
		}
		return nil, errors.New("malformed HTTP response " + responseMarker)
	})
	verifier, err := NewVerifier(stub.issuer(), testAudience, testClock, WithTransport(transport))
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}
	token := stub.token(t, testKey(), testKeyID)

	got, err := verifier.Authenticate(t.Context(), rawIDToken(t, token))

	assertRejected(t, got, err, ErrTransientFailure, token)
}

func TestAuthenticate_RequestsDocumentsWithoutCache(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	recordHeader := func(next http.HandlerFunc, got *atomic.Value) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			got.Store(r.Header.Get("Cache-Control"))
			next(w, r)
		}
	}
	var discoveryHeader, jwksHeader atomic.Value
	stub.respondDiscovery(recordHeader(*stub.discovery.Load(), &discoveryHeader))
	stub.respondJWKS(recordHeader(*stub.jwks.Load(), &jwksHeader))

	assertAuthenticated(t, stub.verifier(t), stub.token(t, testKey(), testKeyID))

	if got := discoveryHeader.Load(); got != "no-cache" {
		t.Errorf("discovery Cache-Control = %v, want no-cache", got)
	}
	if got := jwksHeader.Load(); got != "no-cache" {
		t.Errorf("JWKS Cache-Control = %v, want no-cache", got)
	}
}

func TestAuthenticate_DefaultClientRejectsUntrustedCertificate(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	verifier, err := NewVerifier(stub.issuer(), testAudience, testClock)
	if err != nil {
		t.Fatalf("NewVerifier returned error: %v", err)
	}
	token := stub.token(t, testKey(), testKeyID)

	got, err := verifier.Authenticate(t.Context(), rawIDToken(t, token))

	assertRejected(t, got, err, ErrTransientFailure, token)
	if requests := stub.discoveryRequests.Load(); requests != 0 {
		t.Errorf("discovery requests = %d, want 0", requests)
	}
}

func TestAuthenticate_DiscoveryFailureIsNotKept(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	verifier := stub.verifier(t)
	token := stub.token(t, testKey(), testKeyID)
	stub.respondDiscovery(respondStatus(500))

	got, err := verifier.Authenticate(t.Context(), rawIDToken(t, token))
	assertRejected(t, got, err, ErrTransientFailure, token)

	stub.respondDiscovery(respondBody(stub.discoveryBody()))
	assertAuthenticated(t, verifier, token)
}

func TestAuthenticate_JWKSFailureKeepsDiscovery(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	verifier := stub.verifier(t)
	token := stub.token(t, testKey(), testKeyID)
	valid := *stub.jwks.Load()
	stub.respondJWKS(respondStatus(500))

	got, err := verifier.Authenticate(t.Context(), rawIDToken(t, token))
	assertRejected(t, got, err, ErrTransientFailure, token)

	stub.respondJWKS(valid)
	assertAuthenticated(t, verifier, token)
	if requests := stub.discoveryRequests.Load(); requests != 1 {
		t.Errorf("discovery requests = %d, want 1", requests)
	}
}

func TestAuthenticate_ReusesRetrievedDiscoveryAndKeys(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	verifier := stub.verifier(t)
	token := stub.token(t, testKey(), testKeyID)

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
	stub := newIssuerStub(t, testKey(), testKeyID)
	verifier := stub.verifier(t)
	assertAuthenticated(t, verifier, stub.token(t, testKey(), testKeyID))

	rotated := generateKey()
	stub.respondJWKS(respondBody(jwksBody(t, rotated, "rotated")))
	assertAuthenticated(t, verifier, stub.token(t, rotated, "rotated"))

	if requests := stub.discoveryRequests.Load(); requests != 1 {
		t.Errorf("discovery requests = %d, want 1", requests)
	}
}

func TestAuthenticate_JWKSFailureKeepsPreviousKeys(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	verifier := stub.verifier(t)
	token := stub.token(t, testKey(), testKeyID)
	assertAuthenticated(t, verifier, token)

	stub.respondJWKS(respondStatus(500))
	unknownKeyToken := stub.token(t, generateKey(), "unknown")
	got, err := verifier.Authenticate(t.Context(), rawIDToken(t, unknownKeyToken))
	assertRejected(t, got, err, ErrTransientFailure, unknownKeyToken)

	assertAuthenticated(t, verifier, token)
}

func TestAuthenticate_RetrievedKeysRejectInvalidToken(t *testing.T) {
	tests := []struct {
		name  string
		token func(t *testing.T, stub *issuerStub) string
	}{
		{"signature by another key under the same key ID", func(t *testing.T, stub *issuerStub) string {
			return stub.token(t, generateKey(), testKeyID)
		}},
		{"unknown key ID", func(t *testing.T, stub *issuerStub) string {
			return stub.token(t, generateKey(), "unknown")
		}},
		{"issuer differing from the fixed one", func(t *testing.T, _ *issuerStub) string {
			return signWithKeyID(t, jose.RS256, testKey(), testKeyID, marshalClaims(t, claimsWith(map[string]any{"iss": "https://other.example"})))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrAuthenticationFailure", func(t *testing.T) {
			stub := newIssuerStub(t, testKey(), testKeyID)
			token := tt.token(t, stub)

			got, err := stub.verifier(t).Authenticate(t.Context(), rawIDToken(t, token))

			assertRejected(t, got, err, ErrAuthenticationFailure, token)
			if errors.Is(err, ErrTransientFailure) || errors.Is(err, ErrInternalFailure) {
				t.Errorf("Authenticate error = %v, want only ErrAuthenticationFailure", err)
			}
		})
	}
}

func TestAuthenticate_NoKeyIDIsRejectedWithoutRetrieval(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	token := stub.token(t, testKey(), "")

	got, err := stub.verifier(t).Authenticate(t.Context(), rawIDToken(t, token))

	assertRejected(t, got, err, ErrAuthenticationFailure, token)
	if requests := stub.discoveryRequests.Load() + stub.jwksRequests.Load(); requests != 0 {
		t.Errorf("retrieved documents %d times, want 0", requests)
	}
}

func TestAuthenticate_ConcurrentCallsShareRetrieval(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	verifier := stub.verifier(t)
	token := rawIDToken(t, stub.token(t, testKey(), testKeyID))

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

// 記録先を共有すると、取得に失敗した呼び出しの失敗を、取得していない呼び出しが受け取る。
func TestAuthenticate_ConcurrentCallsKeepRetrievalFailureToThemselves(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	verifier := stub.verifier(t)
	assertAuthenticated(t, verifier, stub.token(t, testKey(), testKeyID))
	stub.respondJWKS(respondStatus(500))

	unknownKeyToken := rawIDToken(t, stub.token(t, generateKey(), "unknown"))
	otherAudience := marshalClaims(t, claimsWith(map[string]any{"iss": stub.issuer(), "aud": "other"}))
	rejectedToken := rawIDToken(t, signWithKeyID(t, jose.RS256, testKey(), testKeyID, otherAudience))

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

// go-oidc は解析できた JWT だけを渡すため、Authenticate からは通らない。oidc.KeySet の契約として直接確かめる。
func TestIssuerKeySetVerifySignature_MalformedTokenReturnsErrorWithoutRetrieval(t *testing.T) {
	stub := newIssuerStub(t, testKey(), testKeyID)
	issuer, err := parseIssuer(stub.issuer())
	if err != nil {
		t.Fatalf("parseIssuer returned error: %v", err)
	}
	keySet := newIssuerKeySet(issuer, WithTransport(stub.server.Client().Transport))

	if _, err := keySet.VerifySignature(t.Context(), "not-a-jwt"); err == nil {
		t.Fatal("VerifySignature returned no error")
	}
	if requests := stub.discoveryRequests.Load(); requests != 0 {
		t.Errorf("discovery requests = %d, want 0", requests)
	}
}
