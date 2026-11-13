package handler

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/authn"
	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/caller"
	"github.com/tmknom/gates/internal/nonzero"
	"github.com/tmknom/gates/internal/request"
	"github.com/tmknom/gates/internal/signer"
	"github.com/tmknom/gates/internal/tokenissuer"
)

// verifierStub は OIDC Verifier のテストダブルである。
type verifierStub struct {
	caller caller.Caller
	err    error
	panics bool
	calls  int
}

func (v *verifierStub) Authenticate(context.Context, request.UnverifiedIDToken) (caller.Caller, error) {
	v.calls++
	if v.panics {
		panicWithSecret()
	}
	return v.caller, v.err
}

// tokenIssuerStub は Token Issuer のテストダブルである。
// installation があれば、受け取った GrantFunc にそれを渡し、Grant が拒否すればその error を返す。
// 成功は照合を経た Token Issuer からしか起きないため、err も panics も持たない stub は、呼ばれないことを確かめる場合にだけ使う。成功は newTokenIssuer の Token Issuer で作る。
type tokenIssuerStub struct {
	installation *authz.Installation
	err          error
	panics       bool
	calls        int
	ctx          context.Context
}

func (i *tokenIssuerStub) IssueInstallationToken(ctx context.Context, grant tokenissuer.GrantFunc) (tokenissuer.InstallationToken, error) {
	i.calls++
	i.ctx = ctx
	if i.installation != nil {
		if _, err := grant(*i.installation); err != nil {
			return tokenissuer.InstallationToken{}, err
		}
	}
	if i.panics {
		panicWithSecret()
	}
	return tokenissuer.InstallationToken{}, i.err
}

// secretMarker は、nonzero.Require の panic の値に型名として入る目印である。
// panic の値が応答、監査記録、ログのどれにも現れないことを、この名前が現れないことで確かめる。
// forbidigo がテストでも panic を禁じるため、目印を値に入れるのは nonzero.Require の文言に頼る。頼れることは TestPanicWithSecret_ValueContainsMarker が確かめる。
type secretMarker struct{}

func (secretMarker) IsZero() bool {
	return true
}

func panicWithSecret() {
	nonzero.Require(secretMarker{})
}

// invocation は、Handle の応答と、Handle が書いたログの行である。
type invocation struct {
	response map[string]any
	logs     []map[string]any
}

// handle は Handle を呼び、応答とログの行を返す。
// どのケースでも、資格情報と panic の値が成功応答の token 以外に現れないことを確かめる。
func handle(t *testing.T, verifier oidcVerifier, tokenIssuer installationTokenIssuer, payload []byte) invocation {
	t.Helper()
	var log bytes.Buffer
	requestID, err := NewRequestID(lambdaRequestID)
	if err != nil {
		t.Fatalf("NewRequestID returned error: %v", err)
	}
	h := NewHandler(verifier, tokenIssuer, allowedOwner(t), slog.New(slog.NewJSONHandler(&log, nil)))

	b := h.Handle(t.Context(), requestID, payload)

	decoded := decodeResponse(t, b)
	withoutToken := map[string]any{}
	for key, value := range decoded {
		if key != "token" {
			withoutToken[key] = value
		}
	}
	for name, output := range map[string]string{"response": string(marshal(t, withoutToken)), "log": log.String()} {
		for _, marker := range []string{idTokenMarker, tokenMarker, "secretMarker"} {
			if strings.Contains(output, marker) {
				t.Errorf("%s contains %q: %s", name, marker, output)
			}
		}
	}
	var logs []map[string]any
	for line := range strings.Lines(log.String()) {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line %q is not a JSON object: %v", line, err)
		}
		logs = append(logs, record)
	}
	return invocation{response: decoded, logs: logs}
}

// auditRecord は、ログの行のうち、ただ 1 行の監査記録を返す。
func (inv invocation) auditRecord(t *testing.T) map[string]any {
	t.Helper()
	var records []map[string]any
	for _, record := range inv.logs {
		if record["msg"] == "audit record" {
			records = append(records, record)
		}
	}
	if len(records) != 1 {
		t.Fatalf("audit records = %v, want exactly one", records)
	}
	if records[0]["request_id"] != lambdaRequestID {
		t.Errorf("audit record request_id = %v, want %q", records[0]["request_id"], lambdaRequestID)
	}
	return records[0]
}

// assertFailed は、失敗応答のエラーコードと相関 ID、監査記録の結果を確かめ、監査記録を返す。
func (inv invocation) assertFailed(t *testing.T, code string) map[string]any {
	t.Helper()
	if inv.response["ok"] != false {
		t.Errorf("ok = %v, want false", inv.response["ok"])
	}
	if got, _ := inv.response["error"].(map[string]any); got["code"] != code {
		t.Errorf("error = %v, want code %q", inv.response["error"], code)
	}
	if inv.response["request_id"] != lambdaRequestID {
		t.Errorf("request_id = %v, want %q", inv.response["request_id"], lambdaRequestID)
	}
	record := inv.auditRecord(t)
	if record["result"] != code {
		t.Errorf("audit record result = %v, want %q", record["result"], code)
	}
	return record
}

// auditKeys は、常に記録する項目と slog のキーに、記録する条件を満たした項目のキーを加える。
func auditKeys(keys ...string) []string {
	return append([]string{"time", "level", "msg", "request_id", "result"}, keys...)
}

func installation(t *testing.T, installationOwnerID int64, suspendedAt time.Time) *authz.Installation {
	t.Helper()
	inst, err := authz.NewInstallation(installationID, installationOwnerID, suspendedAt)
	if err != nil {
		t.Fatalf("NewInstallation returned error: %v", err)
	}
	return &inst
}

//trace:checks AC-01 AC-18
func TestHandle_IssuesTokenAndWritesOneAuditRecordOfSuccess(t *testing.T) {
	inv := handle(t, &verifierStub{caller: verifiedCaller(t)}, newTokenIssuer(t, recordedIssuance(), http.StatusNoContent), validPayload(t))

	if inv.response["ok"] != true || inv.response["token"] != tokenMarker || inv.response["request_id"] != lambdaRequestID {
		t.Errorf("response = %v, want the success response with the issued token and the request ID", inv.response)
	}
	record := inv.auditRecord(t)
	assertKeys(t, "audit record", record, auditKeys("caller", "request", "installation")...)
	if record["result"] != "success" {
		t.Errorf("result = %v, want success", record["result"])
	}
	if got, _ := record["caller"].(map[string]any); got["owner_id"] != float64(allowedOwnerID) {
		t.Errorf("caller = %v, want owner_id %d", record["caller"], allowedOwnerID)
	}
	wantRequest := map[string]any{
		"repositories": []any{allowedOwnerName + "/repo-b", allowedOwnerName + "/Example-Repo"},
		"permissions":  map[string]any{"contents": "read"},
	}
	if !reflect.DeepEqual(record["request"], wantRequest) {
		t.Errorf("request = %v, want %v", record["request"], wantRequest)
	}
	if want := map[string]any{"id": float64(installationID), "owner_id": float64(allowedOwnerID)}; !reflect.DeepEqual(record["installation"], want) {
		t.Errorf("installation = %v, want %v", record["installation"], want)
	}
}

//trace:checks AC-08
func TestHandle_InvalidRequestDoesNotAuthenticate(t *testing.T) {
	verifier := &verifierStub{caller: verifiedCaller(t)}
	tokenIssuer := &tokenIssuerStub{}

	// GATES 用 OIDC JWT は正しい形のまま、対象だけを不正にし、拒否の理由に JWT が入らないことも確かめる
	inv := handle(t, verifier, tokenIssuer, marshal(t, map[string]any{
		"version":      1,
		"id_token":     idTokenMarker,
		"repositories": []string{},
		"permissions":  map[string]string{"contents": "read"},
	}))

	record := inv.assertFailed(t, "invalid_request")
	assertKeys(t, "audit record", record, auditKeys("reason")...)
	if verifier.calls != 0 || tokenIssuer.calls != 0 {
		t.Errorf("Authenticate calls = %d, Token Issuer calls = %d, want neither called", verifier.calls, tokenIssuer.calls)
	}
}

//trace:checks AC-02 AC-13 AC-17
func TestHandle_AuthenticationFailureStopsBeforeAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{name: "rejected ID token returns invalid_id_token", err: fmt.Errorf("%w: signature mismatch", authn.ErrAuthenticationFailure), code: "invalid_id_token"},
		{name: "transient key retrieval failure returns identity_verification_unavailable", err: fmt.Errorf("%w: JWKS timeout", authn.ErrTransientFailure), code: "identity_verification_unavailable"},
		{name: "unexpected provider response returns internal_error", err: fmt.Errorf("%w: issuer mismatch", authn.ErrInternalFailure), code: "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenIssuer := &tokenIssuerStub{}

			inv := handle(t, &verifierStub{err: tc.err}, tokenIssuer, validPayload(t))

			record := inv.assertFailed(t, tc.code)
			assertKeys(t, "audit record", record, auditKeys("reason", "request")...)
			if record["reason"] != tc.err.Error() {
				t.Errorf("reason = %v, want %q", record["reason"], tc.err.Error())
			}
			if tokenIssuer.calls != 0 {
				t.Errorf("Token Issuer calls = %d, want 0", tokenIssuer.calls)
			}
		})
	}
}

// Token Issuer が呼ばれないことで、KMS と GitHub API に進まないことを確かめる。
//
//trace:checks AC-02 AC-04 AC-18
func TestHandle_AuthorizationDeniedDoesNotCallTokenIssuer(t *testing.T) {
	tokenIssuer := &tokenIssuerStub{}

	inv := handle(t, &verifierStub{caller: callerOf(t, allowedOwnerID+1)}, tokenIssuer, validPayload(t))

	record := inv.assertFailed(t, "authorization_denied")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller")...)
	if tokenIssuer.calls != 0 {
		t.Errorf("Token Issuer calls = %d, want 0", tokenIssuer.calls)
	}
}

// Grant がどの条件で拒否するかは Authorization が決めるため、ここでは拒否されても解決した Installation が監査記録に残ることだけを確かめる。
// Grant の認可拒否は、Authorize と同じ認可条件を判定し直すため、Authorize を通った要求では起きない。
func TestHandle_GrantRejectionKeepsResolvedInstallation(t *testing.T) {
	// インストール先 Owner ID が許可オーナーと異なり、監査記録の owner_id が許可オーナーの値と見分けられる
	inst := installation(t, allowedOwnerID+1, time.Time{})

	inv := handle(t, &verifierStub{caller: verifiedCaller(t)}, &tokenIssuerStub{installation: inst}, validPayload(t))

	record := inv.assertFailed(t, "installation_resolution_failed")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller", "installation")...)
	if want := map[string]any{"id": float64(installationID), "owner_id": float64(allowedOwnerID + 1)}; !reflect.DeepEqual(record["installation"], want) {
		t.Errorf("installation = %v, want %v", record["installation"], want)
	}
}

// 失敗の種別とエラーコードの対応は errorCodeOf が決めるため、ここでは写したエラーコードが応答と監査記録の両方に出て、拒否の理由が部品の文言であることだけを確かめる。
//
//trace:checks AC-13
func TestHandle_TokenIssuerFailureWritesErrorCodeAndReason(t *testing.T) {
	err := fmt.Errorf("%w: KMS throttled", signer.ErrTransientFailure)

	inv := handle(t, &verifierStub{caller: verifiedCaller(t)}, &tokenIssuerStub{err: err}, validPayload(t))

	record := inv.assertFailed(t, "signing_unavailable")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller")...)
	if record["reason"] != err.Error() {
		t.Errorf("reason = %v, want %q", record["reason"], err.Error())
	}
}

//trace:checks AC-12 AC-18
func TestHandle_VerificationFailureWritesRevocationOutcome(t *testing.T) {
	allRepositories := recordedIssuance()
	allRepositories["repository_selection"] = "all"

	inv := handle(t, &verifierStub{caller: verifiedCaller(t)}, newTokenIssuer(t, allRepositories, http.StatusNoContent), validPayload(t))

	record := inv.assertFailed(t, "issuance_verification_failed")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller", "installation", "revocation")...)
	if record["revocation"] != "succeeded" {
		t.Errorf("revocation = %v, want %q", record["revocation"], "succeeded")
	}
}

// 失効の結果を持たない照合失敗は Token Issuer の誤りで、試みなかったとも試みたとも記録できないため、internal_error で終える。
func TestHandle_VerificationFailureWithoutRevocationOutcomeReturnsInternalError(t *testing.T) {
	err := fmt.Errorf("%w: effective repositories do not match", tokenissuer.ErrVerificationFailure)

	inv := handle(t, &verifierStub{caller: verifiedCaller(t)}, &tokenIssuerStub{err: err}, validPayload(t))

	record := inv.assertFailed(t, "internal_error")
	assertKeys(t, "audit record", record, auditKeys("reason", "request", "caller")...)
	if record["reason"] != reasonPanicRecovered {
		t.Errorf("reason = %v, want %q", record["reason"], reasonPanicRecovered)
	}
}

func TestHandle_PassesRequestContextToTokenIssuer(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), now)
	defer cancel()
	tokenIssuer := &tokenIssuerStub{err: errors.New("unexpected failure")}
	requestID, err := NewRequestID(lambdaRequestID)
	if err != nil {
		t.Fatalf("NewRequestID returned error: %v", err)
	}

	NewHandler(&verifierStub{caller: verifiedCaller(t)}, tokenIssuer, allowedOwner(t), slog.New(slog.DiscardHandler)).Handle(ctx, requestID, validPayload(t))

	if tokenIssuer.ctx != ctx {
		t.Errorf("Token Issuer received ctx %v, want the request ctx %v", tokenIssuer.ctx, ctx)
	}
}

func TestHandle_RecoversPanicOfComponent(t *testing.T) {
	for _, tc := range []struct {
		name        string
		verifier    *verifierStub
		tokenIssuer *tokenIssuerStub
		keys        []string
	}{
		{name: "panic of OIDC Verifier keeps the token request", verifier: &verifierStub{panics: true}, tokenIssuer: &tokenIssuerStub{}, keys: auditKeys("reason", "request")},
		{name: "panic of Token Issuer keeps the values settled before it", verifier: &verifierStub{caller: verifiedCaller(t)}, tokenIssuer: &tokenIssuerStub{installation: installation(t, allowedOwnerID, time.Time{}), panics: true}, keys: auditKeys("reason", "request", "caller", "installation")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv := handle(t, tc.verifier, tc.tokenIssuer, validPayload(t))

			record := inv.assertFailed(t, "internal_error")
			assertKeys(t, "audit record", record, tc.keys...)
			if record["reason"] != reasonPanicRecovered {
				t.Errorf("reason = %v, want the fixed reason %q", record["reason"], reasonPanicRecovered)
			}
		})
	}
}

//trace:checks AC-19
func TestHandle_LogsRecoveredPanicWithoutItsValue(t *testing.T) {
	inv := handle(t, &verifierStub{panics: true}, &tokenIssuerStub{}, validPayload(t))

	var errorLogs []map[string]any
	for _, record := range inv.logs {
		if record["level"] == slog.LevelError.String() {
			errorLogs = append(errorLogs, record)
		}
	}
	if len(errorLogs) != 1 {
		t.Fatalf("ERROR logs = %v, want exactly one", errorLogs)
	}
	assertKeys(t, "panic log", errorLogs[0], "time", "level", "msg", "request_id", "stack")
	if errorLogs[0]["msg"] != "panic recovered" || errorLogs[0]["request_id"] != lambdaRequestID {
		t.Errorf("panic log = %v, want msg %q and request_id %q", errorLogs[0], "panic recovered", lambdaRequestID)
	}
	if stack, _ := errorLogs[0]["stack"].(string); !strings.Contains(stack, "Authenticate") {
		t.Errorf("stack = %q, want the stack trace through the panicking Authenticate", stack)
	}
}

func TestPanicWithSecret_ValueContainsMarker(t *testing.T) {
	defer func() {
		if got := fmt.Sprint(recover()); !strings.Contains(got, "secretMarker") {
			t.Errorf("panic value = %q, want it to contain secretMarker", got)
		}
	}()

	panicWithSecret()
}

func TestNewHandler_ZeroAllowedOwnerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewHandler did not panic for the zero AllowedOwner")
		}
	}()

	_ = NewHandler(&verifierStub{}, &tokenIssuerStub{}, authz.AllowedOwner{}, slog.New(slog.DiscardHandler))
}
