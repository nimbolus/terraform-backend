package basic

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/nimbolus/terraform-backend/terraform"
)

type BasicAuth struct{}

func NewBasicAuth() *BasicAuth {
	return &BasicAuth{}
}

func (l *BasicAuth) GetName() string {
	return "basic"
}

func (b *BasicAuth) Authenticate(req *http.Request, s *terraform.State) (bool, error) {
	username, password, ok := req.BasicAuth()
	if !ok {
		return false, fmt.Errorf("no basic auth header found")
	}

	id := fmt.Sprintf("%s:%s;%s", username, password, s.ID)
	hash := sha256.Sum256([]byte(id))
	s.ID = fmt.Sprintf("%x", hash[:])
	return true, nil
}
