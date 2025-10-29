package redis

import (
	"testing"

	"github.com/google/uuid"

	"github.com/nimbolus/terraform-backend/pkg/client/redis/redistest"
	"github.com/nimbolus/terraform-backend/pkg/lock/util"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func TestLock(t *testing.T) {
	l := NewLock(redistest.NewPoolIfIntegrationTest(t))

	util.LockTest(t, l)
}

func TestGetLock(t *testing.T) {
	l := NewLock(redistest.NewPoolIfIntegrationTest(t))

	expectedLock := uuid.New().String()

	s := &terraform.State{
		ID:      terraform.GetStateID("test", "test"),
		Project: "test",
		Name:    "test",
		Lock:    terraform.LockInfo{ID: expectedLock},
	}

	{
		err := l.setLock(s)
		if err != nil {
			t.Error(err)
		}
	}

	// retrieve it again
	{
		lock, err := l.getLock(s)
		if err != nil {
			t.Error(err)
		}

		if lock.ID != expectedLock {
			t.Errorf("lock mismatch: %s != %s", lock.ID, expectedLock)
		}
	}

	// delete lock
	{
		err := l.deleteLock(s)
		if err != nil {
			t.Error(err)
		}
	}
}
