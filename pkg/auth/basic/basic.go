package basic

import (
	"crypto/sha256"
	"fmt"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "basic"

type BasicAuth struct{}

func NewBasicAuth() *BasicAuth {
	return &BasicAuth{}
}

func (b *BasicAuth) GetName() string {
	return Name
}

// Authenticate hashes the state ID with the secret, so every secret has its own set of states.
func (b *BasicAuth) Authenticate(secret string, s *terraform.State) (bool, error) {
	s.ID = terraform.HashID("basic", secret, s.ID)

	if s.LegacyID != "" {
		s.LegacyID = legacyID(secret, s.LegacyID)
	}

	return true, nil
}

// legacyID returns the ID used before v0.3.0.
// TODO: remove in 1.0.0.
func legacyID(secret, id string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", secret, id)))
	return fmt.Sprintf("%x", hash[:])
}
