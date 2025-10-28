//go:build integration || transit
// +build integration transit

package transit

import (
	"os"
	"testing"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/kms/util"
)

func init() {
	viper.AutomaticEnv()
}

func TestKMS(t *testing.T) {
	if err := os.Setenv("VAULT_ADDR", "http://localhost:8200"); err != nil {
		t.Fatalf("preparing env: %v", err)
	}

	if err := os.Setenv("VAULT_TOKEN", "dev-only-token"); err != nil {
		t.Fatalf("preparing env: %v", err)
	}

	k, err := NewVaultTransit("transit", "terraform-backend")
	if err != nil {
		t.Error(err)
	}

	util.KMSTest(t, k)
}
