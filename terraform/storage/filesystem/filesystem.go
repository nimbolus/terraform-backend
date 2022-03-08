package filesystem

import (
	"errors"
	"fmt"
	"os"

	"github.com/nimbolus/terraform-backend/terraform"
)

type FileSystemStorage struct {
	directory string
}

func NewFileSystemStorage(directory string) (*FileSystemStorage, error) {
	err := os.MkdirAll(directory, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %v", directory, err)
	}

	return &FileSystemStorage{
		directory: directory,
	}, nil
}

func (f *FileSystemStorage) GetName() string {
	return "file"
}

func (f *FileSystemStorage) SaveState(s *terraform.State) error {
	return os.WriteFile(fmt.Sprintf("%s/%s.tfstate", f.directory, s.ID), s.Data, 0600)
}

func (f *FileSystemStorage) GetState(id string) (*terraform.State, error) {
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

func (f *FileSystemStorage) getFileName(id string) string {
	return fmt.Sprintf("%s/%s.tfstate", f.directory, id)
}
