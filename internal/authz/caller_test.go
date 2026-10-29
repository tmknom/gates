package authz_test

import (
	"errors"
	"testing"

	"github.com/tmknom/gates/internal/authz"
)

func TestNewCaller(t *testing.T) {
	execution := authz.ExecutionContext{
		Actor:          "octocat",
		ActorID:        "583231",
		EventName:      "push",
		Ref:            "refs/heads/main",
		SHA:            "ffac537e6cbbf934b08745a378932722df287a53",
		WorkflowRef:    "example-org/app/.github/workflows/ci.yml@refs/heads/main",
		JobWorkflowRef: "example-org/app/.github/workflows/ci.yml@refs/heads/main",
		Environment:    "production",
	}
	got, err := authz.NewCaller(11, 22, 33, 2, execution)
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	if got.OwnerID() != 11 {
		t.Errorf("OwnerID() = %d, want 11", got.OwnerID())
	}
	if got.RepositoryID() != 22 {
		t.Errorf("RepositoryID() = %d, want 22", got.RepositoryID())
	}
	if got.RunID() != 33 {
		t.Errorf("RunID() = %d, want 33", got.RunID())
	}
	if got.RunAttempt() != 2 {
		t.Errorf("RunAttempt() = %d, want 2", got.RunAttempt())
	}
	if got.ExecutionContext() != execution {
		t.Errorf("ExecutionContext() = %+v, want %+v", got.ExecutionContext(), execution)
	}
}

func TestNewCaller_AllowsMissingExecutionContext(t *testing.T) {
	if _, err := authz.NewCaller(11, 22, 33, 1, authz.ExecutionContext{}); err != nil {
		t.Errorf("NewCaller returned error: %v", err)
	}
}

func TestNewCaller_Invalid(t *testing.T) {
	tests := []struct {
		name         string
		ownerID      int64
		repositoryID int64
		runID        int64
		runAttempt   int64
	}{
		{name: "zero owner id returns ErrInvalidCaller", ownerID: 0, repositoryID: 22, runID: 33, runAttempt: 1},
		{name: "negative owner id returns ErrInvalidCaller", ownerID: -1, repositoryID: 22, runID: 33, runAttempt: 1},
		{name: "zero repository id returns ErrInvalidCaller", ownerID: 11, repositoryID: 0, runID: 33, runAttempt: 1},
		{name: "negative repository id returns ErrInvalidCaller", ownerID: 11, repositoryID: -1, runID: 33, runAttempt: 1},
		{name: "zero run id returns ErrInvalidCaller", ownerID: 11, repositoryID: 22, runID: 0, runAttempt: 1},
		{name: "negative run id returns ErrInvalidCaller", ownerID: 11, repositoryID: 22, runID: -1, runAttempt: 1},
		{name: "zero run attempt returns ErrInvalidCaller", ownerID: 11, repositoryID: 22, runID: 33, runAttempt: 0},
		{name: "negative run attempt returns ErrInvalidCaller", ownerID: 11, repositoryID: 22, runID: 33, runAttempt: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := authz.NewCaller(tt.ownerID, tt.repositoryID, tt.runID, tt.runAttempt, authz.ExecutionContext{})
			if !errors.Is(err, authz.ErrInvalidCaller) {
				t.Errorf("NewCaller() error = %v, want %v", err, authz.ErrInvalidCaller)
			}
		})
	}
}
