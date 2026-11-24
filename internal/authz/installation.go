package authz

import (
	"fmt"

	"github.com/tmknom/gates/internal/githubid"
)

// Installation は、GitHub App の Installation のうち、発行許可の確定に要る属性である。
// フィールドを非公開にし、ゼロ値以外は NewInstallation でしか作れないようにする。
type Installation struct {
	id                  githubid.InstallationID
	installationOwnerID githubid.OwnerID
	suspended           bool
}

// NewInstallation は、INV-010 が定める Installation の条件を検証する。
// 応答から欠落した ID を 0 として受け取っても Installation にしないため。
func NewInstallation(id, installationOwnerID int64, suspended bool) (Installation, error) {
	installationID, err := githubid.NewInstallationID(id)
	if err != nil {
		return Installation{}, fmt.Errorf("%w: installation id %w", ErrInvalidInstallation, err)
	}
	ownerID, err := githubid.NewOwnerID(installationOwnerID)
	if err != nil {
		return Installation{}, fmt.Errorf("%w: installation owner id %w", ErrInvalidInstallation, err)
	}
	return Installation{id: installationID, installationOwnerID: ownerID, suspended: suspended}, nil
}

// ID は、Installation ID を返す。
func (i Installation) ID() githubid.InstallationID {
	return i.id
}

// InstallationOwnerID は、インストール先 Owner ID を返す。
func (i Installation) InstallationOwnerID() githubid.OwnerID {
	return i.installationOwnerID
}

// verifyUsableBy は、Installation が許可オーナーの発行に使える（RULE-011）かを確かめる。
func (i Installation) verifyUsableBy(allowedOwner AllowedOwner) error {
	if i.installationOwnerID != allowedOwner.ID() {
		return fmt.Errorf("%w: installation owner id does not match allowed owner id", ErrInstallationUnavailable)
	}
	if i.suspended {
		return fmt.Errorf("%w: installation is suspended", ErrInstallationUnavailable)
	}
	return nil
}
