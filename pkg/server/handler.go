package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/nimbolus/terraform-backend/pkg/auth"
	"github.com/nimbolus/terraform-backend/pkg/kms"
	"github.com/nimbolus/terraform-backend/pkg/lock"
	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func HTTPResponse(w http.ResponseWriter, r *http.Request, code int, body string) {
	log.Tracef("response: %d %s", code, body)
	w.WriteHeader(code)
	fmt.Fprint(w, body)

	requestCount.With(prometheus.Labels{
		"method": r.Method,
		"path":   r.URL.Path,
		"code":   strconv.Itoa(code),
	}).Inc()
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	log.Debugf("%s %s", r.Method, r.URL.Path)
	HTTPResponse(w, r, http.StatusOK, "")
}

func StateHandler(store storage.Storage, locker lock.Locker, kms kms.KMS) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			HTTPResponse(w, r, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(r)
		state := &terraform.State{
			ID:      terraform.GetStateID(vars["project"], vars["name"]),
			Project: vars["project"],
			Name:    vars["name"],
		}

		log.Infof("%s %s", r.Method, r.URL.Path)
		log.Tracef("request: %s %s: %s", r.Method, r.URL.Path, body)

		if ok, err := auth.Authenticate(r, state); err != nil {
			log.Warnf("failed process authentication for state id %s: %v", state.ID, err)
			HTTPResponse(w, r, http.StatusForbidden, err.Error())

			return
		} else if !ok {
			log.Warnf("failed to authenticate request for state id %s", state.ID)
			HTTPResponse(w, r, http.StatusForbidden, "Permission denied")

			return
		}

		switch r.Method {
		case "LOCK":
			Lock(w, r, state, body, locker)
		case "UNLOCK":
			Unlock(w, r, state, body, locker)
		case http.MethodGet:
			Get(w, r, state, store, kms)
		case http.MethodPost:
			Post(w, r, state, body, locker, store, kms)
		case http.MethodDelete:
			Delete(w, r, state, store)
		default:
			log.Warnf("unknown method %s called", r.Method)
			HTTPResponse(w, r, http.StatusNotImplemented, "Not implemented")

			return
		}
	}
}

func Lock(w http.ResponseWriter, r *http.Request, state *terraform.State, body []byte, locker lock.Locker) {
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
	stateID := state.ID
	state, err := store.GetState(state.ID)
	if errors.Is(err, storage.ErrStateNotFound) {
		log.Debugf("state with id %s does not exist", stateID)
		HTTPResponse(w, r, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		log.Warnf("failed to get state with id %s: %v", stateID, err)
		HTTPResponse(w, r, http.StatusBadRequest, err.Error())
		return
	}

	if kms != nil && len(state.Data) > 0 {
		state.Data, err = kms.Decrypt(state.Data)
		if err != nil {
			log.Errorf("failed to decrypt state with id %s: %v", state.ID, err)
			HTTPResponse(w, r, http.StatusInternalServerError, "")
			return
		}
	}

	HTTPResponse(w, r, http.StatusOK, string(state.Data))
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

	data, err := kms.Encrypt(body)
	if err != nil {
		log.Errorf("failed to encrypt state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusInternalServerError, "")
		return
	}

	state.Data = data

	err = store.SaveState(state)
	if err != nil {
		log.Warnf("failed to save state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusBadRequest, err.Error())
		return
	}

	HTTPResponse(w, r, http.StatusOK, "")
}

func Delete(w http.ResponseWriter, r *http.Request, state *terraform.State, store storage.Storage) {
	log.Debugf("delete state with id %s", state.ID)

	err := store.DeleteState(state.ID)
	if err != nil {
		log.Warnf("failed to delete state with id %s: %v", state.ID, err)
		HTTPResponse(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	HTTPResponse(w, r, http.StatusOK, "")
}
