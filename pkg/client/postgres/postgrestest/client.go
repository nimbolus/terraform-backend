package postgrestest

import (
	"database/sql"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/client/postgres"
)

func NewIfIntegrationTest(t testing.TB) *sql.DB {
	t.Helper()

	if v := os.Getenv("INTEGRATION_TEST"); v == "" {
		t.Skip("env var INTEGRATION_TEST not set")
	}

	t.Setenv("POSTGRES_CONNECTION", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")

	viper.AutomaticEnv()

	c, err := postgres.NewClient()
	require.NoError(t, err)

	return c
}
