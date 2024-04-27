package lock

import (
	"fmt"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

type Locker interface {
	GetName() string
	Lock(s *terraform.State) (ok bool, err error)
	Unlock(s *terraform.State) (ok bool, err error)
	GetLock(s *terraform.State) (terraform.LockInfo, error)
}

type LockerWithForceUnlockEnabled struct {
	Locker
}

func NewLockerWithForceUnlockEnabled(l Locker) *LockerWithForceUnlockEnabled {
	return &LockerWithForceUnlockEnabled{l}
}

func (l *LockerWithForceUnlockEnabled) Unlock(state *terraform.State) (bool, error) {
	if state.Lock.ID == "" {
		lock, err := l.GetLock(state)
		if err != nil {
			return false, fmt.Errorf("failed to get lock for force-unlocking: %w", err)
		}
		state.Lock = lock
	}

	return l.Locker.Unlock(state)
}
