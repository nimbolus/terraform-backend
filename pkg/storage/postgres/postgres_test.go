package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/client/postgres/postgrestest"
	"github.com/nimbolus/terraform-backend/pkg/storage/util"
)

func TestStorage(t *testing.T) {
	s, err := NewPostgresStorage(postgrestest.NewIfIntegrationTest(t), "states")
	require.NoError(t, err)

	util.StorageTest(t, s)
}
