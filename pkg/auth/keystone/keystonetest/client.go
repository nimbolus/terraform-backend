package keystonetest

import (
	"os"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/stretchr/testify/require"
)

func NewIfIntegrationTest(t testing.TB) *gophercloud.ServiceClient {
	t.Helper()

	if v := os.Getenv("INTEGRATION_TEST"); v == "" {
		t.Skip("env var INTEGRATION_TEST not set")
	}

	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: "http://localhost:5000/v3",
		Username:         "admin",
		Password:         "admin",
		DomainName:       "Default",
		TenantName:       "admin",
	}

	provider, err := openstack.AuthenticatedClient(t.Context(), authOpts)
	require.NoError(t, err)

	identityClient, err := openstack.NewIdentityV3(provider, gophercloud.EndpointOpts{})
	require.NoError(t, err)

	return identityClient
}
