package protected

import (
	"errors"
	"testing"

	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/repository"
)

//trace:checks AC-AUTHZ-012
func TestRepositories_Verify(t *testing.T) {
	tests := []struct {
		name        string
		document    string
		id          int64
		fullName    string
		wantErr     error
		wantMessage string
	}{
		{name: "list without protected repositories accepts any repository", document: "repositories: []\n", id: 200, fullName: "example-org/platform-infra"},
		{name: "repository matching neither the name nor the id is accepted", id: 999, fullName: "example-org/app"},
		{name: "repository matching both the name and the id is accepted", id: 200, fullName: "example-org/platform-infra"},
		{name: "name differing only in case with the same id is accepted", id: 200, fullName: "example-org/Platform-Infra"},
		{
			name: "repository matching only the name is rejected", id: 999, fullName: "example-org/platform-infra",
			wantErr: errProtectedIDMismatch, wantMessage: "protected repositories[0] matches the repository name but not the repository id",
		},
		{
			name: "name differing only in case with another id is rejected", id: 999, fullName: "example-org/PLATFORM-INFRA",
			wantErr: errProtectedIDMismatch, wantMessage: "protected repositories[0] matches the repository name but not the repository id",
		},
		{
			name: "repository matching only the id is rejected", id: 201, fullName: "example-org/renamed-records",
			wantErr: errProtectedNameMismatch, wantMessage: "protected repositories[1] matches the repository id but not the repository name",
		},
		{
			name: "name of one protected repository and id of another are rejected", id: 201, fullName: "example-org/platform-infra",
			wantErr: errProtectedIDMismatch, wantMessage: "protected repositories[0] matches the repository name but not the repository id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := tt.document
			if document == "" {
				document = validDocument
			}
			err := mustParse(t, document).Verify(mustRepositoryID(t, tt.id), mustParseFullName(t, tt.fullName))
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("Verify returned error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) || err.Error() != tt.wantMessage {
				t.Errorf("Verify error = %v, want %q wrapping %v", err, tt.wantMessage, tt.wantErr)
			}
		})
	}
}

func TestRepositories_VerifyIncompleteEffectiveRepository(t *testing.T) {
	r := mustParse(t, validDocument)
	t.Run("zero repository id is rejected", func(t *testing.T) {
		if err := r.Verify(githubid.RepositoryID{}, mustParseFullName(t, "example-org/app")); !errors.Is(err, errIncompleteEffectiveRepository) {
			t.Errorf("Verify error = %v, want %v", err, errIncompleteEffectiveRepository)
		}
	})
	t.Run("zero full name is rejected", func(t *testing.T) {
		if err := r.Verify(mustRepositoryID(t, 999), repository.FullName{}); !errors.Is(err, errIncompleteEffectiveRepository) {
			t.Errorf("Verify error = %v, want %v", err, errIncompleteEffectiveRepository)
		}
	})
}

func TestRepositories_VerifyZeroValue(t *testing.T) {
	err := Repositories{}.Verify(mustRepositoryID(t, 1), mustParseFullName(t, "example-org/app"))
	if !errors.Is(err, errUninitialized) {
		t.Errorf("Verify error = %v, want %v", err, errUninitialized)
	}
}
