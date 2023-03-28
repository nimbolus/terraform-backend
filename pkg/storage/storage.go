package storage

import (
	"errors"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

var (
	ErrStateNotFound = errors.New("state does not exist")
)

type Storage interface {
	GetName() string
	SaveState(s *terraform.State) error
	GetState(id string) (*terraform.State, error)
	DeleteState(id string) error
}

type Countable interface {
	CountStoredObjects() (int, error)
}
