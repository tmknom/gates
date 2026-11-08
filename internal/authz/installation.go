package authz

import (
	"fmt"
	"time"

	"github.com/tmknom/gates/internal/githubid"
)

// Installation は、GitHub App の Installation のうち、発行許可の確定に要る属性である。
// フィールドを非公開にし、ゼロ値以外は NewInstallation でしか作れないようにする。
type Installation struct {
	id                  githubid.InstallationID
	installationOwnerID githubid.OwnerID
	suspendedAt         time.Time
}

// NewInstallation は、Domain Model の「Invariants」が定める Installation の条件を検証する。
// 応答から欠落した ID を 0 として受け取っても Installation にしないため。
// suspendedAt はゼロ値で停止していないことを表す。GitHub の停止時刻がゼロ値の時刻になることはないため。
func NewInstallation(id, installationOwnerID int64, suspendedAt time.Time) (Installation, error) {
	installationID, err := githubid.NewInstallationID(id)
	if err != nil {
		return Installation{}, fmt.Errorf("%w: installation id %w", ErrInvalidInstallation, err)
	}
	ownerID, err := githubid.NewOwnerID(installationOwnerID)
	if err != nil {
		return Installation{}, fmt.Errorf("%w: installation owner id %w", ErrInvalidInstallation, err)
	}
	return Installation{id: installationID, installationOwnerID: ownerID, suspendedAt: suspendedAt}, nil
}

// ID は、Installation ID を返す。
func (i Installation) ID() githubid.InstallationID {
	return i.id
}

// InstallationOwnerID は、インストール先 Owner ID を返す。
func (i Installation) InstallationOwnerID() githubid.OwnerID {
	return i.installationOwnerID
}
