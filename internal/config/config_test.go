package config_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/tmknom/gates/internal/authz"
	"github.com/tmknom/gates/internal/config"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/permission"
	"github.com/tmknom/gates/internal/repository"
)

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
			if owner.ID().Int64() != tt.wantOwnerID {
				t.Errorf("AllowedOwner().ID() = %d, want %d", owner.ID().Int64(), tt.wantOwnerID)
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

// 発行上限の規則は permission が検証する。ここでは、カンマで区切ったどの組も発行上限に入り、ほかの組が入らないことを確かめる
func TestLoad_IssuablePermissions(t *testing.T) {
	got, err := config.Load(lookupEnv(validEnv()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	tests := []struct {
		name     string
		levels   map[string]string
		contains bool
	}{
		{name: "first entry is contained", levels: map[string]string{"contents": "read"}, contains: true},
		{name: "last entry is contained", levels: map[string]string{"pull_requests": "write"}, contains: true},
		{name: "entry not in the variable is not contained", levels: map[string]string{"contents": "write"}, contains: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := permission.NewPermissions(tt.levels)
			if err != nil {
				t.Fatalf("NewPermissions returned error: %v", err)
			}
			if c := got.IssuablePermissions().Contains(p); c != tt.contains {
				t.Errorf("IssuablePermissions().Contains(%v) = %v, want %v", tt.levels, c, tt.contains)
			}
		})
	}
}

// 一覧の規則は protected が検証する。ここでは、環境変数の一覧が判定に使われ、保護対象のない一覧がゼロ値と区別されることを確かめる
func TestLoad_ProtectedRepositories(t *testing.T) {
	const document = `repositories:
  - id: "200"
    name: platform-infra
    callers: []
`
	got, err := config.Load(lookupEnv(withEnv(map[string]string{"GATES_PROTECTED_REPOSITORIES": document})))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	callerRepositoryID, err := githubid.NewRepositoryID(100)
	if err != nil {
		t.Fatalf("NewRepositoryID returned error: %v", err)
	}
	permissions, err := permission.NewPermissions(map[string]string{"contents": "read"})
	if err != nil {
		t.Fatalf("NewPermissions returned error: %v", err)
	}
	tests := []struct {
		name    string
		target  string
		allowed bool
	}{
		{name: "protected repository in the variable is denied", target: "Example-Org/platform-infra", allowed: false},
		{name: "repository not in the variable is allowed", target: "Example-Org/app", allowed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := repository.ParseFullName(tt.target)
			if err != nil {
				t.Fatalf("ParseFullName returned error: %v", err)
			}
			err = got.ProtectedRepositories().Authorize(callerRepositoryID, "deploy-tools", []repository.FullName{target}, permissions)
			if (err == nil) != tt.allowed {
				t.Errorf("ProtectedRepositories().Authorize() error = %v, want allowed %v", err, tt.allowed)
			}
		})
	}
}

func TestLoad_NoProtectedRepositoriesIsNotZero(t *testing.T) {
	got, err := config.Load(lookupEnv(validEnv()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.ProtectedRepositories().IsZero() {
		t.Error("ProtectedRepositories().IsZero() = true, want false for repositories: []")
	}
}

// 欠落か空の環境変数は、その誤りだけを報告し、値の条件の誤りを重ねない。直す箇所を読み違えないため
func TestLoad_MissingOrEmpty(t *testing.T) {
	for key := range validEnv() {
		t.Run("missing "+key+" returns ErrInvalidConfig", func(t *testing.T) {
			env := validEnv()
			delete(env, key)
			assertInvalid(t, env, key+": is not set")
			_, err := config.Load(lookupEnv(env))
			if n := strings.Count(err.Error(), key+": "); n != 1 {
				t.Errorf("Load() error = %q reports %s %d times, want once", err, key, n)
			}
		})
		t.Run("empty "+key+" returns ErrInvalidConfig", func(t *testing.T) {
			env := withEnv(map[string]string{key: ""})
			assertInvalid(t, env, key+": must not be empty")
			_, err := config.Load(lookupEnv(env))
			if n := strings.Count(err.Error(), key+": "); n != 1 {
				t.Errorf("Load() error = %q reports %s %d times, want once", err, key, n)
			}
		})
	}
}

// 表記ごとの規則は githubid が検証する。ここでは、表記の誤りが環境変数の名前を付けた初期化の失敗になることを確かめる
func TestLoad_InvalidOwnerID(t *testing.T) {
	env := withEnv(map[string]string{"GATES_ALLOWED_OWNER_ID": "01"})
	assertInvalid(t, env, "GATES_ALLOWED_OWNER_ID: must be a positive decimal integer")
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

// 組ごとの規則は permission が検証する。ここでは、カンマで分けた要素を、空の要素も前後の空白も残したまま渡すことと、誤りが初期化の失敗になることを確かめる
//
//trace:checks AC-AUTHZ-010
func TestLoad_InvalidIssuablePermissions(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "leading comma", value: ",contents:read"},
		{name: "trailing comma", value: "contents:read,"},
		{name: "consecutive commas", value: "contents:read,,issues:read"},
		{name: "entry with surrounding spaces", value: "contents:read, issues:read"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrInvalidConfig", func(t *testing.T) {
			env := withEnv(map[string]string{"GATES_ISSUABLE_PERMISSIONS": tt.value})
			assertInvalid(t, env, "GATES_ISSUABLE_PERMISSIONS: ")
		})
	}
}

// 一覧の規則は protected が検証する。ここでは、一覧の誤りが環境変数の名前を付けた初期化の失敗になることを確かめる
//
//trace:checks AC-AUTHZ-013
func TestLoad_InvalidProtectedRepositories(t *testing.T) {
	env := withEnv(map[string]string{"GATES_PROTECTED_REPOSITORIES": "repositories:\n  - id: 200\n    name: app\n    callers: []\n"})
	assertInvalid(t, env, "GATES_PROTECTED_REPOSITORIES: invalid protected repositories: ")
}

// 種別の規則そのものは authz が検証する。ここでは誤りが種別の環境変数の名前で報告されることだけを確かめる
func TestLoad_InvalidOwnerType(t *testing.T) {
	env := withEnv(map[string]string{"GATES_ALLOWED_OWNER_TYPE": "organization"})
	assertInvalid(t, env, "GATES_ALLOWED_OWNER_TYPE: ")
}

func TestLoad_AggregatesErrors(t *testing.T) {
	env := withEnv(map[string]string{
		"GATES_ALLOWED_OWNER_ID":       "0",
		"GATES_ALLOWED_OWNER_TYPE":     "org",
		"GATES_KMS_KEY_ARN":            "",
		"GATES_ISSUABLE_PERMISSIONS":   "members:read",
		"GATES_PROTECTED_REPOSITORIES": "repositories: {}\n",
	})
	delete(env, "GATES_GITHUB_APP_CLIENT_ID")
	assertInvalid(t, env,
		"GATES_ALLOWED_OWNER_ID: ",
		"GATES_ALLOWED_OWNER_TYPE: ",
		"GATES_GITHUB_APP_CLIENT_ID: is not set",
		"GATES_KMS_KEY_ARN: must not be empty",
		"GATES_ISSUABLE_PERMISSIONS: ",
		"GATES_PROTECTED_REPOSITORIES: ",
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
		{name: "issuable permissions with a malformed entry", overrides: map[string]string{"GATES_ISSUABLE_PERMISSIONS": "contents:read,SecretMarker"}},
		{name: "protected repositories with an invalid name", overrides: map[string]string{"GATES_PROTECTED_REPOSITORIES": "repositories:\n  - id: \"200\"\n    name: Secret/Marker\n    callers: []\n"}},
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

// validEnv は、すべての環境変数が条件を満たす組である。
func validEnv() map[string]string {
	return map[string]string{
		"GATES_ALLOWED_OWNER_ID":       "12345",
		"GATES_ALLOWED_OWNER_NAME":     "Example-Org",
		"GATES_ALLOWED_OWNER_TYPE":     "Organization",
		"GATES_GITHUB_APP_CLIENT_ID":   "Iv23liExample",
		"GATES_AUDIENCE":               "https://gates.example.com",
		"GATES_KMS_KEY_ARN":            "arn:aws:kms:ap-northeast-1:123456789012:key/11111111-2222-3333-4444-555555555555",
		"GATES_ISSUABLE_PERMISSIONS":   "contents:read,pull_requests:write",
		"GATES_PROTECTED_REPOSITORIES": "repositories: []\n",
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
