package permission

import (
	"maps"
	"slices"
	"testing"
)

// TestAllowedPermissions_MatchesGitHubAppConstraints は、許可権限を GitHub App Constraints の「権限名と分類」の表と一行ずつ突き合わせる。
// 期待値は表の行の組み分けのまま書き、行を移したときに設計書と見比べられるようにする。
func TestAllowedPermissions_MatchesGitHubAppConstraints(t *testing.T) {
	rows := []struct {
		names  []string
		levels []string
	}{
		{
			names: []string{
				"actions", "administration", "artifact_metadata", "attestations", "checks", "code_quality", "codespaces",
				"contents", "dependabot_secrets", "deployments", "discussions", "environments", "issues", "merge_queues",
				"packages", "pages", "pull_requests", "repository_custom_properties", "repository_hooks",
				"secret_scanning_alerts", "secrets", "security_events", "single_file", "statuses", "vulnerability_alerts",
			},
			levels: []string{"read", "write"},
		},
		{names: []string{"workflows"}, levels: []string{"read", "write"}},
		{names: []string{"repository_projects"}, levels: []string{"read", "write", "admin"}},
		{names: []string{"metadata"}, levels: []string{"read"}},
		{
			names: []string{
				"actions_variables", "agent_secrets", "agent_tasks", "agent_variables", "codespaces_lifecycle_admin",
				"codespaces_secrets", "copilot_agent_settings", "installation_repositories", "license_compliance_alerts",
				"repo_secret_scanning_dismissal_requests", "repository_advisories", "repository_creation",
				"secret_scanning_bypass_requests",
			},
			levels: []string{"read", "write"},
		},
		{names: []string{"codespaces_metadata", "repository_license_policy"}, levels: []string{"read"}},
	}
	want := make(map[entry]bool)
	wantNames := make(map[string]bool)
	for _, row := range rows {
		for _, name := range row.names {
			wantNames[name] = true
			for _, level := range row.levels {
				want[entry{name: name, level: level}] = true
			}
		}
	}
	if len(wantNames) != 43 || len(want) != 84 {
		t.Fatalf("expected table has %d names and %d entries, want 43 and 84", len(wantNames), len(want))
	}

	got := make(map[entry]bool)
	for e := range allowedPermissions {
		got[e] = true
	}
	if !maps.Equal(got, want) {
		for e := range got {
			if !want[e] {
				t.Errorf("allowed permissions has %s:%s, which is not in the table", e.name, e.level)
			}
		}
		for e := range want {
			if !got[e] {
				t.Errorf("allowed permissions lacks %s:%s", e.name, e.level)
			}
		}
	}
}

// TestAllowedPermissions_ExcludesNonRepositoryPermissions は、スキーマ「App Permissions」の権限名のうち、
// Installation に現れなかった 29（Organization permissions、Account permissions、Enterprise permissions、`interaction_limits`）が、どの水準でも許可権限にないことを確かめる。
// 出典は REST API Description のスキーマ「App Permissions」で、許可権限の表とは別の資料から期待値を取る。
func TestAllowedPermissions_ExcludesNonRepositoryPermissions(t *testing.T) {
	excluded := []string{
		"custom_properties_for_organizations", "members", "organization_administration", "organization_custom_roles",
		"organization_custom_org_roles", "organization_custom_properties", "organization_copilot_seat_management",
		"organization_copilot_agent_settings", "organization_copilot_requests", "organization_external_properties_for_repos",
		"organization_announcement_banners", "organization_events", "organization_hooks", "organization_personal_access_tokens",
		"organization_personal_access_token_requests", "organization_plan", "organization_projects", "organization_packages",
		"organization_secrets", "organization_self_hosted_runners", "organization_user_blocking",
		"email_addresses", "followers", "git_ssh_keys", "gpg_keys", "interaction_limits", "profile", "starring",
		"enterprise_custom_properties_for_organizations",
	}
	if len(excluded) != 29 {
		t.Fatalf("excluded names = %d, want 29", len(excluded))
	}
	for _, name := range excluded {
		for _, level := range []string{"read", "write", "admin"} {
			if allowedPermissions[entry{name: name, level: level}] {
				t.Errorf("allowed permissions has %s:%s", name, level)
			}
		}
	}
}

// TestAllowedPermissions_TermsAreWellFormed は、許可権限のどの組も権限名と水準の文字種を満たし、形式検証を通る要求で表せることを確かめる。
func TestAllowedPermissions_TermsAreWellFormed(t *testing.T) {
	for _, e := range slices.Collect(maps.Keys(allowedPermissions)) {
		if !wellFormedTerm(e.name) || !wellFormedTerm(e.level) {
			t.Errorf("%q:%q does not satisfy the term format", e.name, e.level)
		}
	}
}
