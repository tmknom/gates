package config_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/config"
)

// validEnv は、6 つの環境変数がすべて条件を満たす組である。
func validEnv() map[string]string {
	return map[string]string{
		"GATES_ALLOWED_OWNER_ID":     "12345",
		"GATES_ALLOWED_OWNER_NAME":   "Example-Org",
		"GATES_ALLOWED_OWNER_TYPE":   "Organization",
		"GATES_GITHUB_APP_CLIENT_ID": "Iv23liExample",
		"GATES_AUDIENCE":             "https://gates.example.com",
		"GATES_KMS_KEY_ARN":          "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555",
	}
}

func lookupEnv(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func withEnv(overrides map[string]string) map[string]string {
	env := validEnv()
	maps.Copy(env, overrides)
	return env
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name          string
		overrides     map[string]string
		wantOwnerID   int64
		wantOwnerType authz.OwnerType
		wantAudience  string
	}{
		{
			name:          "organization owner is loaded",
			wantOwnerID:   12345,
			wantOwnerType: authz.OwnerTypeOrganization,
			wantAudience:  "https://gates.example.com",
		},
		{
			name:          "largest int64 owner id is loaded",
			overrides:     map[string]string{"GATES_ALLOWED_OWNER_ID": "9223372036854775807"},
			wantOwnerID:   9223372036854775807,
			wantOwnerType: authz.OwnerTypeOrganization,
			wantAudience:  "https://gates.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.Load(lookupEnv(withEnv(tt.overrides)))
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			owner := got.AllowedOwner()
			if owner.ID() != tt.wantOwnerID {
				t.Errorf("AllowedOwner().ID() = %d, want %d", owner.ID(), tt.wantOwnerID)
			}
			if owner.Name().String() != "Example-Org" {
				t.Errorf("AllowedOwner().Name() = %q, want %q", owner.Name(), "Example-Org")
			}
			if owner.Type() != tt.wantOwnerType {
				t.Errorf("AllowedOwner().Type() = %q, want %q", owner.Type(), tt.wantOwnerType)
			}
			if got.ClientID() != "Iv23liExample" {
				t.Errorf("ClientID() = %q, want %q", got.ClientID(), "Iv23liExample")
			}
			if got.Audience() != tt.wantAudience {
				t.Errorf("Audience() = %q, want %q", got.Audience(), tt.wantAudience)
			}
			wantKMSKeyARN := validEnv()["GATES_KMS_KEY_ARN"]
			if got.KMSKeyARN() != wantKMSKeyARN {
				t.Errorf("KMSKeyARN() = %q, want %q", got.KMSKeyARN(), wantKMSKeyARN)
			}
		})
	}
}

func assertInvalid(t *testing.T, env map[string]string, want ...string) {
	t.Helper()
	_, err := config.Load(lookupEnv(env))
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("Load() error = %v, want %v", err, config.ErrInvalidConfig)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("Load() error = %q, want it to contain %q", err, w)
		}
	}
}

func TestLoad_MissingOrEmpty(t *testing.T) {
	for key := range validEnv() {
		t.Run("missing "+key+" returns ErrInvalidConfig", func(t *testing.T) {
			env := validEnv()
			delete(env, key)
			assertInvalid(t, env, key+": is not set")
		})
		t.Run("empty "+key+" returns ErrInvalidConfig", func(t *testing.T) {
			assertInvalid(t, withEnv(map[string]string{key: ""}), key+": must not be empty")
		})
	}
}

func TestLoad_InvalidOwnerID(t *testing.T) {
	tests := []struct {
		name    string
		ownerID string
	}{
		{name: "zero", ownerID: "0"},
		{name: "negative", ownerID: "-1"},
		{name: "plus sign", ownerID: "+1"},
		{name: "leading zero", ownerID: "01"},
		{name: "non-digit", ownerID: "12a"},
		{name: "underscore", ownerID: "1_000"},
		{name: "int64 overflow", ownerID: "9223372036854775808"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" owner id returns ErrInvalidConfig", func(t *testing.T) {
			env := withEnv(map[string]string{"GATES_ALLOWED_OWNER_ID": tt.ownerID})
			assertInvalid(t, env, "GATES_ALLOWED_OWNER_ID: must be a positive decimal integer")
		})
	}
}

func TestLoad_InvalidOwnerName(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
	}{
		{name: "underscore", overrides: map[string]string{"GATES_ALLOWED_OWNER_NAME": "Example_Org"}},
		// Owner ID か種別が不正なときも、オーナー名の誤りを報告する
		{name: "with invalid owner id", overrides: map[string]string{"GATES_ALLOWED_OWNER_NAME": "Example_Org", "GATES_ALLOWED_OWNER_ID": "0"}},
		{name: "with invalid owner type", overrides: map[string]string{"GATES_ALLOWED_OWNER_NAME": "Example_Org", "GATES_ALLOWED_OWNER_TYPE": "org"}},
	}
	for _, tt := range tests {
		t.Run(tt.name+" owner name returns ErrInvalidConfig", func(t *testing.T) {
			assertInvalid(t, withEnv(tt.overrides), "GATES_ALLOWED_OWNER_NAME: ")
		})
	}
}

// 種別の規則そのものは authz が検証する。ここでは誤りが種別の環境変数の名前で報告されることだけを確かめる
func TestLoad_InvalidOwnerType(t *testing.T) {
	env := withEnv(map[string]string{"GATES_ALLOWED_OWNER_TYPE": "organization"})
	assertInvalid(t, env, "GATES_ALLOWED_OWNER_TYPE: ")
}

func TestLoad_AggregatesErrors(t *testing.T) {
	env := withEnv(map[string]string{
		"GATES_ALLOWED_OWNER_ID":   "0",
		"GATES_ALLOWED_OWNER_TYPE": "org",
		"GATES_KMS_KEY_ARN":        "",
	})
	delete(env, "GATES_GITHUB_APP_CLIENT_ID")
	assertInvalid(t, env,
		"GATES_ALLOWED_OWNER_ID: ",
		"GATES_ALLOWED_OWNER_TYPE: ",
		"GATES_GITHUB_APP_CLIENT_ID: is not set",
		"GATES_KMS_KEY_ARN: must not be empty",
	)
}

func TestLoad_ErrorsDoNotContainValues(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
	}{
		{name: "owner id", overrides: map[string]string{"GATES_ALLOWED_OWNER_ID": "98765432109876543210"}},
		{name: "owner id with a non-digit", overrides: map[string]string{"GATES_ALLOWED_OWNER_ID": "SecretMarker1"}},
		{name: "owner name", overrides: map[string]string{"GATES_ALLOWED_OWNER_NAME": "Secret_Marker"}},
		{name: "owner type", overrides: map[string]string{"GATES_ALLOWED_OWNER_TYPE": "SecretMarker"}},
	}
	for _, tt := range tests {
		t.Run("invalid "+tt.name+" is not written in the error", func(t *testing.T) {
			_, err := config.Load(lookupEnv(withEnv(tt.overrides)))
			if err == nil {
				t.Fatal("Load returned no error")
			}
			for key, value := range tt.overrides {
				if strings.Contains(err.Error(), value) {
					t.Errorf("Load() error = %q contains the value of %s", err, key)
				}
			}
		})
	}
}
