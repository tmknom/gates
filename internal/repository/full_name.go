package repository

import (
	"fmt"
	"strings"
)

const maxRepositoryNameLength = 100

// FullName は、`owner/repo` 形式のリポジトリの完全名である。
// 監査記録に要求どおりの表記を残すため、正規化せずに保持する。
// OwnerName と同じく == と map のキーに使えない。
type FullName struct {
	ownerName      OwnerName
	repositoryName string
}

// ParseFullName は、一度だけ `/` で分割し、Lambda Design の名前の文字種に従って両部分を検証する。
// `/` の数は数えず、二つ目以降はリポジトリ名の文字種の検証で拒否する。
func ParseFullName(s string) (FullName, error) {
	owner, repo, found := strings.Cut(s, "/")
	if !found {
		return FullName{}, fmt.Errorf("%w: must be in the form owner/repo", ErrInvalidFullName)
	}
	ownerName, err := ParseOwnerName(owner)
	if err != nil {
		return FullName{}, fmt.Errorf("%w: %w", ErrInvalidFullName, err)
	}
	if err := validateRepositoryName(repo); err != nil {
		return FullName{}, err
	}
	return FullName{ownerName: ownerName, repositoryName: repo}, nil
}

func validateRepositoryName(s string) error {
	for i := range len(s) {
		c := s[i]
		if !isASCIIAlphanumeric(c) && c != '-' && c != '_' && c != '.' {
			return fmt.Errorf("%w: repository name must consist of ASCII letters, digits, '-', '_', or '.'", ErrInvalidFullName)
		}
	}
	// 文字種を ASCII に限った後なので、バイト数が文字数に等しい
	if s == "" || len(s) > maxRepositoryNameLength {
		return fmt.Errorf("%w: repository name must be 1 to %d characters", ErrInvalidFullName, maxRepositoryNameLength)
	}
	return nil
}

// OwnerName は、完全名の Owner 部分を返す。
func (f FullName) OwnerName() OwnerName {
	return f.ownerName
}

// RepositoryName は、Owner を除いたリポジトリ名を返す。
func (f FullName) RepositoryName() string {
	return f.repositoryName
}

// String は、ParseFullName に渡された表記をそのまま返す。
func (f FullName) String() string {
	return f.ownerName.String() + "/" + f.repositoryName
}

// Equal は、ASCII の範囲で大文字小文字を区別せずに比べる。
func (f FullName) Equal(other FullName) bool {
	return f.ownerName.Equal(other.ownerName) && equalFoldASCII(f.repositoryName, other.repositoryName)
}
