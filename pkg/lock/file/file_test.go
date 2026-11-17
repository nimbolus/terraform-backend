package file

import (
	"testing"

	"github.com/nimbolus/terraform-backend/pkg/lock/util"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func TestLock(t *testing.T) {
	l, err := NewLock(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	util.LockTest(t, l)
}

// A lock stays when a new server starts on the same directory.
func TestLockSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	s := terraform.State{
		ID:   terraform.GetStateID("test", "restart"),
		Lock: terraform.LockInfo{ID: "holder", Operation: "apply", Who: "test"},
	}
	other := terraform.State{ID: s.ID, Lock: terraform.LockInfo{ID: "other"}}

	before, err := NewLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := before.Lock(&s); err != nil || !ok {
		t.Fatalf("lock: %v %v", ok, err)
	}

	after, err := NewLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := after.Lock(&other); err != nil || ok {
		t.Fatalf("a second holder got the lock after the restart: %v %v", ok, err)
	}
	if !other.Lock.Equal(s.Lock) {
		t.Fatalf("the restarted server reports %v, not the holder %v", other.Lock, s.Lock)
	}
	if ok, err := after.Unlock(&s); err != nil || !ok {
		t.Fatalf("the holder cannot unlock after the restart: %v %v", ok, err)
	}
}

func TestLockRefusesPathInID(t *testing.T) {
	l, err := NewLock(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Lock(&terraform.State{ID: "../escape"}); err == nil {
		t.Fatal("a state id with a slash got a lock file")
	}
}
