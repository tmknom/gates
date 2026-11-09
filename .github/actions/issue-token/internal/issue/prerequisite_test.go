package issue

import (
	"strings"
	"testing"
)

// satisfiedConfig は、すべての前提を満たす Config である。
func satisfiedConfig() Config {
	return Config{
		Repositories:     "example-org/repo-a",
		Permissions:      "contents:read",
		RoleARN:          "arn:aws:iam::123456789012:role/gates-role",
		FunctionName:     "gates-function",
		AWSRegion:        "ap-northeast-1",
		Audience:         "gates-audience",
		OIDCRequestURL:   "https://token.example/token?api-version=2.0",
		OIDCRequestToken: "request-token",
		OutputPath:       "/github/file_commands/output",
		StatePath:        "/github/file_commands/state",
	}
}

func TestCheckPrerequisites_SatisfiedReturnsNil(t *testing.T) {
	t.Parallel()

	if err := checkPrerequisites(satisfiedConfig()); err != nil {
		t.Errorf("checkPrerequisites() error = %v, want nil", err)
	}
}

func TestCheckPrerequisites_WhitespaceInputIsNotEmpty(t *testing.T) {
	t.Parallel()

	cfg := satisfiedConfig()
	cfg.Repositories = " "
	if err := checkPrerequisites(cfg); err != nil {
		t.Errorf("checkPrerequisites() error = %v, want nil because input validation judges the elements", err)
	}
}

func TestCheckPrerequisites_ReportsUnmetPrerequisite(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		modify func(*Config)
		want   string
	}{
		"empty request token asks for id-token: write": {
			modify: func(c *Config) { c.OIDCRequestToken = "" },
			want:   "ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set; declare permissions: id-token: write in the workflow",
		},
		"empty request URL": {
			modify: func(c *Config) { c.OIDCRequestURL = "" },
			want:   "ACTIONS_ID_TOKEN_REQUEST_URL is not set",
		},
		"empty GITHUB_OUTPUT": {
			modify: func(c *Config) { c.OutputPath = "" },
			want:   "GITHUB_OUTPUT is not set",
		},
		"empty GITHUB_STATE": {
			modify: func(c *Config) { c.StatePath = "" },
			want:   "GITHUB_STATE is not set",
		},
		"empty repositories": {
			modify: func(c *Config) { c.Repositories = "" },
			want:   "input repositories is empty",
		},
		"empty permissions": {
			modify: func(c *Config) { c.Permissions = "" },
			want:   "input permissions is empty",
		},
		"empty role-arn": {
			modify: func(c *Config) { c.RoleARN = "" },
			want:   "input role-arn is empty",
		},
		"empty function-name": {
			modify: func(c *Config) { c.FunctionName = "" },
			want:   "input function-name is empty",
		},
		"empty aws-region": {
			modify: func(c *Config) { c.AWSRegion = "" },
			want:   "input aws-region is empty",
		},
		"empty audience": {
			modify: func(c *Config) { c.Audience = "" },
			want:   "input audience is empty",
		},
	}
	for name, tt := range tests {
		t.Run(name+" returns its message", func(t *testing.T) {
			t.Parallel()

			cfg := satisfiedConfig()
			tt.modify(&cfg)
			if err := checkPrerequisites(cfg); err == nil || err.Error() != tt.want {
				t.Errorf("checkPrerequisites() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCheckPrerequisites_ReportsAllUnmetPrerequisites(t *testing.T) {
	t.Parallel()

	cfg := satisfiedConfig()
	cfg.OIDCRequestToken = ""
	cfg.Audience = ""
	err := checkPrerequisites(cfg)
	if err == nil || !strings.Contains(err.Error(), "ACTIONS_ID_TOKEN_REQUEST_TOKEN is not set") || !strings.Contains(err.Error(), "input audience is empty") {
		t.Errorf("checkPrerequisites() error = %v, want both unmet prerequisites", err)
	}
}
