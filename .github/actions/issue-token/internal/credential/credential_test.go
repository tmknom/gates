package credential_test

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"strings"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

const secret = "secret-value"

// credentials は、資格情報の型を、同じ検証に掛けるために並べる。
func credentials() map[string]any {
	aws := credential.NewAWSCredentials(secret, secret, secret)
	return map[string]any{
		"OIDCRequestToken":        credential.NewOIDCRequestToken(secret),
		"OIDCJWT":                 credential.NewOIDCJWT(secret),
		"AWSAccessKeyID":          aws.AccessKeyID,
		"AWSSecretAccessKey":      aws.SecretAccessKey,
		"AWSSessionToken":         aws.SessionToken,
		"AWSCredentials":          aws,
		"InstallationAccessToken": credential.NewInstallationAccessToken(secret),
	}
}

func TestCredentials_HideValueInOutput(t *testing.T) {
	t.Parallel()

	for name, value := range credentials() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			outputs := map[string]struct {
				format func() (string, error)
				// fmt は非公開フィールドの Format を呼ばないため、そこでは伏せた表記を求めず、値が出ないことだけを確かめる
				wantRedacted bool
			}{
				"%v":  {func() (string, error) { return fmt.Sprintf("%v", value), nil }, true},
				"%+v": {func() (string, error) { return fmt.Sprintf("%+v", value), nil }, true},
				"%s":  {func() (string, error) { return fmt.Sprintf("%s", value), nil }, true},
				"%#v": {func() (string, error) { return fmt.Sprintf("%#v", value), nil }, true},
				"%+v of an exported field": {func() (string, error) {
					return fmt.Sprintf("%+v", struct{ Value any }{value}), nil
				}, true},
				"%+v of an unexported field": {func() (string, error) {
					return fmt.Sprintf("%+v", struct{ value any }{value}), nil
				}, false},
				"encoding/json": {func() (string, error) {
					b, err := json.Marshal(value)
					return string(b), err
				}, true},
				"encoding/json/v2": {func() (string, error) {
					b, err := jsonv2.Marshal(value)
					return string(b), err
				}, true},
			}
			for output, tt := range outputs {
				t.Run(output+" does not contain the value", func(t *testing.T) {
					t.Parallel()

					got, err := tt.format()
					if err != nil {
						t.Fatalf("output error = %v", err)
					}
					if strings.Contains(got, secret) {
						t.Errorf("output = %q, must not contain the value", got)
					}
					if tt.wantRedacted && !strings.Contains(got, "[REDACTED]") {
						t.Errorf("output = %q, want the redacted notation", got)
					}
				})
			}
		})
	}
}

func TestCredentials_ExposeReturnsValue(t *testing.T) {
	t.Parallel()

	tests := map[string]func() string{
		"OIDCRequestToken":        credential.NewOIDCRequestToken(secret).Expose,
		"OIDCJWT":                 credential.NewOIDCJWT(secret).Expose,
		"InstallationAccessToken": credential.NewInstallationAccessToken(secret).Expose,
	}
	for name, expose := range tests {
		t.Run(name+" returns the value given to the constructor", func(t *testing.T) {
			t.Parallel()

			if got := expose(); got != secret {
				t.Errorf("Expose() = %q, want %q", got, secret)
			}
		})
	}
}

func TestNewAWSCredentials_KeepsEachValueInItsField(t *testing.T) {
	t.Parallel()

	got := credential.NewAWSCredentials("access-key-id", "secret-access-key", "session-token")
	values := map[string]string{
		"AccessKeyID":     got.AccessKeyID.Expose(),
		"SecretAccessKey": got.SecretAccessKey.Expose(),
		"SessionToken":    got.SessionToken.Expose(),
	}
	want := map[string]string{
		"AccessKeyID":     "access-key-id",
		"SecretAccessKey": "secret-access-key",
		"SessionToken":    "session-token",
	}
	for field, value := range values {
		if value != want[field] {
			t.Errorf("%s.Expose() = %q, want %q", field, value, want[field])
		}
	}
}
