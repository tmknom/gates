package repositorypolicy

import (
	"errors"
	"testing"
)

//trace:checks AC-AUTHZ-014
func TestFetchResult_Authorize(t *testing.T) {
	withMetadata := replaceOnce(t, validDocument, "      contents: read\n", "      contents: read\n      metadata: read\n")
	tests := []struct {
		name        string
		document    string
		callerID    int64
		callerName  string
		levels      map[string]string
		wantErr     error
		wantMessage string
	}{
		{name: "same level as the permission limit is allowed", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"contents": "read", "pull_requests": "write"}},
		{name: "lower level than the permission limit is allowed", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"pull_requests": "read"}},
		{name: "allowed caller name differing only in case is allowed", callerID: 100, callerName: "Deploy-Tools", levels: map[string]string{"contents": "read"}},
		{name: "explicit implied permission within the permission limit is allowed", document: withMetadata, callerID: 100, callerName: "deploy-tools", levels: map[string]string{"contents": "read", "metadata": "read"}},
		{
			name: "caller matching neither is denied", callerID: 999, callerName: "other", levels: map[string]string{"contents": "read"},
			wantErr: errNoMatchingAllowedCaller, wantMessage: "repository policy has no allowed caller matching both the caller repository id and name",
		},
		{
			name: "id only match is denied", callerID: 100, callerName: "renamed-tools", levels: map[string]string{"contents": "read"},
			wantErr: errAllowedCallerNameMismatch, wantMessage: "repository policy has an allowed caller matching the caller repository id but not the name",
		},
		{
			name: "name only match is denied", callerID: 999, callerName: "deploy-tools", levels: map[string]string{"contents": "read"},
			wantErr: errAllowedCallerIDMismatch, wantMessage: "repository policy has an allowed caller matching the caller repository name but not the id",
		},
		{
			name: "higher level than the permission limit is denied", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"contents": "write"},
			wantErr: errExceedsPermissionLimit, wantMessage: "permissions are not within the permission limit of the repository policy",
		},
		{
			name: "permission name outside the permission limit is denied", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"issues": "read"},
			wantErr: errExceedsPermissionLimit, wantMessage: "permissions are not within the permission limit of the repository policy",
		},
		{
			name: "level outside allowed permissions is denied", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"contents": "admin"},
			wantErr: errExceedsPermissionLimit, wantMessage: "permissions are not within the permission limit of the repository policy",
		},
		{
			name: "explicit implied permission outside the permission limit is denied", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"contents": "read", "metadata": "read"},
			wantErr: errExceedsPermissionLimit, wantMessage: "permissions are not within the permission limit of the repository policy",
		},
		{
			name: "repository policy without allowed callers is denied", document: "callers: []\n", callerID: 100, callerName: "deploy-tools", levels: map[string]string{"contents": "read"},
			wantErr: errNoAllowedCallers, wantMessage: "repository policy lists no allowed callers",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := tt.document
			if document == "" {
				document = validDocument
			}
			err := Document(document).Authorize(mustRepositoryID(t, tt.callerID), tt.callerName, mustNewPermissions(t, tt.levels))
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Authorize returned error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) || err.Error() != tt.wantMessage {
				t.Errorf("Authorize error = %v, want %q", err, tt.wantMessage)
			}
		})
	}
}

//trace:checks AC-AUTHZ-015
func TestFetchResult_Authorize_WithoutValidRepositoryPolicy(t *testing.T) {
	tests := []struct {
		name        string
		result      FetchResult
		wantErr     error
		wantMessage string
	}{
		{name: "missing file is denied", result: NotFound(), wantErr: errNotFound, wantMessage: "repository policy is not found"},
		{name: "path that is not a file is denied", result: NotFile(), wantErr: errNotFile, wantMessage: "repository policy is not a file"},
		{name: "file over the size limit is denied", result: TooLarge(), wantErr: errTooLarge, wantMessage: "repository policy is too large"},
		{name: "invalid document is denied", result: Document(replaceOnce(t, validDocument, "permissions:\n      contents: read\n      pull_requests: write\n", "permissions: {}\n")), wantErr: errInvalid, wantMessage: "repository policy is invalid"},
		{name: "zero value is denied", result: FetchResult{}, wantErr: errUninitialized, wantMessage: "repository policy fetch result is not initialized"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.Authorize(mustRepositoryID(t, 100), "deploy-tools", mustNewPermissions(t, map[string]string{"contents": "read"}))
			if !errors.Is(err, tt.wantErr) || err.Error() != tt.wantMessage {
				t.Errorf("Authorize error = %v, want %q", err, tt.wantMessage)
			}
		})
	}
}
