package basic

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

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
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, state.ID, terraform.GetStateID(project, name))
}
