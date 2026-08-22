package keystone

import (
	"testing"

	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/tokens"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/auth/keystone/keystonetest"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func TestAuth(t *testing.T) {
	k := keystonetest.NewIfIntegrationTest(t)

	authOpts := tokens.AuthOptions{
		Username:   "admin",
		Password:   "admin",
		DomainName: "Default",
		Scope: tokens.Scope{
			ProjectName: "admin",
			DomainName:  "Default",
		},
	}

	result := tokens.Create(t.Context(), k, &authOpts)
	token, err := result.ExtractToken()
	require.NoError(t, err)

	project, err := result.ExtractProject()
	require.NoError(t, err)

	a := NewKeystoneAuth("http://localhost:5000/v3")

	t.Run("success", func(t *testing.T) {
		state := &terraform.State{
			ID:      terraform.GetStateID(project.ID, "prod"),
			Project: project.ID,
			Name:    "prod",
		}

		ok, err := a.Authenticate(token.ID, state)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("invalid project", func(t *testing.T) {
		state := &terraform.State{
			ID:      terraform.GetStateID("sample", "prod"),
			Project: "sample",
			Name:    "prod",
		}

		ok, err := a.Authenticate(token.ID, state)
		require.NoError(t, err)
		require.False(t, ok)
	})
}
