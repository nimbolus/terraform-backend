package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/auth/basic"
	"github.com/nimbolus/terraform-backend/pkg/kms"
	localkms "github.com/nimbolus/terraform-backend/pkg/kms/local"
	locallock "github.com/nimbolus/terraform-backend/pkg/lock/local"
	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/storage/filesystem"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const (
	testSecret = "some-random-secret"
	testLockID = "cf290ef3-6090-410e-9784-d017a4b1536a"
)

type migrationTest struct {
	t      *testing.T
	router http.Handler
	store  *filesystem.FileSystemStorage
	testStore
	locker *locallock.Lock
	kms    kms.KMS
}

// testStore fails saving states if failSave is set, fails deleting the state with the ID failDelete, and calls
// afterGet once after a state was read.
type testStore struct {
	storage.Storage
	failSave   bool
	failDelete string
	afterGet   func(id string, err error)
}

func (f *testStore) DeleteState(id string) error {
	if id == f.failDelete {
		return errors.New("delete failed")
	}

	return f.Storage.DeleteState(id)
}

func (f *testStore) SaveState(s *terraform.State) error {
	if f.failSave {
		return errors.New("save failed")
	}

	return f.Storage.SaveState(s)
}

func (f *testStore) GetState(id string) (*terraform.State, error) {
	s, err := f.Storage.GetState(id)

	if hook := f.afterGet; hook != nil {
		f.afterGet = nil
		hook(id, err)
	}

	return s, err
}

func newMigrationTest(t *testing.T) *migrationTest {
	t.Helper()

	store, err := filesystem.NewFileSystemStorage(t.TempDir())
	require.NoError(t, err)

	k, err := localkms.NewKMS("x8DiIkAKRQT7cF55NQLkAZk637W3bGVOUjGeMX5ZGXY=")
	require.NoError(t, err)

	m := &migrationTest{t: t, store: store, testStore: testStore{Storage: store}, locker: locallock.NewLock(), kms: k}

	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/state/{project}/{name}", StateHandler(&m.testStore, m.locker, k))
	m.router = r

	return m
}

// ids returns the new and the legacy storage ID of a state with basic auth.
func ids(t *testing.T, project, name string) (string, string) {
	t.Helper()

	s := &terraform.State{
		ID:       terraform.GetStateID(project, name),
		LegacyID: terraform.GetLegacyStateID(project, name),
	}

	_, err := basic.NewBasicAuth().Authenticate(testSecret, s)
	require.NoError(t, err)

	return s.ID, s.LegacyID
}

func (m *migrationTest) do(method, path string, body string) *httptest.ResponseRecorder {
	m.t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.SetBasicAuth("basic", testSecret)

	rec := httptest.NewRecorder()
	m.router.ServeHTTP(rec, req)

	return rec
}

func lockBodyWithID(t *testing.T) string {
	t.Helper()

	lock, err := json.Marshal(terraform.LockInfo{ID: testLockID})
	require.NoError(t, err)

	return string(lock)
}

// lockAndUnlock locks and unlocks the state, as a Terraform run without changes does.
func (m *migrationTest) lockAndUnlock(path string) {
	m.t.Helper()

	require.Equal(m.t, http.StatusOK, m.do("LOCK", path, lockBodyWithID(m.t)).Code)
	require.Equal(m.t, http.StatusOK, m.do("UNLOCK", path, lockBodyWithID(m.t)).Code)
}

// write locks the state, writes it and unlocks it again, as Terraform does.
func (m *migrationTest) write(path, data string) {
	m.t.Helper()

	require.Equal(m.t, http.StatusOK, m.do("LOCK", path, lockBodyWithID(m.t)).Code)
	require.Equal(m.t, http.StatusOK, m.do(http.MethodPost, path+"?ID="+testLockID, data).Code)
	require.Equal(m.t, http.StatusOK, m.do("UNLOCK", path, lockBodyWithID(m.t)).Code)
}

func (m *migrationTest) requireState(path, data string) {
	m.t.Helper()

	rec := m.do(http.MethodGet, path, "")
	require.Equal(m.t, http.StatusOK, rec.Code)
	require.Equal(m.t, data, rec.Body.String())
}

// saveLegacy stores a state as previous versions did: encrypted, without metadata.
func (m *migrationTest) saveLegacy(id, data string) {
	m.t.Helper()

	encrypted, err := m.kms.Encrypt([]byte(data))
	require.NoError(m.t, err)
	require.NoError(m.t, m.store.SaveState(&terraform.State{ID: id, Data: encrypted}))
}

func (m *migrationTest) requireExists(id string, exists bool) {
	m.t.Helper()

	_, err := m.store.GetState(id)
	if exists {
		require.NoError(m.t, err)
	} else {
		require.ErrorIs(m.t, err, storage.ErrStateNotFound)
	}
}

func TestMigration_ReadLegacy(t *testing.T) {
	m := newMigrationTest(t)
	id, legacyID := ids(t, "project1", "example")
	m.saveLegacy(legacyID, "legacy")

	m.requireState("/state/project1/example", "legacy")

	// reading doesn't migrate
	m.requireExists(legacyID, true)
	m.requireExists(id, false)
}

func TestMigration_MigrateOnLock(t *testing.T) {
	m := newMigrationTest(t)
	id, legacyID := ids(t, "project1", "example")
	m.saveLegacy(legacyID, "legacy")

	m.lockAndUnlock("/state/project1/example")

	m.requireExists(legacyID, false)
	m.requireState("/state/project1/example", "legacy")

	stored, err := m.store.GetState(id)
	require.NoError(t, err)

	var s terraform.StoredState
	require.NoError(t, json.Unmarshal(stored.Data, &s))
	require.Equal(t, terraform.FormatV1, s.Metadata.Format)
	require.Equal(t, "dev", s.Metadata.TerraformBackendVersion)
	require.Equal(t, "project1", s.Metadata.Project)
	require.Equal(t, "example", s.Metadata.State)
	require.False(t, s.Metadata.LastUpdated.IsZero())
	require.NotContains(t, string(s.Data), "legacy", "the data must be encrypted")
}

func TestMigration_ReadDuringMigration(t *testing.T) {
	m := newMigrationTest(t)
	id, legacyID := ids(t, "project1", "example")
	m.saveLegacy(legacyID, "legacy")

	// migrate the state right after the read missed the new ID, before it reads the legacy ID
	m.afterGet = func(gotID string, err error) {
		require.Equal(t, id, gotID)
		require.ErrorIs(t, err, storage.ErrStateNotFound)
		m.lockAndUnlock("/state/project1/example")
	}

	m.requireState("/state/project1/example", "legacy")
	require.Nil(t, m.afterGet, "the migration must have run")
	m.requireExists(legacyID, false)
}

func TestMigration_WriteAfterMigration(t *testing.T) {
	m := newMigrationTest(t)
	_, legacyID := ids(t, "project1", "example")
	m.saveLegacy(legacyID, "legacy")

	m.write("/state/project1/example", "new")

	m.requireExists(legacyID, false)
	m.requireState("/state/project1/example", "new")
}

func TestMigration_NoMigrationWithoutLock(t *testing.T) {
	m := newMigrationTest(t)
	id, legacyID := ids(t, "project1", "example")
	m.saveLegacy(legacyID, "legacy")

	ok, err := m.locker.Lock(&terraform.State{ID: id, Lock: terraform.LockInfo{ID: "other"}})
	require.NoError(t, err)
	require.True(t, ok)

	require.Equal(t, http.StatusLocked, m.do("LOCK", "/state/project1/example", lockBodyWithID(t)).Code)

	m.requireExists(legacyID, true)
	m.requireExists(id, false)
}

func TestMigration_StaleLegacyCopyIsDeleted(t *testing.T) {
	m := newMigrationTest(t)
	_, legacyID := ids(t, "project1", "example")

	m.write("/state/project1/example", "new")
	m.saveLegacy(legacyID, "legacy")

	m.lockAndUnlock("/state/project1/example")

	m.requireExists(legacyID, false)
	m.requireState("/state/project1/example", "new")
}

func TestMigration_InvalidNewStateKeepsLegacyCopy(t *testing.T) {
	m := newMigrationTest(t)
	id, legacyID := ids(t, "project1", "example")
	m.saveLegacy(legacyID, "legacy")
	require.NoError(t, m.store.SaveState(&terraform.State{ID: id, Data: []byte(`{"metadata": {"form`)}))

	m.lockAndUnlock("/state/project1/example")

	m.requireExists(legacyID, true)
}

func TestMigration_FailedMigrationKeepsLegacyState(t *testing.T) {
	m := newMigrationTest(t)
	id, legacyID := ids(t, "project1", "example")
	m.saveLegacy(legacyID, "legacy")

	m.failSave = true
	m.lockAndUnlock("/state/project1/example")
	m.failSave = false

	m.requireExists(id, false)
	m.requireState("/state/project1/example", "legacy")

	// the write after a failed migration still removes the legacy copy
	m.write("/state/project1/example", "new")

	m.requireExists(legacyID, false)
	m.requireState("/state/project1/example", "new")
}

func TestMigration_NewIDTakesPrecedence(t *testing.T) {
	m := newMigrationTest(t)
	_, legacyID := ids(t, "project1", "example")

	m.write("/state/project1/example", "new")
	m.saveLegacy(legacyID, "legacy")

	m.requireState("/state/project1/example", "new")
}

func TestMigration_Delete(t *testing.T) {
	m := newMigrationTest(t)
	id, legacyID := ids(t, "project1", "example")

	m.write("/state/project1/example", "new")
	m.saveLegacy(legacyID, "legacy")

	require.Equal(t, http.StatusOK, m.do(http.MethodDelete, "/state/project1/example", "").Code)

	m.requireExists(id, false)
	m.requireExists(legacyID, false)
	require.Equal(t, http.StatusNotFound, m.do(http.MethodGet, "/state/project1/example", "").Code)
}

func TestMigration_DeleteFailsOnLegacyCopy(t *testing.T) {
	m := newMigrationTest(t)
	id, legacyID := ids(t, "project1", "example")

	m.write("/state/project1/example", "new")
	m.saveLegacy(legacyID, "legacy")
	m.failDelete = legacyID

	require.Equal(t, http.StatusInternalServerError, m.do(http.MethodDelete, "/state/project1/example", "").Code)

	// nothing was deleted, so the state isn't replaced by its stale legacy copy
	m.requireExists(id, true)
	m.requireState("/state/project1/example", "new")
}

func TestMigration_DeleteLegacyOnly(t *testing.T) {
	m := newMigrationTest(t)
	_, legacyID := ids(t, "project1", "example")
	m.saveLegacy(legacyID, "legacy")

	require.Equal(t, http.StatusOK, m.do(http.MethodDelete, "/state/project1/example", "").Code)

	m.requireExists(legacyID, false)
}

func TestMigration_CollidingStatesAreSeparated(t *testing.T) {
	m := newMigrationTest(t)
	_, legacyID := ids(t, "a-b", "c")
	_, otherLegacyID := ids(t, "a", "b-c")
	require.Equal(t, legacyID, otherLegacyID, "the legacy IDs collide")

	m.saveLegacy(legacyID, "shared")
	m.lockAndUnlock("/state/a-b/c")

	// the shared legacy copy was migrated to a-b/c
	m.requireState("/state/a-b/c", "shared")
	require.Equal(t, http.StatusNotFound, m.do(http.MethodGet, "/state/a/b-c", "").Code)

	m.write("/state/a-b/c", "a-b/c")

	m.write("/state/a/b-c", "a/b-c")

	m.requireState("/state/a-b/c", "a-b/c")
	m.requireState("/state/a/b-c", "a/b-c")
}

func TestCheckMigration_NoStates(t *testing.T) {
	m := newMigrationTest(t)

	var out bytes.Buffer

	_, err := CheckMigration(m.store, &out)
	require.ErrorContains(t, err, "found no states")
	require.Empty(t, out.String())
}

func TestCheckMigration(t *testing.T) {
	m := newMigrationTest(t)
	_, legacyID := ids(t, "project1", "legacy")
	m.saveLegacy(legacyID, "legacy")
	m.write("/state/project1/new", "new")

	var out bytes.Buffer

	legacy, err := CheckMigration(m.store, &out)
	require.NoError(t, err)
	require.Equal(t, 1, legacy)
	require.Equal(t, "not migrated: "+legacyID+"\n1 of 2 states not migrated\n", out.String())

	m.lockAndUnlock("/state/project1/legacy")
	out.Reset()

	legacy, err = CheckMigration(m.store, &out)
	require.NoError(t, err)
	require.Equal(t, 0, legacy)
	require.Equal(t, "0 of 2 states not migrated\n", out.String())
}
