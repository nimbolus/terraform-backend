package vaulttest

import (
	"os"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/client/vault"
)

func NewIfIntegrationTest(t testing.TB) *api.Client {
	t.Helper()

	if v := os.Getenv("INTEGRATION_TEST"); v == "" {
		t.Skip("env var INTEGRATION_TEST not set")
	}

	t.Setenv("VAULT_ADDR", "http://localhost:8200")
	t.Setenv("VAULT_TOKEN", "dev-only-token")

	viper.AutomaticEnv()

	c, err := vault.NewVaultClient()
	require.NoError(t, err)

	return c
}
