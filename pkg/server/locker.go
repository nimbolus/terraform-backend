package server

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/lock"
	"github.com/nimbolus/terraform-backend/pkg/lock/local"
	"github.com/nimbolus/terraform-backend/pkg/lock/postgres"
	"github.com/nimbolus/terraform-backend/pkg/lock/redis"
)

func GetLocker() (l lock.Locker, err error) {
	viper.SetDefault("lock_backend", local.Name)
	backend := viper.GetString("lock_backend")

	switch backend {
	case local.Name:
		l = local.NewLock()
	case redis.Name:
		l = redis.NewLock()
	case postgres.Name:
		viper.SetDefault("lock_postgres_table", "locks")
		l, err = postgres.NewLock(viper.GetString("lock_postgres_table"))
	default:
		err = fmt.Errorf("backend is not implemented")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize lock backend %s: %v", backend, err)
	}

	viper.SetDefault("force_unlock_enabled", true)

	if viper.GetBool("force_unlock_enabled") {
		l = lock.NewLockerWithForceUnlockEnabled(l)
	}

	return
}
