package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/auth/basic"
	localkms "github.com/nimbolus/terraform-backend/pkg/kms/local"
	"github.com/nimbolus/terraform-backend/pkg/lock"
	locallock "github.com/nimbolus/terraform-backend/pkg/lock/local"
	"github.com/nimbolus/terraform-backend/pkg/storage/filesystem"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func newUnitTestRouter(t *testing.T, maxBodySize int64) (http.Handler, lock.Locker) {
	t.Helper()

	viper.Set("max_body_size", maxBodySize)
	t.Cleanup(func() {
		viper.Set("max_body_size", nil)
	})

	store, err := filesystem.NewFileSystemStorage(t.TempDir())
	require.NoError(t, err)

	kms, err := localkms.NewKMS("x8DiIkAKRQT7cF55NQLkAZk637W3bGVOUjGeMX5ZGXY=")
	require.NoError(t, err)

	locker := locallock.NewLock()

	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/state/{project}/{name}", StateHandler(store, locker, kms))

	return r, locker
}

// lockBody returns a LOCK request body, padded to exactly size bytes.
func lockBody(t *testing.T, size int) []byte {
	t.Helper()

	body, err := json.Marshal(terraform.LockInfo{ID: "cf290ef3-6090-410e-9784-d017a4b1536a"})
	require.NoError(t, err)
	require.LessOrEqual(t, len(body), size)

	return append(body, bytes.Repeat([]byte(" "), size-len(body))...)
}

// readTracker records whether the request body was read.
type readTracker struct {
	io.Reader
	read bool
}

func (r *readTracker) Read(p []byte) (int, error) {
	r.read = true
	return r.Reader.Read(p)
}

func TestStateHandler_BodySizeLimit(t *testing.T) {
	const limit = 1024

	tests := []struct {
		name       string
		size       int
		wantCode   int
		wantLocked bool
	}{
		{name: "below limit", size: limit - 1, wantCode: http.StatusOK, wantLocked: true},
		{name: "at limit", size: limit, wantCode: http.StatusOK, wantLocked: true},
		{name: "above limit", size: limit + 1, wantCode: http.StatusRequestEntityTooLarge, wantLocked: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, locker := newUnitTestRouter(t, limit)

			req := httptest.NewRequest("LOCK", "/state/project1/example", bytes.NewReader(lockBody(t, tt.size)))
			req.SetBasicAuth("basic", "some-random-secret")

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			require.Equal(t, tt.wantCode, rec.Code)

			state := &terraform.State{ID: terraform.GetStateID("project1", "example")}
			ok, err := basic.NewBasicAuth().Authenticate("some-random-secret", state)
			require.NoError(t, err)
			require.True(t, ok)

			_, err = locker.GetLock(state)
			if tt.wantLocked {
				require.NoError(t, err)
			} else {
				require.Error(t, err, "state must not be locked if the body was rejected")
			}
		})
	}
}

func TestStateHandler_BodyNotReadBeforeAuthentication(t *testing.T) {
	r, _ := newUnitTestRouter(t, 1024)

	body := &readTracker{Reader: bytes.NewReader(lockBody(t, 512))}
	req := httptest.NewRequest("LOCK", "/state/project1/example", body)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, body.read, "body must not be read for unauthenticated requests")
}
