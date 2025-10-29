package internal_test

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/internal"
)

func TestSecretEnvOrFile(t *testing.T) {
	viper.AutomaticEnv()

	t.Setenv("ENV_SECRET", "V3ry5ecr3t")

	connStr, err := internal.SecretEnvOrFile("env_secret", "env_secret_file")
	require.NoError(t, err)
	require.Equal(t, "V3ry5ecr3t", connStr)

	tmpFile, err := os.CreateTemp(os.TempDir(), "terraform-backend-test-secret-env-file")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("V3ry5ecr3tFr0mF1l3")
	require.NoError(t, err)

	t.Setenv("ENV_SECRET_FILE", tmpFile.Name())

	connStr, err = internal.SecretEnvOrFile("env_secret", "env_secret_file")
	require.NoError(t, err)
	require.Equal(t, "V3ry5ecr3tFr0mF1l3", connStr)
}
