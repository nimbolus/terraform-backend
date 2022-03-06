package file

import (
	"errors"
	"fmt"
	"os"

	"github.com/nimbolus/terraform-backend/terraform"
)

type FileStore struct {
	directory string
}

func NewFileStore(directory string) (*FileStore, error) {
	err := os.MkdirAll(directory, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %v", directory, err)
	}

	return &FileStore{
		directory: directory,
	}, nil
}

func (f *FileStore) GetName() string {
	return "file"
}

func (f *FileStore) SaveState(s *terraform.State) error {
	return os.WriteFile(fmt.Sprintf("%s/%s.tfstate", f.directory, s.ID), s.Data, 0600)
}

func (f *FileStore) GetState(id string) (*terraform.State, error) {
	if _, err := os.Stat(f.getFileName(id)); errors.Is(err, os.ErrNotExist) {
		f, err := os.Create(f.getFileName(id))
		if err != nil {
			return nil, err
		}
		defer f.Close()

		return &terraform.State{}, nil
	}

	d, err := os.ReadFile(f.getFileName(id))
	if err != nil {
		return nil, err
	}

	return &terraform.State{
		Data: d,
	}, nil
}

func (f *FileStore) getFileName(id string) string {
	return fmt.Sprintf("%s/%s.tfstate", f.directory, id)
}
