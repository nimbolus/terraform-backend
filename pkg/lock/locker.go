package lock

import (
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

type Locker interface {
	GetName() string
	Lock(s *terraform.State) (ok bool, err error)
	Unlock(s *terraform.State) (ok bool, err error)
}
