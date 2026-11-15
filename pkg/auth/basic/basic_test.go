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
		ID:       terraform.GetStateID(project, name),
		LegacyID: terraform.GetLegacyStateID(project, name),
		Project:  project,
		Name:     name,
	}

	ok, err := a.Authenticate(secret, state)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, state.ID, terraform.GetStateID(project, name))
	require.False(t, terraform.IsLegacyStateID(state.ID))
	require.NotEqual(t, state.LegacyID, terraform.GetLegacyStateID(project, name))
	require.True(t, terraform.IsLegacyStateID(state.LegacyID))
}

func TestAuth_LegacyID(t *testing.T) {
	state := &terraform.State{LegacyID: terraform.GetLegacyStateID("project1", "example")}

	_, err := NewBasicAuth().Authenticate("some-random-secret", state)
	require.NoError(t, err)
	// sha256("some-random-secret:" + sha256("project1-example")), the ID previous versions used
	require.Equal(t, "d82238e1158b32f0b445c5da058608a8c1d83551f890b19b7e90d78cce1a808d", state.LegacyID)
}
