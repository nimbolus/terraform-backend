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

func (l *BasicAuth) GetName() string {
	return Name
}

func (b *BasicAuth) Authenticate(secret string, s *terraform.State) (bool, error) {
	id := fmt.Sprintf("%s:%s", secret, s.ID)
	hash := sha256.Sum256([]byte(id))
	s.ID = fmt.Sprintf("%x", hash[:])
	return true, nil
}
