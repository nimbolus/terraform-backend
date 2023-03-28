package storage

import (
	"github.com/nimbolus/terraform-backend/pkg/terraform"
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
