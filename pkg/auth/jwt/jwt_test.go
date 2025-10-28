//go:build integration || jwt
// +build integration jwt

package jwt

import (
	"os"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/client/vault"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func init() {
	viper.AutomaticEnv()
}

func TestAuth(t *testing.T) {
	if err := os.Setenv("VAULT_ADDR", "http://localhost:8200"); err != nil {
		t.Fatalf("preparing env: %v", err)
	}

	if err := os.Setenv("VAULT_TOKEN", "dev-only-token"); err != nil {
		t.Fatalf("preparing env: %v", err)
	}

	v, err := vault.NewVaultClient()
	if err != nil {
		t.Fatalf("creating vault client: %v", err)
	}

	entityToken, err := v.Auth().Token().CreateWithRole(&api.TokenCreateRequest{EntityAlias: "sample"}, "sample")
	if err != nil {
		t.Fatalf("retrieving entity-bound vault token: %v", err)
	}

	et, err := entityToken.TokenID()
	if err != nil {
		t.Fatalf("extracting entity-bound vault token: %v", err)
	}

	v.SetToken(et)

	token, err := v.Logical().Read("identity/oidc/token/terraform-backend-sample")
	if err != nil {
		t.Fatalf("retrieving JWT token: %v", err)
	}

	a := NewJWTAuth("http://localhost:8200/v1/identity/oidc")

	{
		state := &terraform.State{
			ID:      terraform.GetStateID("other-project", "prod"),
			Project: "other-project",
			Name:    "prod",
		}

		ok, err := a.Authenticate(token.Data["token"].(string), state)
		if err != nil {
			t.Errorf("authenticating: %v", err)
		}

		if ok {
			t.Errorf("authentication succeeded")
		}
	}

	{
		state := &terraform.State{
			ID:      terraform.GetStateID("sample", "other-name"),
			Project: "sample",
			Name:    "other-name",
		}

		ok, err := a.Authenticate(token.Data["token"].(string), state)
		if err != nil {
			t.Errorf("authenticating: %v", err)
		}

		if ok {
			t.Errorf("authentication succeeded")
		}
	}

	{
		state := &terraform.State{
			ID:      terraform.GetStateID("sample", "prod"),
			Project: "sample",
			Name:    "prod",
		}

		ok, err := a.Authenticate(token.Data["token"].(string), state)
		if err != nil {
			t.Errorf("authenticating: %v", err)
		}

		if !ok {
			t.Errorf("authentication failed")
		}
	}
}
