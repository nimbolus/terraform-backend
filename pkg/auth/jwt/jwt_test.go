package jwt

import (
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/client/vault/vaulttest"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func TestAuth(t *testing.T) {
	v := vaulttest.NewIfIntegrationTest(t)

	entityToken, err := v.Auth().Token().CreateWithRole(&api.TokenCreateRequest{EntityAlias: "sample"}, "sample")
	require.NoError(t, err)

	et, err := entityToken.TokenID()
	require.NoError(t, err)

	v.SetToken(et)

	token, err := v.Logical().Read("identity/oidc/token/terraform-backend-sample")
	require.NoError(t, err)

	a := NewJWTAuth("http://localhost:8200/v1/identity/oidc")

	t.Run("success", func(t *testing.T) {
		state := &terraform.State{
			ID:      terraform.GetStateID("other-project", "prod"),
			Project: "other-project",
			Name:    "prod",
		}

		ok, err := a.Authenticate(token.Data["token"].(string), state)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("invalid name", func(t *testing.T) {
		state := &terraform.State{
			ID:      terraform.GetStateID("sample", "other-name"),
			Project: "sample",
			Name:    "other-name",
		}

		ok, err := a.Authenticate(token.Data["token"].(string), state)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("invalid project", func(t *testing.T) {
		state := &terraform.State{
			ID:      terraform.GetStateID("sample", "prod"),
			Project: "sample",
			Name:    "prod",
		}

		ok, err := a.Authenticate(token.Data["token"].(string), state)
		require.NoError(t, err)
		require.True(t, ok)
	})
}
