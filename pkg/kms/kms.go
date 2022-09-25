package kms

type KMS interface {
	GetName() string
	Encrypt(d []byte) ([]byte, error)
	Decrypt(d []byte) ([]byte, error)
}
