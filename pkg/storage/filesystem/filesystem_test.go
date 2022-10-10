package filesystem

import (
	"testing"

	"github.com/nimbolus/terraform-backend/pkg/storage/util"
)

func TestStorage(t *testing.T) {
	s, err := NewFileSystemStorage("./storage")
	if err != nil {
		t.Error(err)
	}

	util.StorageTest(t, s)
}
