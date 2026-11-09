package response_test

import (
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/response"
)

func TestSuccess_ItemsConvertsTheRemainingItems(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		fixture string
		want    response.Items
	}{
		"all items, repositories in response order and permissions sorted by name": {
			fixture: "success.json",
			want: response.Items{
				ExpiresAt:    "2030-01-01T00:00:00Z",
				Repositories: "example-org/repo-b,example-org/repo-a",
				Permissions:  "contents:read,metadata:read",
				RequestID:    "abc",
			},
		},
		"one repository and one permission have no separator": {
			fixture: "success_single.json",
			want: response.Items{
				ExpiresAt:    "2030-01-01T00:00:00Z",
				Repositories: "example-org/repo-a",
				Permissions:  "contents:read",
				RequestID:    "abc",
			},
		},
		"expires_at that is not RFC 3339 is accepted as is": {
			fixture: "success_not_rfc3339.json",
			want: response.Items{
				ExpiresAt:    "tomorrow",
				Repositories: "example-org/repo-b,example-org/repo-a",
				Permissions:  "contents:read,metadata:read",
				RequestID:    "abc",
			},
		},
		"percent signs and double colons are not encoded": {
			fixture: "success_percent.json",
			want: response.Items{
				ExpiresAt:    "20%30::01",
				Repositories: "example-org/repo%0A",
				Permissions:  "contents%:read%",
				RequestID:    "ab%c",
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := success(t, tt.fixture).Items()
			if err != nil {
				t.Fatalf("Items() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("Items() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSuccess_ItemsRejectsAnUnusableItem(t *testing.T) {
	t.Parallel()

	const withID = ` (request id "abc")`
	tests := map[string]struct {
		fixture string
		want    string
	}{
		"request_id is missing":           {"success_request_id_missing.json", "the success response has no usable request_id"},
		"request_id is a number":          {"success_request_id_number.json", "the success response has no usable request_id"},
		"request_id is empty":             {"success_request_id_empty.json", "the success response has no usable request_id"},
		"request_id contains a newline":   {"success_request_id_newline.json", "the success response has no usable request_id"},
		"expires_at is missing":           {"success_expires_at_missing.json", "the success response has no usable expires_at" + withID},
		"expires_at is a number":          {"success_expires_at_number.json", "the success response has no usable expires_at" + withID},
		"expires_at is empty":             {"success_expires_at_empty.json", "the success response has no usable expires_at" + withID},
		"expires_at contains a newline":   {"success_expires_at_newline.json", "the success response has no usable expires_at" + withID},
		"repositories is not an array":    {"success_repositories_not_array.json", "the success response has no usable repositories" + withID},
		"repositories is empty":           {"success_repositories_empty_array.json", "the success response has no usable repositories" + withID},
		"repositories has a number":       {"success_repositories_number_element.json", "the success response has no usable repositories" + withID},
		"repositories has an empty item":  {"success_repositories_empty_element.json", "the success response has no usable repositories" + withID},
		"repositories has a newline":      {"success_repositories_newline_element.json", "the success response has no usable repositories" + withID},
		"repositories has a comma":        {"success_repositories_comma_element.json", "the success response has no usable repositories" + withID},
		"permissions is not an object":    {"success_permissions_not_object.json", "the success response has no usable permissions" + withID},
		"permissions is empty":            {"success_permissions_empty_object.json", "the success response has no usable permissions" + withID},
		"permissions level is a number":   {"success_permissions_number_level.json", "the success response has no usable permissions" + withID},
		"permissions level is empty":      {"success_permissions_empty_level.json", "the success response has no usable permissions" + withID},
		"permissions name is empty":       {"success_permissions_empty_name.json", "the success response has no usable permissions" + withID},
		"permissions name has a newline":  {"success_permissions_newline_name.json", "the success response has no usable permissions" + withID},
		"permissions level has a newline": {"success_permissions_newline_level.json", "the success response has no usable permissions" + withID},
		"permissions name has a comma":    {"success_permissions_comma_name.json", "the success response has no usable permissions" + withID},
		"permissions name has a colon":    {"success_permissions_colon_name.json", "the success response has no usable permissions" + withID},
		"permissions level has a comma":   {"success_permissions_comma_level.json", "the success response has no usable permissions" + withID},
		"permissions level has a colon":   {"success_permissions_colon_level.json", "the success response has no usable permissions" + withID},
		// Runner は ##[ で始まる旧形式のワークフローコマンドを行の途中からも読むため、ログに出す値に許さない
		"request_id has a legacy workflow command":        {"success_request_id_command.json", "the success response has no usable request_id"},
		"expires_at has a legacy workflow command":        {"success_expires_at_command.json", "the success response has no usable expires_at" + withID},
		"repositories has a legacy workflow command":      {"success_repositories_command_element.json", "the success response has no usable repositories" + withID},
		"permissions name has a legacy workflow command":  {"success_permissions_command_name.json", "the success response has no usable permissions" + withID},
		"permissions level has a legacy workflow command": {"success_permissions_command_level.json", "the success response has no usable permissions" + withID},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := success(t, tt.fixture).Items()
			if err == nil {
				t.Fatalf("Items() error = nil, want %q", tt.want)
			}
			if err.Error() != tt.want {
				t.Errorf("Items() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}
