//go:build integration || postgres
// +build integration postgres

package postgres

import (
	"testing"

	"github.com/nimbolus/terraform-backend/pkg/lock/util"
)

func TestLock(t *testing.T) {
	l, err := NewLock()
	if err != nil {
		t.Error(err)
	}

	util.LockTest(t, l)
}
