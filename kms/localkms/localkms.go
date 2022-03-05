package localkms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

type LocalKMS struct {
	cipher cipher.AEAD
}

func NewLocalKMS(key string) (*LocalKMS, error) {
	gcm, err := buildCipher(key)
	if err != nil {
		return nil, err
	}

	return &LocalKMS{
		cipher: gcm,
	}, nil
}

func GenerateKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to create key for local KMS: %v", err)
	}

	return hex.EncodeToString(bytes), nil
}

func (v *LocalKMS) GetName() string {
	return "local"
}

func (s *LocalKMS) Encrypt(d []byte) ([]byte, error) {
	nonce := make([]byte, s.cipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to create nonce for seal with local KMS: %v", err)
	}

	return s.cipher.Seal(nonce, nonce, d, nil), nil
}

func (s *LocalKMS) Decrypt(d []byte) ([]byte, error) {
	nonceSize := s.cipher.NonceSize()
	nonce, ciphertext := d[:nonceSize], d[nonceSize:]

	plaintext, err := s.cipher.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to unseal with simple KMS: %v", err)
	}

	return plaintext, nil
}

func buildCipher(key string) (cipher.AEAD, error) {
	k, err := hex.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create simple KMS key: %v", err)
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("failed to create simple KMS cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create simple KMS gcm: %v", err)
	}

	return gcm, err
}
