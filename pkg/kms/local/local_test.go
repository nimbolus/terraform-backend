package local

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/kms/util"
)

func TestKMS(t *testing.T) {
	k, err := NewKMS("x8DiIkAKRQT7cF55NQLkAZk637W3bGVOUjGeMX5ZGXY=")
	require.NoError(t, err)

	util.KMSTest(t, k)
}
