package caller_test

import (
	"errors"
	"testing"

	"github.com/tmknom/gates/internal/caller"
)

func TestNewCaller(t *testing.T) {
	attributes := caller.WorkflowTriggerAttributes{
		Actor:          "octocat",
		ActorID:        "583231",
		EventName:      "push",
		Ref:            "refs/heads/main",
		SHA:            "ffac537e6cbbf934b08745a378932722df287a53",
		WorkflowRef:    "example-org/app/.github/workflows/ci.yml@refs/heads/main",
		JobWorkflowRef: "example-org/app/.github/workflows/ci.yml@refs/heads/main",
		Environment:    "production",
	}
	got, err := caller.NewCaller(11, 22, 33, 2, attributes)
	if err != nil {
		t.Fatalf("NewCaller returned error: %v", err)
	}
	if got.OwnerID().Int64() != 11 {
		t.Errorf("OwnerID() = %d, want 11", got.OwnerID().Int64())
	}
	if got.RepositoryID().Int64() != 22 {
		t.Errorf("RepositoryID() = %d, want 22", got.RepositoryID().Int64())
	}
	if got.RunID().Int64() != 33 {
		t.Errorf("RunID() = %d, want 33", got.RunID().Int64())
	}
	if got.RunAttempt().Int64() != 2 {
		t.Errorf("RunAttempt() = %d, want 2", got.RunAttempt().Int64())
	}
	if got.WorkflowTriggerAttributes() != attributes {
		t.Errorf("WorkflowTriggerAttributes() = %+v, want %+v", got.WorkflowTriggerAttributes(), attributes)
	}
}

func TestNewCaller_AllowsMissingWorkflowTriggerAttributes(t *testing.T) {
	if _, err := caller.NewCaller(11, 22, 33, 1, caller.WorkflowTriggerAttributes{}); err != nil {
		t.Errorf("NewCaller returned error: %v", err)
	}
}

// TestNewCaller_Invalid は、各 ID と試行番号の形式の判定を githubid に任せていることを 0 で確かめ、負の値の拒否は githubid のテストに任せる。
func TestNewCaller_Invalid(t *testing.T) {
	tests := []struct {
		name         string
		ownerID      int64
		repositoryID int64
		runID        int64
		runAttempt   int64
	}{
		{name: "zero owner id returns ErrInvalidCaller", ownerID: 0, repositoryID: 22, runID: 33, runAttempt: 1},
		{name: "zero repository id returns ErrInvalidCaller", ownerID: 11, repositoryID: 0, runID: 33, runAttempt: 1},
		{name: "zero run id returns ErrInvalidCaller", ownerID: 11, repositoryID: 22, runID: 0, runAttempt: 1},
		{name: "zero run attempt returns ErrInvalidCaller", ownerID: 11, repositoryID: 22, runID: 33, runAttempt: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := caller.NewCaller(tt.ownerID, tt.repositoryID, tt.runID, tt.runAttempt, caller.WorkflowTriggerAttributes{})
			if !errors.Is(err, caller.ErrInvalidCaller) {
				t.Errorf("NewCaller() error = %v, want %v", err, caller.ErrInvalidCaller)
			}
		})
	}
}
