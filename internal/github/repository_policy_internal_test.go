package github

import (
	"net/http"
	"testing"
)

// 完全名の形式検証を通らない文字を含めて、オーナー名とリポジトリ名がそれぞれパスの 1 区画に収まることを確かめる。
func TestRepositoryPolicyEndpoint_EscapesEachNameAsOneSegment(t *testing.T) {
	tests := []struct {
		name           string
		ownerName      string
		repositoryName string
		want           string
	}{
		{name: "slash in the owner name", ownerName: "a/b", repositoryName: "repo", want: "/repos/a%2Fb/repo/contents/.github/gates-policy.yml"},
		{name: "slash in the repository name", ownerName: "owner", repositoryName: "a/b", want: "/repos/owner/a%2Fb/contents/.github/gates-policy.yml"},
		{name: "question mark", ownerName: "owner", repositoryName: "a?b", want: "/repos/owner/a%3Fb/contents/.github/gates-policy.yml"},
		{name: "number sign", ownerName: "owner", repositoryName: "a#b", want: "/repos/owner/a%23b/contents/.github/gates-policy.yml"},
		{name: "escaped slash", ownerName: "owner", repositoryName: "a%2Fb", want: "/repos/owner/a%252Fb/contents/.github/gates-policy.yml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := NewClient().newRequestTo(t.Context(), http.MethodGet, repositoryPolicyEndpoint(tt.ownerName, tt.repositoryName), NewAppJWT("app.jwt"))

			if got := request.URL.String(); got != "https://api.github.com"+tt.want {
				t.Errorf("request URL = %q, want %q", got, "https://api.github.com"+tt.want)
			}
		})
	}
}
