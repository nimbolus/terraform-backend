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
	lock, err := l.GetLock(state)
	if err != nil {
		return false, fmt.Errorf("failed to get lock for force-unlocking: %w", err)
	}

	if state.Lock.ID == "" {
		// terraform doesn't send the lock id and verifies it on client side
		state.Lock = lock
	} else {
		// opentofu does send the lock id, so it's possible to also verify it on server side
		if state.Lock.ID == lock.ID {
			state.Lock = lock
		}
	}

	return l.Locker.Unlock(state)
}
