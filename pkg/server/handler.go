package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/auth"
	"github.com/nimbolus/terraform-backend/pkg/kms"
	"github.com/nimbolus/terraform-backend/pkg/lock"
	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
	"github.com/nimbolus/terraform-backend/pkg/version"
)

func HTTPResponse(w http.ResponseWriter, r *http.Request, code int, body string) {
	log.Tracef("response: %d %s", code, body)
	w.WriteHeader(code)
	fmt.Fprint(w, body)

	requestCount.With(prometheus.Labels{
		"method": r.Method,
		"path":   routeTemplate(r),
		"code":   strconv.Itoa(code),
	}).Inc()
}

// routeTemplate returns the path template of the matched route (e.g. /state/{project}/{name}),
// so that the metric labels don't grow with every requested state path.
func routeTemplate(r *http.Request) string {
	if route := mux.CurrentRoute(r); route != nil {
		if tpl, err := route.GetPathTemplate(); err == nil {
			return tpl
		}
	}

	return "unknown"
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	log.Debugf("%s %s", r.Method, r.URL.Path)
	HTTPResponse(w, r, http.StatusOK, "")
}

func StateHandler(store storage.Storage, locker lock.Locker, kms kms.KMS) func(http.ResponseWriter, *http.Request) {
	viper.SetDefault("max_body_size", 100<<20)
	maxBodySize := viper.GetInt64("max_body_size")

	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		vars := mux.Vars(r)
		state := &terraform.State{
			ID:       terraform.GetStateID(vars["project"], vars["name"]),
			LegacyID: terraform.GetLegacyStateID(vars["project"], vars["name"]),
			Project:  vars["project"],
			Name:     vars["name"],
		}

		log.Infof("%s %s", r.Method, r.URL.Path)

		if ok, err := auth.Authenticate(r, state); err != nil {
			log.Warnf("failed process authentication for state id %s: %v", state.ID, err)
			HTTPResponse(w, r, http.StatusForbidden, err.Error())

			return
		} else if !ok {
			log.Warnf("failed to authenticate request for state id %s", state.ID)
			HTTPResponse(w, r, http.StatusForbidden, "Permission denied")

			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodySize))
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				log.Warnf("request body for state id %s exceeds limit of %d bytes", state.ID, maxBodySize)
				HTTPResponse(w, r, http.StatusRequestEntityTooLarge, "Request body too large")

				return
			}

			HTTPResponse(w, r, http.StatusInternalServerError, err.Error())

			return
		}

		log.Tracef("request: %s %s: %s", r.Method, r.URL.Path, body)

		switch r.Method {
		case "LOCK":
			Lock(w, r, state, body, locker, store)
		case "UNLOCK":
			Unlock(w, r, state, body, locker)
		case http.MethodGet:
			Get(w, r, state, store, kms)
		case http.MethodPost:
			Post(w, r, state, body, locker, store, kms)
		case http.MethodDelete:
			Delete(w, r, state, locker, store)
		default:
			log.Warnf("unknown method %s called", r.Method)
			HTTPResponse(w, r, http.StatusNotImplemented, "Not implemented")

			return
		}
	}
}

func Lock(w http.ResponseWriter, r *http.Request, state *terraform.State, body []byte, locker lock.Locker, store storage.Storage) {
	log.Debugf("try to lock state with id %s", state.ID)

	if err := json.Unmarshal(body, &state.Lock); err != nil {
		log.Errorf("failed to unmarshal lock info: %v", err)
		HTTPResponse(w, r, http.StatusBadRequest, "")
		return
	}

	if ok, err := locker.Lock(state); err != nil {
		log.Errorf("failed to lock state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusInternalServerError, "")
	} else if !ok {
		log.Warnf("state with id %s is already locked by %s", state.ID, state.Lock)

		lockInfo, err := json.Marshal(state.Lock)
		if err != nil {
			log.Errorf("failed to marshal lock info: %v", err)
			HTTPResponse(w, r, http.StatusInternalServerError, "")
			return
		}

		HTTPResponse(w, r, http.StatusLocked, string(lockInfo))
	} else {
		log.Debugf("state with id %s was locked successfully", state.ID)

		// The lock is held, so no write can happen during the migration.
		migrateLegacyState(state, store)

		HTTPResponse(w, r, http.StatusOK, "")
	}
}

func Unlock(w http.ResponseWriter, r *http.Request, state *terraform.State, body []byte, locker lock.Locker) {
	log.Debugf("try to unlock state with id %s", state.ID)

	if len(body) == 0 {
		state.Lock = terraform.LockInfo{}
	} else if err := json.Unmarshal(body, &state.Lock); err != nil {
		log.Errorf("failed to unmarshal lock info: %v", err)
		HTTPResponse(w, r, http.StatusBadRequest, "")
		return
	}

	if ok, err := locker.Unlock(state); err != nil {
		log.Errorf("failed to unlock state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusInternalServerError, "")
	} else if !ok {
		log.Warnf("failed to unlock state with id %s: locks not equal", state.ID)

		lockInfo, err := json.Marshal(state.Lock)
		if err != nil {
			log.Errorf("failed to marshal lock info: %v", err)
			HTTPResponse(w, r, http.StatusInternalServerError, "")
			return
		}

		HTTPResponse(w, r, http.StatusBadRequest, string(lockInfo))
	} else {
		log.Debugf("state with id %s was unlocked successfully", state.ID)
		HTTPResponse(w, r, http.StatusOK, "")
	}
}

func Get(w http.ResponseWriter, r *http.Request, state *terraform.State, store storage.Storage, kms kms.KMS) {
	log.Debugf("get state with id %s", state.ID)

	data, err := readState(state, store)
	if errors.Is(err, storage.ErrStateNotFound) {
		log.Debugf("state with id %s does not exist", state.ID)
		HTTPResponse(w, r, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		log.Warnf("failed to get state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusBadRequest, err.Error())
		return
	}

	if kms != nil && len(data) > 0 {
		data, err = kms.Decrypt(data)
		if err != nil {
			log.Errorf("failed to decrypt state with id %s: %v", state.ID, err)
			HTTPResponse(w, r, http.StatusInternalServerError, "")
			return
		}
	}

	HTTPResponse(w, r, http.StatusOK, string(data))
}

// readState returns the encrypted data of the state. States that weren't written since v0.3.0 are read from their
// legacy ID.
func readState(state *terraform.State, store storage.Storage) ([]byte, error) {
	stored, err := store.GetState(state.ID)
	if errors.Is(err, storage.ErrStateNotFound) && state.LegacyID != "" {
		data, legacyErr := readLegacyState(state, store)
		if !errors.Is(legacyErr, storage.ErrStateNotFound) {
			return data, legacyErr
		}

		// A migration between the two reads moved the state to its new ID. The migration saves the new ID before it
		// deletes the legacy copy, so it's there now.
		stored, err = store.GetState(state.ID)
	}

	if err != nil {
		return nil, err
	}

	s, err := terraform.UnmarshalStoredState(stored.Data, state)
	if err != nil {
		return nil, err
	}

	return s.Data, nil
}

func Post(w http.ResponseWriter, r *http.Request, state *terraform.State, body []byte, locker lock.Locker, store storage.Storage, kms kms.KMS) {
	reqLockID := r.URL.Query().Get("ID")

	lock, err := locker.GetLock(state)
	if err != nil {
		log.Warnf("failed to get lock for state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusBadRequest, "")
		return
	}

	if lock.ID != reqLockID {
		log.Warnf("attempting to write state with wrong lock %s (expected %s)", reqLockID, lock.ID)
		HTTPResponse(w, r, http.StatusBadRequest, "")
		return
	}

	log.Debugf("save state with id %s", state.ID)

	state.Data, err = kms.Encrypt(body)
	if err != nil {
		log.Errorf("failed to encrypt state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusInternalServerError, "")
		return
	}

	err = saveState(state, store)
	if err != nil {
		log.Warnf("failed to save state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// The lock migrates legacy states, this only cleans up if that failed. A failure is logged, the new ID is read first.
	_ = deleteLegacyState(state, store)

	HTTPResponse(w, r, http.StatusOK, "")
}

// saveState stores the encrypted data of the state with its metadata.
func saveState(state *terraform.State, store storage.Storage) error {
	data, err := terraform.MarshalStoredState(state, version.Get(), time.Now())
	if err != nil {
		return fmt.Errorf("encoding state: %w", err)
	}

	return store.SaveState(&terraform.State{ID: state.ID, Data: data})
}

// Delete removes the state. It takes the lock for the duration of the deletion, so that a state
// can't be deleted while another client holds its lock, and releases it afterwards, so that no lock
// is left behind for the deleted state. A client that holds the lock can delete the state by sending
// its lock ID as ?ID=, which also releases its lock.
func Delete(w http.ResponseWriter, r *http.Request, state *terraform.State, locker lock.Locker, store storage.Storage) {
	log.Debugf("delete state with id %s", state.ID)

	reqLockID := r.URL.Query().Get("ID")
	state.Lock = terraform.LockInfo{
		ID:        uuid.New().String(),
		Operation: "OperationTypeDelete",
		Who:       "terraform-backend",
		Created:   time.Now().UTC().Format(time.RFC3339Nano),
		Info:      "deleting state",
	}

	locked, err := locker.Lock(state)
	if err != nil {
		log.Errorf("failed to lock state with id %s for deletion: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusInternalServerError, "")
		return
	}

	// a failed Lock() sets state.Lock to the current lock
	if !locked && (reqLockID == "" || state.Lock.ID != reqLockID) {
		log.Warnf("refusing to delete state with id %s, it is locked by %s", state.ID, state.Lock)

		lockInfo, err := json.Marshal(state.Lock)
		if err != nil {
			log.Errorf("failed to marshal lock info: %v", err)
			HTTPResponse(w, r, http.StatusInternalServerError, "")
			return
		}

		HTTPResponse(w, r, http.StatusLocked, string(lockInfo))
		return
	}

	// The legacy copy first: reads fall back to it while the new ID is missing. If it fails, the state is kept.
	deleteErr := deleteLegacyState(state, store)
	if deleteErr == nil {
		deleteErr = store.DeleteState(state.ID)
		if deleteErr != nil {
			log.Warnf("failed to delete state with id %s: %v", state.ID, deleteErr)
		}
	}

	if unlocked, err := locker.Unlock(state); err != nil || !unlocked {
		log.Errorf("failed to release lock of state with id %s after deletion: unlocked=%t, err=%v", state.ID, unlocked, err)
		HTTPResponse(w, r, http.StatusInternalServerError, "failed to release the lock")
		return
	}

	if deleteErr != nil {
		HTTPResponse(w, r, http.StatusInternalServerError, deleteErr.Error())
		return
	}

	HTTPResponse(w, r, http.StatusOK, "")
}
