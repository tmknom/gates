package request_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/request"
)

const maxPayloadBytes = 16 * 1024

// payloadFields は、正当な Payload のフィールドを並び順どおりに持つ。値は JSON の断片である。
var payloadFields = []struct{ name, value string }{
	{name: "version", value: `1`},
	{name: "id_token", value: `"header.payload.signature"`},
	{name: "repositories", value: `["example-org/repo-a","example-org/repo-b"]`},
	{name: "permissions", value: `{"contents":"read"}`},
}

// payloadWith は、正当な Payload の name の値を value に差し替える。name が既知のフィールドでなければ末尾に加える。
func payloadWith(name, value string) string {
	members := make([]string, 0, len(payloadFields)+1)
	replaced := false
	for _, f := range payloadFields {
		if f.name == name {
			members = append(members, member(name, value))
			replaced = true
			continue
		}
		members = append(members, member(f.name, f.value))
	}
	if !replaced {
		members = append(members, member(name, value))
	}
	return "{" + strings.Join(members, ",") + "}"
}

// payloadWithout は、正当な Payload から name のフィールドを除く。
func payloadWithout(name string) string {
	members := make([]string, 0, len(payloadFields))
	for _, f := range payloadFields {
		if f.name != name {
			members = append(members, member(f.name, f.value))
		}
	}
	return "{" + strings.Join(members, ",") + "}"
}

func member(name, value string) string {
	return fmt.Sprintf("%q:%s", name, value)
}

// repositoriesJSON は、org/repo-0 から始まる n 件の完全名の配列を返す。
func repositoriesJSON(n int) string {
	names := make([]string, n)
	for i := range n {
		names[i] = fmt.Sprintf(`"org/repo-%d"`, i)
	}
	return "[" + strings.Join(names, ",") + "]"
}

// payloadOfSize は、id_token の長さを調整して size バイトちょうどの正当な形の Payload を返す。
func payloadOfSize(size int) string {
	padding := size - len(payloadWith("id_token", `""`))
	return payloadWith("id_token", `"`+strings.Repeat("a", padding)+`"`)
}

func TestValidate(t *testing.T) {
	token, req, err := request.Validate([]byte(payloadWith("repositories", `["Example-Org/Repo-A","example-org/repo-b"]`)))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if got, want := token.Expose(), "header.payload.signature"; got != want {
		t.Errorf("RawIDToken.Expose() = %q, want %q", got, want)
	}
	assertRepositories(t, req.Repositories(), []string{"Example-Org/Repo-A", "example-org/repo-b"})
	assertPermissions(t, req.Permissions(), map[string]string{"contents": "read"})
}

func TestTokenRequest_RepositoriesCannotBeModifiedByCaller(t *testing.T) {
	_, req, err := request.Validate([]byte(payloadWith("repositories", `["org/repo-a","org/repo-b"]`)))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	got := req.Repositories()
	got[0] = got[1]
	assertRepositories(t, req.Repositories(), []string{"org/repo-a", "org/repo-b"})
}

func TestValidate_KeepsPermissionsOutsideAllowedPermissions(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		permissions map[string]string
	}{
		{name: "contents write is kept", payload: payloadWith("permissions", `{"contents":"write"}`), permissions: map[string]string{"contents": "write"}},
		{name: "permission other than contents is kept", payload: payloadWith("permissions", `{"issues":"read"}`), permissions: map[string]string{"issues": "read"}},
		{name: "additional metadata read is kept", payload: payloadWith("permissions", `{"contents":"read","metadata":"read"}`), permissions: map[string]string{"contents": "read", "metadata": "read"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, req, err := request.Validate([]byte(tt.payload))
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			assertPermissions(t, req.Permissions(), tt.permissions)
		})
	}
}

func TestValidate_AcceptsBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "payload of exactly 16 KiB is accepted", payload: payloadOfSize(maxPayloadBytes)},
		{name: "500 repositories are accepted", payload: payloadWith("repositories", repositoriesJSON(500))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := request.Validate([]byte(tt.payload)); err != nil {
				t.Errorf("Validate returned error: %v", err)
			}
		})
	}
}

func TestValidate_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    []error
	}{
		// Payload 全体
		{name: "payload over 16 KiB returns ErrPayloadTooLarge", payload: payloadOfSize(maxPayloadBytes + 1), want: []error{request.ErrPayloadTooLarge}},
		{name: "empty payload returns ErrMalformedPayload", payload: ``, want: []error{request.ErrMalformedPayload}},
		{name: "truncated JSON returns ErrMalformedPayload", payload: `{"version":1,`, want: []error{request.ErrMalformedPayload}},
		{name: "invalid JSON returns ErrMalformedPayload", payload: `x`, want: []error{request.ErrMalformedPayload}},
		{name: "top-level null returns ErrMalformedPayload", payload: `null`, want: []error{request.ErrMalformedPayload}},
		{name: "top-level array returns ErrMalformedPayload", payload: `[1]`, want: []error{request.ErrMalformedPayload}},
		{name: "top-level string returns ErrMalformedPayload", payload: `"x"`, want: []error{request.ErrMalformedPayload}},
		{name: "top-level number returns ErrMalformedPayload", payload: `1`, want: []error{request.ErrMalformedPayload}},
		{name: "duplicate top-level key returns ErrMalformedPayload", payload: `{"version":1,"version":1,"id_token":"t","repositories":["org/repo"],"permissions":{"contents":"read"}}`, want: []error{request.ErrMalformedPayload}},
		{name: "duplicate key in permissions returns ErrMalformedPayload", payload: payloadWith("permissions", `{"contents":"write","contents":"read"}`), want: []error{request.ErrMalformedPayload}},
		{name: "invalid UTF-8 returns ErrMalformedPayload", payload: payloadWith("repositories", "[\"org/repo\xff\"]"), want: []error{request.ErrMalformedPayload}},
		{name: "data after the object returns ErrMalformedPayload", payload: payloadWith("version", `1`) + ` {}`, want: []error{request.ErrMalformedPayload}},
		{name: "unknown caller field returns ErrUnknownField", payload: payloadWith("caller", `"org/repo"`), want: []error{request.ErrUnknownField}},
		{name: "unknown owner_id field returns ErrUnknownField", payload: payloadWith("owner_id", `1`), want: []error{request.ErrUnknownField}},
		{name: "unknown installation_id field returns ErrUnknownField", payload: payloadWith("installation_id", `1`), want: []error{request.ErrUnknownField}},
		{name: "unknown api_host field returns ErrUnknownField", payload: payloadWith("api_host", `"https://api.example.com"`), want: []error{request.ErrUnknownField}},
		{name: "repository_selection for all repositories returns ErrUnknownField", payload: payloadWith("repository_selection", `"all"`), want: []error{request.ErrUnknownField}},
		{name: "field name in different case returns ErrUnknownField", payload: payloadWith("Version", `1`), want: []error{request.ErrUnknownField}},

		// version
		{name: "missing version returns ErrInvalidVersion", payload: payloadWithout("version"), want: []error{request.ErrInvalidVersion}},
		{name: "null version returns ErrInvalidVersion", payload: payloadWith("version", `null`), want: []error{request.ErrInvalidVersion}},
		{name: "version 0 returns ErrInvalidVersion", payload: payloadWith("version", `0`), want: []error{request.ErrInvalidVersion}},
		{name: "version 2 returns ErrInvalidVersion", payload: payloadWith("version", `2`), want: []error{request.ErrInvalidVersion}},
		{name: "version 1.0 returns ErrInvalidVersion", payload: payloadWith("version", `1.0`), want: []error{request.ErrInvalidVersion}},
		{name: "version string returns ErrInvalidVersion", payload: payloadWith("version", `"1"`), want: []error{request.ErrInvalidVersion}},
		{name: "version boolean returns ErrInvalidVersion", payload: payloadWith("version", `true`), want: []error{request.ErrInvalidVersion}},

		// id_token
		{name: "missing id_token returns ErrInvalidIDToken", payload: payloadWithout("id_token"), want: []error{request.ErrInvalidIDToken}},
		{name: "null id_token returns ErrInvalidIDToken", payload: payloadWith("id_token", `null`), want: []error{request.ErrInvalidIDToken}},
		{name: "non-string id_token returns ErrInvalidIDToken", payload: payloadWith("id_token", `5`), want: []error{request.ErrInvalidIDToken}},
		{name: "empty id_token returns ErrInvalidIDToken", payload: payloadWith("id_token", `""`), want: []error{request.ErrInvalidIDToken}},

		// repositories
		{name: "missing repositories returns ErrInvalidRepositories", payload: payloadWithout("repositories"), want: []error{request.ErrInvalidRepositories}},
		{name: "null repositories returns ErrInvalidRepositories", payload: payloadWith("repositories", `null`), want: []error{request.ErrInvalidRepositories}},
		{name: "non-array repositories returns ErrInvalidRepositories", payload: payloadWith("repositories", `"org/repo"`), want: []error{request.ErrInvalidRepositories}},
		{name: "empty repositories returns ErrInvalidRepositories", payload: payloadWith("repositories", `[]`), want: []error{request.ErrInvalidRepositories}},
		{name: "non-string element returns ErrInvalidRepositories", payload: payloadWith("repositories", `[7]`), want: []error{request.ErrInvalidRepositories}},
		{name: "null element returns ErrInvalidRepositories and ErrInvalidFullName", payload: payloadWith("repositories", `[null]`), want: []error{request.ErrInvalidRepositories, repository.ErrInvalidFullName}},
		{name: "element without slash returns ErrInvalidRepositories and ErrInvalidFullName", payload: payloadWith("repositories", `["repo"]`), want: []error{request.ErrInvalidRepositories, repository.ErrInvalidFullName}},
		{name: "501 repositories return ErrInvalidRepositories", payload: payloadWith("repositories", repositoriesJSON(501)), want: []error{request.ErrInvalidRepositories}},
		{name: "duplicate full names return ErrInvalidRepositories", payload: payloadWith("repositories", `["org/repo","org/repo"]`), want: []error{request.ErrInvalidRepositories}},
		{name: "full names differing only in case return ErrInvalidRepositories", payload: payloadWith("repositories", `["org/repo","ORG/REPO"]`), want: []error{request.ErrInvalidRepositories}},

		// permissions
		{name: "missing permissions returns ErrInvalidPermissions", payload: payloadWithout("permissions"), want: []error{request.ErrInvalidPermissions}},
		{name: "null permissions returns ErrInvalidPermissions", payload: payloadWith("permissions", `null`), want: []error{request.ErrInvalidPermissions}},
		{name: "non-object permissions returns ErrInvalidPermissions", payload: payloadWith("permissions", `["contents"]`), want: []error{request.ErrInvalidPermissions}},
		{name: "empty permissions returns ErrInvalidPermissions and ErrMalformedPermissions", payload: payloadWith("permissions", `{}`), want: []error{request.ErrInvalidPermissions, permission.ErrMalformedPermissions}},
		{name: "non-string level returns ErrInvalidPermissions", payload: payloadWith("permissions", `{"contents":1}`), want: []error{request.ErrInvalidPermissions}},
		{name: "null level returns ErrInvalidPermissions and ErrMalformedPermissions", payload: payloadWith("permissions", `{"contents":null}`), want: []error{request.ErrInvalidPermissions, permission.ErrMalformedPermissions}},
		{name: "empty level returns ErrInvalidPermissions and ErrMalformedPermissions", payload: payloadWith("permissions", `{"contents":""}`), want: []error{request.ErrInvalidPermissions, permission.ErrMalformedPermissions}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := request.Validate([]byte(tt.payload))
			for _, want := range tt.want {
				if !errors.Is(err, want) {
					t.Errorf("Validate error = %v, want %v", err, want)
				}
			}
		})
	}
}

// TestValidate_ErrorDoesNotContainPayload は、資格情報の非記録のため、Payload の断片がエラー文言に出ないことを確かめる。
func TestValidate_ErrorDoesNotContainPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "id_token is not in error message", payload: strings.Replace(payloadWith("id_token", `"secret"`), `"version":1`, `"version":2`, 1)},
		{name: "unknown field name is not in error message", payload: payloadWith("secret", `1`)},
		{name: "duplicate key name is not in error message", payload: payloadWith("permissions", `{"secret":"read","secret":"read"}`)},
		{name: "permission name with wrong type is not in error message", payload: payloadWith("permissions", `{"secret":1}`)},
		{name: "invalid repository element is not in error message", payload: payloadWith("repositories", `["secret!/repo"]`)},
		{name: "invalid character is not in error message", payload: payloadWith("version", `1`) + ` secret`},
		{name: "number literal is not in error message", payload: payloadWith("version", `12345.5`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := request.Validate([]byte(tt.payload))
			if err == nil {
				t.Fatalf("Validate(%s) returned no error", tt.payload)
			}
			for _, fragment := range []string{"secret", "12345"} {
				if strings.Contains(err.Error(), fragment) {
					t.Errorf("error message %q contains the payload fragment %q", err.Error(), fragment)
				}
			}
		})
	}
}

func assertRepositories(t *testing.T, got []repository.FullName, want []string) {
	t.Helper()
	// FullName.Equal は大文字小文字を区別しないため、要求どおりの表記が保たれたかは String() の列で比べる
	names := make([]string, len(got))
	for i, f := range got {
		names[i] = f.String()
	}
	if !slices.Equal(names, want) {
		t.Errorf("Repositories() = %v, want %v", names, want)
	}
}

func assertPermissions(t *testing.T, got permission.Permissions, want map[string]string) {
	t.Helper()
	w, err := permission.NewPermissions(want)
	if err != nil {
		t.Fatalf("NewPermissions(%v) returned error: %v", want, err)
	}
	if !got.Equal(w) {
		t.Errorf("Permissions() is not equal to %v", want)
	}
}
