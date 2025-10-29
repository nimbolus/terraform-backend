package util

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func StorageTest(t *testing.T, s storage.Storage) {
	state := &terraform.State{
		ID:      terraform.GetStateID("test", "test"),
		Project: "test",
		Name:    "test",
		Data:    []byte("test"),
	}

	nonExisting, err := s.GetState(state.ID)
	require.ErrorIs(t, err, storage.ErrStateNotFound)
	require.Nil(t, nonExisting)

	require.NoError(t, s.SaveState(state))

	savedState, err := s.GetState(state.ID)
	require.NoError(t, err)
	require.Equal(t, state.Data, savedState.Data)

	state.Data = []byte("test2")

	require.NoError(t, s.SaveState(state))

	savedState, err = s.GetState(state.ID)
	require.NoError(t, err)
	require.Equal(t, state.Data, savedState.Data)

	err = s.DeleteState(state.ID)
	require.NoError(t, err)
}
