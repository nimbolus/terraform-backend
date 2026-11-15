package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	tf "github.com/nimbolus/terraform-backend/pkg/terraform"
)

func newUnitStateServer(t *testing.T) *httptest.Server {
	t.Helper()

	r, _ := newUnitTestRouter(t, 1024)
	s := httptest.NewServer(r)
	t.Cleanup(s.Close)

	return s
}

func doRequest(t *testing.T, method, url string, body []byte) (int, []byte) {
	t.Helper()

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.SetBasicAuth("basic", "some-random-secret")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()

	var buf bytes.Buffer
	_, err = buf.ReadFrom(res.Body)
	require.NoError(t, err)

	return res.StatusCode, buf.Bytes()
}

func TestDelete_RespectsLock(t *testing.T) {
	s := newUnitStateServer(t)
	addr := s.URL + "/state/project1/example"

	lockA, err := json.Marshal(tf.LockInfo{ID: "a", Who: "alice"})
	require.NoError(t, err)

	code, _ := doRequest(t, "LOCK", addr, lockA)
	require.Equal(t, http.StatusOK, code)

	code, _ = doRequest(t, http.MethodPost, addr+"?ID=a", []byte(`{"version":4}`))
	require.Equal(t, http.StatusOK, code)

	// another client can't delete the locked state
	code, body := doRequest(t, http.MethodDelete, addr, nil)
	require.Equal(t, http.StatusLocked, code)

	var holder tf.LockInfo
	require.NoError(t, json.Unmarshal(body, &holder))
	require.Equal(t, "a", holder.ID)

	code, _ = doRequest(t, http.MethodDelete, addr+"?ID=wrong", nil)
	require.Equal(t, http.StatusLocked, code)

	code, _ = doRequest(t, http.MethodGet, addr, nil)
	require.Equal(t, http.StatusOK, code, "state must still exist")

	// the lock holder can delete it, which releases the lock
	code, _ = doRequest(t, http.MethodDelete, addr+"?ID=a", nil)
	require.Equal(t, http.StatusOK, code)

	code, _ = doRequest(t, http.MethodGet, addr, nil)
	require.Equal(t, http.StatusNotFound, code)

	lockB, err := json.Marshal(tf.LockInfo{ID: "b"})
	require.NoError(t, err)

	code, _ = doRequest(t, "LOCK", addr, lockB)
	require.Equal(t, http.StatusOK, code, "no lock may be left behind after the deletion")
}

func TestDelete_Unlocked(t *testing.T) {
	s := newUnitStateServer(t)
	addr := s.URL + "/state/project1/example"

	lockA, err := json.Marshal(tf.LockInfo{ID: "a"})
	require.NoError(t, err)

	code, _ := doRequest(t, "LOCK", addr, lockA)
	require.Equal(t, http.StatusOK, code)
	code, _ = doRequest(t, http.MethodPost, addr+"?ID=a", []byte(`{"version":4}`))
	require.Equal(t, http.StatusOK, code)
	code, _ = doRequest(t, "UNLOCK", addr, lockA)
	require.Equal(t, http.StatusOK, code)

	code, _ = doRequest(t, http.MethodDelete, addr, nil)
	require.Equal(t, http.StatusOK, code)

	code, _ = doRequest(t, http.MethodGet, addr, nil)
	require.Equal(t, http.StatusNotFound, code)

	code, _ = doRequest(t, "LOCK", addr, lockA)
	require.Equal(t, http.StatusOK, code, "no lock may be left behind after the deletion")
}
