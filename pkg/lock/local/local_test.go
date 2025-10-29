package local

import (
	"testing"

	"github.com/nimbolus/terraform-backend/pkg/lock/util"
)

func TestLock(t *testing.T) {
	l := NewLock()

	util.LockTest(t, l)
}
