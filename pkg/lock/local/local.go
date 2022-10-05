package local

import (
	"sync"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

type LocalLock struct {
	mutex sync.Mutex
	db    map[string][]byte
}

func NewLocalLock() *LocalLock {
	return &LocalLock{
		db: make(map[string][]byte),
	}
}

func (l *LocalLock) GetName() string {
	return "local"
}

func (l *LocalLock) Lock(s *terraform.State) (bool, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lock, ok := l.db[s.ID]
	if ok {
		if string(lock) == string(s.Lock) {
			// you already have the lock
			return true, nil
		}

		s.Lock = lock

		return false, nil
	}

	l.db[s.ID] = s.Lock

	return true, nil
}

func (l *LocalLock) Unlock(s *terraform.State) (bool, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lock, ok := l.db[s.ID]
	if !ok {
		return false, nil
	}

	if string(lock) != string(s.Lock) {
		s.Lock = lock

		return false, nil
	}

	delete(l.db, s.ID)
	return true, nil
}
