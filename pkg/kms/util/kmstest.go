package util

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/kms"
)

func KMSTest(t *testing.T, k kms.KMS) {
	t.Log(k.GetName())

	plain := []byte(rand.Text())

	t.Logf("plaintext: %s", plain)

	cipher, err := k.Encrypt(plain)
	require.NoError(t, err)

	t.Logf("ciphertext: %v", cipher)

	decrypted, err := k.Decrypt(cipher)
	require.NoError(t, err)

	t.Logf("decrypted: %s", decrypted)

	require.Equal(t, plain, decrypted)
}
