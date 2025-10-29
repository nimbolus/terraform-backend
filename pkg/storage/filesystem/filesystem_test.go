package filesystem

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/storage/util"
)

func TestStorage(t *testing.T) {
	s, err := NewFileSystemStorage("./storage")
	require.NoError(t, err)

	util.StorageTest(t, s)
}
