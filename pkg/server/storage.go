package server

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/internal"
	pgclient "github.com/nimbolus/terraform-backend/pkg/client/postgres"
	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/storage/filesystem"
	"github.com/nimbolus/terraform-backend/pkg/storage/postgres"
	"github.com/nimbolus/terraform-backend/pkg/storage/s3"
)

func GetStorage() (storage.Storage, error) {
	viper.SetDefault("storage_backend", filesystem.Name)
	backend := viper.GetString("storage_backend")

	switch backend {
	case filesystem.Name:
		viper.SetDefault("storage_fs_dir", "./states")
		s, err := filesystem.NewFileSystemStorage(viper.GetString("storage_fs_dir"))
		if err != nil {
			return nil, fmt.Errorf("failed to initialize storage backend %s: %v", backend, err)
		}

		return s, nil
	case postgres.Name:
		db, err := pgclient.NewClient()
		if err != nil {
			return nil, fmt.Errorf("creating postgres client: %w", err)
		}

		viper.SetDefault("storage_postgres_table", "states")

		s, err := postgres.NewPostgresStorage(db, viper.GetString("storage_postgres_table"))
		if err != nil {
			return nil, fmt.Errorf("failed to initialize storage backend %s: %v", backend, err)
		}

		return s, nil
	case s3.Name:
		viper.SetDefault("storage_s3_endpoint", "s3.amazonaws.com")
		viper.SetDefault("storage_s3_use_ssl", true)
		viper.SetDefault("storage_s3_bucket", "terraform-state")

		endpoint := viper.GetString("storage_s3_endpoint")
		useSSL := viper.GetBool("storage_s3_use_ssl")
		accessKey := viper.GetString("storage_s3_access_key")

		secretKey, secretErr := internal.SecretEnvOrFile("storage_s3_secret_key", "storage_s3_secret_key_file")
		if secretErr != nil {
			return nil, fmt.Errorf("getting storage s3 secret key: %w", secretErr)
		}

		bucket := viper.GetString("storage_s3_bucket")

		s, err := s3.NewS3Storage(endpoint, bucket, accessKey, secretKey, useSSL)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize storage backend %s: %v", backend, err)
		}

		return s, nil
	default:
		return nil, fmt.Errorf("backend is not implemented")
	}
}
