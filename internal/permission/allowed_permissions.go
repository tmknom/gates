package permission

// allowedPermissions は、GATES が発行を許可する Permission Entry の集合（許可権限）である。
// 定め方は Security Design の「Permission Boundary」に、列挙する権限名と水準は GitHub App Constraints の「権限名と分類」の表に従う。
// 設定から広げられないよう、公開せず、作る関数も書き換える関数も持たない。
var allowedPermissions = entriesOf([]allowedRow{
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
})

// allowedRow は、GitHub App Constraints の表の一行に当たり、同じ水準を許可する権限名をまとめる。
type allowedRow struct {
	names  []string
	levels []string
}

func entriesOf(rows []allowedRow) map[entry]bool {
	entries := make(map[entry]bool)
	for _, row := range rows {
		for _, name := range row.names {
			for _, level := range row.levels {
				entries[entry{name: name, level: level}] = true
			}
		}
	}
	return entries
}
