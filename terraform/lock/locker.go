package lock

import (
	"fmt"

	"github.com/nimbolus/terraform-backend/terraform"
	"github.com/nimbolus/terraform-backend/terraform/lock/local"
	"github.com/nimbolus/terraform-backend/terraform/lock/redis"
	"github.com/spf13/viper"
)

type Locker interface {
	GetName() string
	Lock(s *terraform.State) (ok bool, err error)
	Unlock(s *terraform.State) (ok bool, err error)
}

func GetLocker() (l Locker, err error) {
	viper.SetDefault("lock_backend", "local")
	backend := viper.GetString("lock_backend")

	switch backend {
	case "local":
		l = local.NewLocalLock()
	case "redis":
		l = redis.NewRedisLock()
	default:
		err = fmt.Errorf("backend is not implemented")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize lock backend %s: %v", backend, err)
	}
	return
}
