package local

import (
	"fmt"
	"sync"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "local"

type Lock struct {
	mutex sync.Mutex
	db    map[string]terraform.LockInfo
}

func NewLock() *Lock {
	return &Lock{
		db: make(map[string]terraform.LockInfo),
	}
}

func (l *Lock) GetName() string {
	return Name
}

func (l *Lock) Lock(s *terraform.State) (bool, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lock, ok := l.db[s.ID]
	if ok {
		if lock.Equal(s.Lock) {
			// you already have the lock
			return true, nil
		}

		s.Lock = lock

		return false, nil
	}

	l.db[s.ID] = s.Lock

	return true, nil
}

func (l *Lock) Unlock(s *terraform.State) (bool, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lock, ok := l.db[s.ID]
	if !ok {
		return false, nil
	}

	if !lock.Equal(s.Lock) {
		s.Lock = lock

		return false, nil
	}

	delete(l.db, s.ID)

	return true, nil
}

func (l *Lock) GetLock(s *terraform.State) (terraform.LockInfo, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lock, ok := l.db[s.ID]
	if !ok {
		return terraform.LockInfo{}, fmt.Errorf("no lock found for state %s", s.ID)
	}

	return lock, nil
}
