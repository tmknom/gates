package github_test

import "testing"

func TestIssuanceResult_EffectiveRepositoriesCannotBeModifiedByCaller(t *testing.T) {
	result := issueBody(t, marshal(t, recordedIssuance()))

	got, _ := result.EffectiveRepositories()
	got[0] = got[1]

	again, _ := result.EffectiveRepositories()
	if again[0].RepositoryID() != 1234567890 {
		t.Errorf("EffectiveRepositories()[0].RepositoryID() = %d after modifying the copy, want 1234567890", again[0].RepositoryID())
	}
}
