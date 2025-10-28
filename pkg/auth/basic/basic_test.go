package basic

import (
	"crypto/rand"
	"testing"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func init() {
	viper.AutomaticEnv()
}

func TestAuth(t *testing.T) {
	a := NewBasicAuth()

	project := "example-project"
	name := "prod"
	secret := rand.Text()

	state := &terraform.State{
		ID:      terraform.GetStateID(project, name),
		Project: project,
		Name:    name,
	}

	ok, err := a.Authenticate(secret, state)
	if err != nil {
		t.Errorf("authenticating: %v", err)
	}

	if !ok {
		t.Errorf("authentication failed")
	}

	if state.ID == terraform.GetStateID(project, name) {
		t.Errorf("state.ID should have been changed")
	}
}
