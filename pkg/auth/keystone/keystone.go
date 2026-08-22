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
}

func NewKeystoneAuth(identityEndpoint string) *KeystoneAuth {
	return &KeystoneAuth{
		identityEndpoint: identityEndpoint,
	}
}

func (k *KeystoneAuth) GetName() string {
	return Name
}

func (k *KeystoneAuth) Authenticate(secret string, s *terraform.State) (bool, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: "http://localhost:5000/v3",
		TokenID:          secret,
	}

	provider, err := openstack.AuthenticatedClient(context.Background(), authOpts)
	if err != nil {
		return false, err
	}

	identityClient, err := openstack.NewIdentityV3(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return false, err
	}

	result := tokens.Get(context.Background(), identityClient, secret)
	if result.Err != nil {
		return false, result.Err
	}

	project, err := result.ExtractProject()
	if err != nil {
		return false, err
	}

	if s.Project == project.ID {
		return true, nil
	}

	return false, nil
}
