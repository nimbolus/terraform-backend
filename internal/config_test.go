package internal_test

import (
	"os"
	"testing"

	"github.com/nimbolus/terraform-backend/internal"
	"github.com/spf13/viper"
)

func TestSecretEnvOrFile(t *testing.T) {
	t.Parallel()

	viper.AutomaticEnv()

	if err := os.Setenv("ENV_SECRET", "V3ry5ecr3t"); err != nil {
		t.Fatal(err)
	}

	connStr, err := internal.SecretEnvOrFile("env_secret", "env_secret_file")
	if err != nil {
		t.Fatal(err)
	}

	if connStr != "V3ry5ecr3t" {
		t.Fatalf("expected %q, got %q", "V3ry5ecr3t", connStr)
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "terraform-backend-test-secret-env-file")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("V3ry5ecr3tFr0mF1l3"); err != nil {
		t.Fatal(err)
	}

	if err := os.Setenv("ENV_SECRET_FILE", tmpFile.Name()); err != nil {
		t.Fatal(err)
	}

	connStr, err = internal.SecretEnvOrFile("env_secret", "env_secret_file")
	if err != nil {
		t.Fatal(err)
	}

	if connStr != "V3ry5ecr3tFr0mF1l3" {
		t.Fatalf("expected %q, got %q", "V3ry5ecr3tFr0mF1l3", connStr)
	}
}
