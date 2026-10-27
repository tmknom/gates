package cmd

import "testing"

func TestEnv_version(t *testing.T) {
	tests := []struct {
		name string
		env  Env
		want string
	}{
		{name: "version and commit", env: Env{Version: "v1.2.3", Commit: "abc123"}, want: "v1.2.3 (abc123)"},
		{name: "version only", env: Env{Version: "v1.2.3"}, want: "v1.2.3"},
		{name: "empty version", env: Env{Commit: "abc123"}, want: "dev (abc123)"},
		{name: "zero value", env: Env{}, want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.env.version(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
