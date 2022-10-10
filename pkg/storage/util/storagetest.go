package util

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func init() {
	viper.AutomaticEnv()
}

func StorageTest(t *testing.T, s storage.Storage) {
	state := &terraform.State{
		ID:      terraform.GetStateID("test", "test"),
		Project: "test",
		Name:    "test",
		Data:    []byte("test"),
	}

	if err := s.SaveState(state); err != nil {
		t.Error(err)
	}

	savedState, err := s.GetState(state.ID)
	if err != nil {
		t.Error(err)
	}

	if string(state.Data) != string(savedState.Data) {
		t.Errorf("state data does not match")
	}

	state.Data = []byte("test2")

	if err := s.SaveState(state); err != nil {
		t.Error(err)
	}

	savedState, err = s.GetState(state.ID)
	if err != nil {
		t.Error(err)
	}

	if string(state.Data) != string(savedState.Data) {
		t.Errorf("state data does not match")
	}

	err = s.DeleteState(state.ID)
	if err != nil {
		t.Error(err)
	}
}
