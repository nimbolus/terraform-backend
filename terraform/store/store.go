package store

import (
	"fmt"

	"github.com/nimbolus/terraform-backend/terraform"
	"github.com/nimbolus/terraform-backend/terraform/store/filestore"
	"github.com/spf13/viper"
)

type Store interface {
	GetName() string
	SaveState(s *terraform.State) error
	GetState(id string) (*terraform.State, error)
}

func GetStore() (s Store, err error) {
	viper.SetDefault("store_backend", "file")
	backend := viper.GetString("store_backend")

	switch backend {
	case "file":
		s, err = filestore.NewFileStore("./example/states")
	default:
		err = fmt.Errorf("backend is not implemented")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store backend %s: %v", backend, err)
	}
	return
}
