package tfcontext_test

import (
	"testing"

	"github.com/ffddorf/tf-preview-github/pkg/tfcontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindBackend(t *testing.T) {
	be, err := tfcontext.FindBackend("./testdata")
	require.NoError(t, err)

	assert.Equal(t, "https://dummy-backend.example.com/state", be.Address)
	assert.Equal(t, "my_user", be.Username)
	assert.Empty(t, be.Password)
}
