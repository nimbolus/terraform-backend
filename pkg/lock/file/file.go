// Package file keeps each lock in a file, so that a lock stays when the
// server restarts. Use it with one server only: the mutex is in the process.
package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "file"

type Lock struct {
	mutex sync.Mutex
	dir   string
}

// NewLock makes the directory of the locks, with mode 0700, when it is absent.
func NewLock(dir string) (*Lock, error) {
	if dir == "" {
		return nil, errors.New("no directory for the locks: set LOCK_FILE_DIR")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating the lock directory %s: %w", dir, err)
	}
	return &Lock{dir: dir}, nil
}

func (l *Lock) GetName() string {
	return Name
}

func (l *Lock) path(s *terraform.State) (string, error) {
	if s.ID == "" || s.ID == "." || s.ID == ".." || strings.ContainsAny(s.ID, "/\x00") {
		return "", fmt.Errorf("state id %q is not a valid file name", s.ID)
	}
	return filepath.Join(l.dir, s.ID+".lock"), nil
}

// read returns fs.ErrNotExist when the state has no lock.
func (l *Lock) read(s *terraform.State) (terraform.LockInfo, error) {
	var lock terraform.LockInfo
	path, err := l.path(s)
	if err != nil {
		return lock, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return lock, err
	}
	if err := json.Unmarshal(raw, &lock); err != nil {
		return lock, fmt.Errorf("reading the lock %s: %w", path, err)
	}
	return lock, nil
}

// write replaces the lock file in one rename, after a sync of the data, so
// that a crash leaves the old file or the new file and never a part.
func (l *Lock) write(s *terraform.State) error {
	path, err := l.path(s)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(s.Lock)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(l.dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return l.syncDir()
}

func (l *Lock) remove(s *terraform.State) error {
	path, err := l.path(s)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return l.syncDir()
}

func (l *Lock) syncDir() error {
	d, err := os.Open(l.dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func (l *Lock) Lock(s *terraform.State) (bool, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lock, err := l.read(s)
	if err == nil {
		if lock.Equal(s.Lock) {
			// you already have the lock
			return true, nil
		}

		s.Lock = lock

		return false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	if err := l.write(s); err != nil {
		return false, err
	}

	return true, nil
}

func (l *Lock) Unlock(s *terraform.State) (bool, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lock, err := l.read(s)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if !lock.Equal(s.Lock) {
		s.Lock = lock

		return false, nil
	}

	if err := l.remove(s); err != nil {
		return false, err
	}

	return true, nil
}

func (l *Lock) GetLock(s *terraform.State) (terraform.LockInfo, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lock, err := l.read(s)
	if errors.Is(err, fs.ErrNotExist) {
		return terraform.LockInfo{}, fmt.Errorf("no lock found for state %s", s.ID)
	}

	return lock, err
}
