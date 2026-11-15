package filesystem

import (
	"errors"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "fs"

const fileSuffix = ".tfstate"

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
	return Name
}

// SaveState writes the state to a temporary file in the same directory, syncs it and renames it
// to the state file. The rename is atomic, so a crash leaves either the old or the new state.
func (f *FileSystemStorage) SaveState(s *terraform.State) (err error) {
	tmp, err := os.CreateTemp(f.directory, "."+s.ID+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary state file: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tmp.Close() // already closed if the rename failed, the write error is what matters
			if rmErr := os.Remove(tmp.Name()); rmErr != nil {
				log.Warnf("failed to remove temporary state file %s: %v", tmp.Name(), rmErr)
			}
		}
	}()

	if _, err = tmp.Write(s.Data); err != nil {
		return fmt.Errorf("failed to write temporary state file: %w", err)
	}

	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary state file: %w", err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary state file: %w", err)
	}

	if err = os.Rename(tmp.Name(), f.getFileName(s.ID)); err != nil {
		return fmt.Errorf("failed to rename temporary state file: %w", err)
	}

	// Sync the directory, so that the rename survives a crash. The state is written already, so a failure only
	// affects durability and doesn't fail the write. Some filesystems don't support syncing directories.
	if err := syncDir(f.directory); err != nil {
		log.Warnf("failed to sync state directory %s, the write may not survive a crash: %v", f.directory, err)
	}

	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close() // nolint: errcheck // read-only handle, the sync error is what matters

	return dir.Sync()
}

func (f *FileSystemStorage) GetState(id string) (*terraform.State, error) {
	d, err := os.ReadFile(f.getFileName(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, storage.ErrStateNotFound
	} else if err != nil {
		return nil, err
	}

	return &terraform.State{
		Data: d,
	}, nil
}

func (f *FileSystemStorage) DeleteState(id string) error {
	if err := os.Remove(f.getFileName(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (f *FileSystemStorage) getFileName(id string) string {
	return fmt.Sprintf("%s/%s%s", f.directory, id, fileSuffix)
}

func (f *FileSystemStorage) ListStateIDs() ([]string, error) {
	entries, err := os.ReadDir(f.directory)
	if err != nil {
		return nil, err
	}

	var ids []string

	for _, e := range entries {
		if id, ok := strings.CutSuffix(e.Name(), fileSuffix); ok && e.Type().IsRegular() {
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// CountStoredObjects counts the state files. Temporary files of writes in progress, or left over from a crash, are not
// counted.
func (f *FileSystemStorage) CountStoredObjects() (int, error) {
	ids, err := f.ListStateIDs()
	if err != nil {
		return 0, err
	}

	return len(ids), nil
}
