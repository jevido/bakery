package http

import (
	"testing"

	"github.com/jevido/bakery/services/api/contexts/identity/domain"
)

func TestPrincipalAllows(t *testing.T) {
	var session Principal
	if !session.Allows(domain.PermissionReadSensitive) {
		t.Error("a Session is not limited by Permissions")
	}
	deploy := Principal{Token: &domain.APIToken{Permissions: []domain.Permission{domain.PermissionDeploy}}}
	if deploy.Allows(domain.PermissionRead) || deploy.Allows(domain.PermissionWrite) || !deploy.Allows(domain.PermissionDeploy) {
		t.Error("a deploy token only deploys")
	}
}
