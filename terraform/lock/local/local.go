package local

import (
	"fmt"
	"sync"

	"github.com/nimbolus/terraform-backend/terraform"
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
	if value, ok := l.db[s.ID]; ok {
		s.Lock = value
		return false, nil
	}
	l.db[s.ID] = s.Lock
	return true, nil
}

func (l *LocalLock) Unlock(s *terraform.State) (bool, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if _, ok := l.db[s.ID]; !ok {
		return false, fmt.Errorf("no lock for id %s found", s.ID)
	}
	delete(l.db, s.ID)
	return true, nil
}
