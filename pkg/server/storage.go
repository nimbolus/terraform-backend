package server

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/storage/filesystem"
	"github.com/nimbolus/terraform-backend/pkg/storage/postgres"
	"github.com/nimbolus/terraform-backend/pkg/storage/s3"
)

func GetStorage() (s storage.Storage, err error) {
	viper.SetDefault("storage_backend", filesystem.Name)
	backend := viper.GetString("storage_backend")

	switch backend {
	case filesystem.Name:
		viper.SetDefault("storage_fs_dir", "./states")
		s, err = filesystem.NewFileSystemStorage(viper.GetString("storage_fs_dir"))
	case postgres.Name:
		viper.SetDefault("storage_postgres_table", "states")
		s, err = postgres.NewPostgresStorage(viper.GetString("storage_postgres_table"))
	case s3.Name:
		viper.SetDefault("storage_s3_endpoint", "s3.amazonaws.com")
		viper.SetDefault("storage_s3_use_ssl", true)
		viper.SetDefault("storage_s3_bucket", "terraform-state")

		endpoint := viper.GetString("storage_s3_endpoint")
		useSSL := viper.GetBool("storage_s3_use_ssl")
		accessKey := viper.GetString("storage_s3_access_key")
		secretKey := viper.GetString("storage_s3_secret_key")
		bucket := viper.GetString("storage_s3_bucket")

		s, err = s3.NewS3Storage(endpoint, bucket, accessKey, secretKey, useSSL)
	default:
		err = fmt.Errorf("backend is not implemented")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage backend %s: %v", backend, err)
	}
	return
}
