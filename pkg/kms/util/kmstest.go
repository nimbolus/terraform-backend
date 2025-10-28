package util

import (
	"crypto/rand"
	"testing"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/kms"
)

func KMSTest(t *testing.T, k kms.KMS) {
	viper.AutomaticEnv()

	t.Log(k.GetName())

	plain := []byte(rand.Text())

	t.Logf("plaintext: %s", plain)

	cipher, err := k.Encrypt(plain)
	if err != nil {
		t.Errorf("encrypting plaintext: %v", err)
	}

	t.Logf("ciphertext: %s", cipher)

	decrypted, err := k.Decrypt(cipher)
	if err != nil {
		t.Errorf("decrypting ciphertext: %v", err)
	}

	t.Logf("decrypted: %s", decrypted)

	if string(plain) != string(decrypted) {
		t.Errorf("decrypted ciphertext does not match: %s != %s", plain, decrypted)
	}
}
