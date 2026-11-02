package keystone

import (
	"context"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/tokens"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "keystone"

type KeystoneAuth struct {
	ctx              context.Context
	identityEndpoint string
	role             string
}

func NewKeystoneAuth(identityEndpoint string, role string) *KeystoneAuth {
	return &KeystoneAuth{
		ctx:              context.Background(),
		identityEndpoint: identityEndpoint,
		role:             role,
	}
}

func (k *KeystoneAuth) GetName() string {
	return Name
}

func (k *KeystoneAuth) Authenticate(secret string, s *terraform.State) (bool, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: k.identityEndpoint,
		TokenID:          secret,
	}

	provider, err := openstack.AuthenticatedClient(k.ctx, authOpts)
	if err != nil {
		return false, err
	}

	identityClient, err := openstack.NewIdentityV3(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return false, err
	}

	result := tokens.Get(k.ctx, identityClient, secret)
	if result.Err != nil {
		return false, result.Err
	}

	project, err := result.ExtractProject()
	if err != nil {
		return false, err
	}

	if project == nil || project.ID != s.Project {
		return false, nil
	}

	roles, err := result.ExtractRoles()
	if err != nil {
		return false, err
	}

	for _, r := range roles {
		if k.role == r.Name {
			return true, nil
		}
	}

	return false, nil
}
