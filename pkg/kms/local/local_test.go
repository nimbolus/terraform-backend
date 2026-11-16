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

func TestDecryptShortCiphertext(t *testing.T) {
	k, err := NewKMS("x8DiIkAKRQT7cF55NQLkAZk637W3bGVOUjGeMX5ZGXY=")
	require.NoError(t, err)

	minLen := k.cipher.NonceSize() + k.cipher.Overhead()
	for _, n := range []int{0, 1, 5, k.cipher.NonceSize(), minLen - 1} {
		_, err := k.Decrypt(make([]byte, n))
		require.Error(t, err, "length %d", n)
	}

	// minimum length passes the length check, but fails authentication
	_, err = k.Decrypt(make([]byte, minLen))
	require.Error(t, err)
}
