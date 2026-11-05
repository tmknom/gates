package authz

import (
	"fmt"
	"time"
)

// Installation は、GitHub App の Installation のうち、発行許可の確定に要る属性である。
// フィールドを非公開にし、ゼロ値以外は NewInstallation でしか作れないようにする。
type Installation struct {
	id                  int64
	installationOwnerID int64
	suspendedAt         time.Time
}

// NewInstallation は、Domain Model の「Invariants」が定める Installation の条件を検証する。
// 応答から欠落した ID を 0 として受け取っても Installation にしないため。
// suspendedAt はゼロ値で停止していないことを表す。GitHub の停止時刻がゼロ値の時刻になることはないため。
func NewInstallation(id, installationOwnerID int64, suspendedAt time.Time) (Installation, error) {
	if id <= 0 {
		return Installation{}, fmt.Errorf("%w: installation id must be a positive integer", ErrInvalidInstallation)
	}
	if installationOwnerID <= 0 {
		return Installation{}, fmt.Errorf("%w: installation owner id must be a positive integer", ErrInvalidInstallation)
	}
	return Installation{id: id, installationOwnerID: installationOwnerID, suspendedAt: suspendedAt}, nil
}

// ID は、Installation ID を返す。
func (i Installation) ID() int64 {
	return i.id
}

// InstallationOwnerID は、インストール先 Owner ID を返す。
func (i Installation) InstallationOwnerID() int64 {
	return i.installationOwnerID
}
