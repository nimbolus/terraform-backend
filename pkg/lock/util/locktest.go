package util

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nimbolus/terraform-backend/pkg/lock"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func LockTest(t *testing.T, l lock.Locker) {
	t.Log(l.GetName())

	s1 := terraform.State{
		ID:      terraform.GetStateID("test", "test"),
		Project: "test",
		Name:    "test",
		Lock: terraform.LockInfo{
			ID:        uuid.New().String(),
			Path:      "",
			Operation: "LockTest",
			Who:       "test",
			Version:   "0.0.0",
			Created:   time.Now().String(),
			Info:      "",
		},
	}
	t.Logf("s1: %s", s1.Lock)

	s2 := terraform.State{
		ID:      terraform.GetStateID("test", "test"),
		Project: "test",
		Name:    "test",
		Lock: terraform.LockInfo{
			ID:        uuid.New().String(),
			Path:      "",
			Operation: "LockTest",
			Who:       "test",
			Version:   "0.0.0",
			Created:   time.Now().String(),
			Info:      "",
		},
	}
	t.Logf("s2: %s", s2.Lock)

	// copy of s2
	s3 := terraform.State{
		ID:      terraform.GetStateID("test", "test"),
		Project: "test",
		Name:    "test",
		Lock:    s2.Lock,
	}

	if locked, err := l.Lock(&s1); err != nil || !locked {
		t.Error(err)
	}

	if lock, err := l.GetLock(&s1); err != nil {
		t.Error(err)
	} else if !lock.Equal(s1.Lock) {
		t.Errorf("lock is not equal: %s != %s", lock, s1.Lock)
	}

	if locked, err := l.Lock(&s1); err != nil || !locked {
		t.Error("should be able to lock twice from the same process")
	}

	if locked, err := l.Lock(&s2); err != nil || locked {
		t.Error("should not be able to lock twice from different processes")
	}

	if !s2.Lock.Equal(s1.Lock) {
		t.Error("failed Lock() should return the lock information of the current lock")
	}

	if unlocked, err := l.Unlock(&s3); err != nil || unlocked {
		t.Error("should not be able to unlock with wrong lock")
	}

	if unlocked, err := l.Unlock(&s1); err != nil || !unlocked {
		t.Error(err)
	}
}
