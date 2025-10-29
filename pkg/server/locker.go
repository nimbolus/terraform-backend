package server

import (
	"fmt"

	"github.com/spf13/viper"

	pgclient "github.com/nimbolus/terraform-backend/pkg/client/postgres"
	redisclient "github.com/nimbolus/terraform-backend/pkg/client/redis"
	"github.com/nimbolus/terraform-backend/pkg/lock"
	"github.com/nimbolus/terraform-backend/pkg/lock/local"
	"github.com/nimbolus/terraform-backend/pkg/lock/postgres"
	"github.com/nimbolus/terraform-backend/pkg/lock/redis"
)

func GetLocker() (lock.Locker, error) {
	viper.SetDefault("lock_backend", local.Name)
	backend := viper.GetString("lock_backend")

	var locker lock.Locker

	switch backend {
	case local.Name:
		locker = local.NewLock()
	case redis.Name:
		locker = redis.NewLock(redisclient.NewPool())
	case postgres.Name:
		db, err := pgclient.NewClient()
		if err != nil {
			return nil, fmt.Errorf("creating postgres client: %w", err)
		}

		viper.SetDefault("lock_postgres_table", "locks")
		l, err := postgres.NewLock(db, viper.GetString("lock_postgres_table"))
		if err != nil {
			return nil, fmt.Errorf("failed to initialize lock backend %s: %v", backend, err)
		}

		locker = l
	default:
		return nil, fmt.Errorf("backend is not implemented")
	}

	viper.SetDefault("force_unlock_enabled", true)

	if viper.GetBool("force_unlock_enabled") {
		locker = lock.NewLockerWithForceUnlockEnabled(locker)
	}

	return locker, nil
}
