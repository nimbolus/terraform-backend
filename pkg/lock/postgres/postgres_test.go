package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/client/postgres/postgrestest"
	"github.com/nimbolus/terraform-backend/pkg/lock/util"
)

func TestLock(t *testing.T) {
	l, err := NewLock(postgrestest.NewIfIntegrationTest(t), "locks")
	require.NoError(t, err)

	util.LockTest(t, l)
}
