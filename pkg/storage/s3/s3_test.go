package s3

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/storage/util"
)

func TestStorage(t *testing.T) {
	if v := os.Getenv("INTEGRATION_TEST"); v == "" {
		t.Skip("env var INTEGRATION_TEST not set")
	}

	s, err := NewS3Storage("localhost:9000", "tf-backend-integration-test", "root", "password", false)
	require.NoError(t, err)

	util.StorageTest(t, s)
}
