package tokenissuer

import (
	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/githubid"
	"github.com/tmknom/gates/internal/repository"
)

// verifiedPolicyReadToken は、ポリシー読み取り許可との照合（INV-019）を通った、照合済みのポリシー読み取りトークンである。
// 返却可能な発行結果と別の非公開の型にし、利用者に返す経路に渡せないようにする。ゼロ値以外は verifyPolicyReadToken でしか作らない。
type verifiedPolicyReadToken struct {
	token github.PolicyReadToken
	// targets は、ポリシー読み取り許可の対象の順に並べた、リポジトリポリシーを読む対象である
	targets []policyReadTarget
}

// policyReadTarget は、リポジトリポリシーを読む対象の完全名と、その対象に対応する照合済みの実効対象の Repository ID の組である。
// 取得の応答は読んだリポジトリを Repository ID で示さないため、ID は照合済みのポリシー読み取りトークンの実効対象から得る。
type policyReadTarget struct {
	// fullName は、ポリシー読み取り許可の表記の完全名である
	fullName     repository.FullName
	repositoryID githubid.RepositoryID
}
