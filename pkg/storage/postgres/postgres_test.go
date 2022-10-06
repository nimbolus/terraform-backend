//go:build integration || postgres
// +build integration postgres

package postgres

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/storage/util"
)

func init() {
	viper.AutomaticEnv()
}

func TestStorage(t *testing.T) {
	s, err := NewPostgresStorage("states")
	if err != nil {
		t.Error(err)
	}

	util.StorageTest(t, s)
}
