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
	// DeleteState deletes the state, a state that doesn't exist is not an error.
	DeleteState(id string) error
}

// Lister is implemented by storage backends that can list the IDs of all stored states.
type Lister interface {
	ListStateIDs() ([]string, error)
}

type Countable interface {
	CountStoredObjects() (int, error)
}
