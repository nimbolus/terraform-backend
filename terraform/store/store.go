package store

import (
	"fmt"

	"github.com/nimbolus/terraform-backend/terraform"
	"github.com/nimbolus/terraform-backend/terraform/store/file"
	"github.com/nimbolus/terraform-backend/terraform/store/s3"
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
		viper.SetDefault("store_local_dir", "./states")
		s, err = file.NewFileStore(viper.GetString("store_local_dir"))
	case "s3":
		viper.SetDefault("store_s3_endpoint", "s3.amazonaws.com")
		viper.SetDefault("store_s3_use_ssl", true)
		viper.SetDefault("store_s3_access_key", "access-key-id")
		viper.SetDefault("store_s3_secret_key", "secret-access-key")
		viper.SetDefault("store_s3_bucket", "terraform-state")

		endpoint := viper.GetString("store_s3_endpoint")
		useSSL := viper.GetBool("store_s3_use_ssl")
		accessKey := viper.GetString("store_s3_access_key")
		secretKey := viper.GetString("store_s3_secret_key")
		bucket := viper.GetString("store_s3_bucket")

		s, err = s3.NewS3Store(endpoint, bucket, accessKey, secretKey, useSSL)
	default:
		err = fmt.Errorf("backend is not implemented")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store backend %s: %v", backend, err)
	}
	return
}
