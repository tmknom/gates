// Package githubid は、GATES が GitHub の実体を同定する不変 ID と、ワークフロー実行の試行番号である。
// どれも RULE-002 の ID の形式に従う。
// 概念ごとに型を分け、Owner ID と Installation ID のように別の概念の ID を取り違えて比べるとコンパイルできないようにする。
package githubid

import (
	"errors"
	"strconv"
)

// ErrInvalidID は、ID が正の整数でないことを表す。文言は、呼び出し元が ID の名前を前置して読める形にする。
var ErrInvalidID = errors.New("must be a positive integer")

// errMalformedDecimal は、ID の表記が符号と先頭の 0 を持たない 10 進表記でないことを表す。文言に入力の値を含めない。
var errMalformedDecimal = errors.New("must be a positive decimal integer without a sign or leading zeros that fits in int64")

// OwnerID は、オーナーの不変 Owner ID である。
// フィールドを非公開にし、ゼロ値以外は NewOwnerID か ParseOwnerID でしか作れないようにする。
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

// ParseOwnerID は、符号と先頭の 0 を持たない 10 進表記だけから Owner ID を作る。
func ParseOwnerID(s string) (OwnerID, error) {
	value, err := parseDecimal(s)
	if err != nil {
		return OwnerID{}, err
	}
	return NewOwnerID(value)
}

// IsZero は、NewOwnerID か ParseOwnerID を経ていないゼロ値かを返す。
func (id OwnerID) IsZero() bool {
	return id.value == 0
}

// Int64 は、ID の値を返す。
func (id OwnerID) Int64() int64 {
	return id.value
}

// RepositoryID は、リポジトリの不変 Repository ID である。
// フィールドを非公開にし、ゼロ値以外は NewRepositoryID か ParseRepositoryID でしか作れないようにする。
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

// ParseRepositoryID は、符号と先頭の 0 を持たない 10 進表記だけから Repository ID を作る。
func ParseRepositoryID(s string) (RepositoryID, error) {
	value, err := parseDecimal(s)
	if err != nil {
		return RepositoryID{}, err
	}
	return NewRepositoryID(value)
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

// parseDecimal は、符号と先頭の 0 を持たない 10 進表記だけを受け付ける。先頭の 0 を拒否するため、0 も受け付けない。
// strconv.ParseUint を 63 ビットで使うのは、strconv.ParseInt と違って符号を受理せず、値が int64 に収まるため。
// strconv の error は値を含むため、包まずに条件だけを返す。
func parseDecimal(s string) (int64, error) {
	value, err := strconv.ParseUint(s, 10, 63)
	if err != nil || s[0] == '0' {
		return 0, errMalformedDecimal
	}
	return int64(value), nil
}
