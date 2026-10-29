package authz

import (
	"fmt"
	"time"
)

// Installation は、GitHub App の Installation のうち、発行許可の確定に要る属性である。
// フィールドを非公開にし、ゼロ値以外は NewInstallation でしか作れないようにする。
type Installation struct {
	id          int64
	accountID   int64
	suspendedAt time.Time
}

// NewInstallation は、Domain Model の「Invariants」が定める Installation の条件を検証する。
// 応答から欠落した ID を 0 として受け取っても Installation にしないため。
// suspendedAt はゼロ値で空を表す。GitHub の停止時刻がゼロ値の時刻になることはないため。
func NewInstallation(id, accountID int64, suspendedAt time.Time) (Installation, error) {
	if id <= 0 {
		return Installation{}, fmt.Errorf("%w: installation id must be a positive integer", ErrInvalidInstallation)
	}
	if accountID <= 0 {
		return Installation{}, fmt.Errorf("%w: account id must be a positive integer", ErrInvalidInstallation)
	}
	return Installation{id: id, accountID: accountID, suspendedAt: suspendedAt}, nil
}
