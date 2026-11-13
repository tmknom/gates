// Package githubid は、GATES が GitHub の実体を同定する不変 ID と、ワークフロー実行の試行番号である。
// どれも RULE-002 の ID の形式に従う。
// 概念ごとに型を分け、Owner ID と Installation ID のように別の概念の ID を取り違えて比べるとコンパイルできないようにする。
package githubid

import "errors"

// ErrInvalidID は、ID が正の整数でないことを表す。文言は、呼び出し元が ID の名前を前置して読める形にする。
var ErrInvalidID = errors.New("must be a positive integer")

// OwnerID は、オーナーの不変 Owner ID である。
// フィールドを非公開にし、ゼロ値以外は NewOwnerID でしか作れないようにする。
type OwnerID struct {
	value int64
}

// NewOwnerID は、ID の形式を満たす値だけから Owner ID を作る。
func NewOwnerID(value int64) (OwnerID, error) {
	if err := validate(value); err != nil {
		return OwnerID{}, err
	}
	return OwnerID{value: value}, nil
}

// IsZero は、NewOwnerID を経ていないゼロ値かを返す。
func (id OwnerID) IsZero() bool {
	return id.value == 0
}

// Int64 は、ID の値を返す。
func (id OwnerID) Int64() int64 {
	return id.value
}

// RepositoryID は、リポジトリの不変 Repository ID である。
// フィールドを非公開にし、ゼロ値以外は NewRepositoryID でしか作れないようにする。
type RepositoryID struct {
	value int64
}

// NewRepositoryID は、ID の形式を満たす値だけから Repository ID を作る。
func NewRepositoryID(value int64) (RepositoryID, error) {
	if err := validate(value); err != nil {
		return RepositoryID{}, err
	}
	return RepositoryID{value: value}, nil
}

// Int64 は、ID の値を返す。
func (id RepositoryID) Int64() int64 {
	return id.value
}

// InstallationID は、GitHub App の Installation の Installation ID である。
// フィールドを非公開にし、ゼロ値以外は NewInstallationID でしか作れないようにする。
type InstallationID struct {
	value int64
}

// NewInstallationID は、ID の形式を満たす値だけから Installation ID を作る。
func NewInstallationID(value int64) (InstallationID, error) {
	if err := validate(value); err != nil {
		return InstallationID{}, err
	}
	return InstallationID{value: value}, nil
}

// IsZero は、NewInstallationID を経ていないゼロ値かを返す。
func (id InstallationID) IsZero() bool {
	return id.value == 0
}

// Int64 は、ID の値を返す。
func (id InstallationID) Int64() int64 {
	return id.value
}

// RunID は、ワークフロー実行のワークフロー実行 ID である。
// フィールドを非公開にし、ゼロ値以外は NewRunID でしか作れないようにする。
type RunID struct {
	value int64
}

// NewRunID は、ID の形式を満たす値だけからワークフロー実行 ID を作る。
func NewRunID(value int64) (RunID, error) {
	if err := validate(value); err != nil {
		return RunID{}, err
	}
	return RunID{value: value}, nil
}

// Int64 は、ID の値を返す。
func (id RunID) Int64() int64 {
	return id.value
}

// RunAttempt は、ワークフロー実行の試行番号である。ID ではないが、ID の形式の規則に従う。
// フィールドを非公開にし、ゼロ値以外は NewRunAttempt でしか作れないようにする。
type RunAttempt struct {
	value int64
}

// NewRunAttempt は、ID の形式を満たす値だけから試行番号を作る。
func NewRunAttempt(value int64) (RunAttempt, error) {
	if err := validate(value); err != nil {
		return RunAttempt{}, err
	}
	return RunAttempt{value: value}, nil
}

// Int64 は、試行番号の値を返す。
func (a RunAttempt) Int64() int64 {
	return a.value
}

func validate(value int64) error {
	if value <= 0 {
		return ErrInvalidID
	}
	return nil
}
