package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/storage/util"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func TestStorage(t *testing.T) {
	s, err := NewFileSystemStorage("./storage")
	require.NoError(t, err)

	util.StorageTest(t, s)
}

func TestSaveStateIsAtomic(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileSystemStorage(dir)
	require.NoError(t, err)

	for _, data := range []string{"first", "second"} {
		require.NoError(t, s.SaveState(&terraform.State{ID: "id", Data: []byte(data)}))

		got, err := s.GetState("id")
		require.NoError(t, err)
		require.Equal(t, data, string(got.Data))
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temporary files may be left behind")
	require.Equal(t, "id.tfstate", entries[0].Name())

	info, err := entries[0].Info()
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestSaveStateFailureRemovesTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileSystemStorage(dir)
	require.NoError(t, err)

	require.NoError(t, s.SaveState(&terraform.State{ID: "id", Data: []byte("old")}))

	// a directory in place of the state file makes the rename fail
	require.NoError(t, os.Remove(filepath.Join(dir, "id.tfstate")))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "id.tfstate"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "id.tfstate", "keep"), nil, 0600))

	require.Error(t, s.SaveState(&terraform.State{ID: "id", Data: []byte("new")}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "the temporary file must be removed after a failure")
	require.Equal(t, "id.tfstate", entries[0].Name())
}

func TestCountStoredObjectsIgnoresTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileSystemStorage(dir)
	require.NoError(t, err)

	require.NoError(t, s.SaveState(&terraform.State{ID: "id", Data: []byte("state")}))

	// left over from a crash during a write
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".id.tmp-123"), []byte("partial"), 0600))

	count, err := s.CountStoredObjects()
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
