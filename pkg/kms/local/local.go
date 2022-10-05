package local

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const Name = "local"

type KMS struct {
	cipher cipher.AEAD
}

func NewKMS(key string) (*KMS, error) {
	gcm, err := buildCipher(key)
	if err != nil {
		return nil, err
	}

	return &KMS{
		cipher: gcm,
	}, nil
}

func GenerateKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to create key for local KMS: %v", err)
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}

func (v *KMS) GetName() string {
	return Name
}

func (s *KMS) Encrypt(d []byte) ([]byte, error) {
	nonce := make([]byte, s.cipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to create nonce for seal with local KMS: %v", err)
	}

	return s.cipher.Seal(nonce, nonce, d, nil), nil
}

func (s *KMS) Decrypt(d []byte) ([]byte, error) {
	nonceSize := s.cipher.NonceSize()
	nonce, ciphertext := d[:nonceSize], d[nonceSize:]

	plaintext, err := s.cipher.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to unseal with simple KMS: %v", err)
	}

	return plaintext, nil
}

func buildCipher(key string) (cipher.AEAD, error) {
	k, err := base64.StdEncoding.DecodeString(key)
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
