package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nimbolus/terraform-backend/kms"
	"github.com/nimbolus/terraform-backend/terraform"
	"github.com/nimbolus/terraform-backend/terraform/auth"
	"github.com/nimbolus/terraform-backend/terraform/lock"
	"github.com/nimbolus/terraform-backend/terraform/store"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func httpResponse(w http.ResponseWriter, code int, body string) {
	log.Tracef("response: %d %s", code, body)
	w.WriteHeader(code)
	fmt.Fprint(w, body)
}

func getStateID(req *http.Request) string {
	vars := mux.Vars(req)
	id := fmt.Sprintf("%s-%s", vars["id"], vars["project"])
	hash := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%x", hash[:])
}

func stateHandler(stateStore store.Store, locker lock.Locker, kms kms.KMS, authenticator auth.Authenticator) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			httpResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		state := &terraform.State{
			ID: getStateID(req),
		}

		log.Infof("%s %s", req.Method, req.URL.Path)
		log.Trace("request: %s %s: %s", req.Method, req.URL.Path, body)

		if ok, err := authenticator.Authenticate(req, state); err != nil {
			log.Warnf("failed to evaluate request authentication for state id %s", state.ID)
			httpResponse(w, http.StatusBadRequest, "Authentication missing")
			return
		} else if !ok {
			log.Warnf("failed to authenticate request for state id %s", state.ID)
			httpResponse(w, http.StatusBadRequest, "Permission denied")
			return
		}

		switch req.Method {
		case "LOCK":
			log.Debugf("try to lock state with id %s", state.ID)
			state.Lock = body

			if ok, err := locker.Lock(state); err != nil {
				log.Errorf("failed to lock state with id %s: %v", state.ID, err)
				httpResponse(w, http.StatusInternalServerError, "")
			} else if !ok {
				log.Warnf("state with id %s is already locked by %s", state.ID, state.Lock)
				httpResponse(w, http.StatusLocked, string(state.Lock))
			} else {
				log.Debugf("state with id %s was locked successfully", state.ID)
				httpResponse(w, http.StatusOK, "")
			}
			return
		case "UNLOCK":
			log.Debugf("try to unlock state with id %s", state.ID)
			state.Lock = body

			if ok, err := locker.Unlock(state); err != nil {
				log.Errorf("failed to unlock state with id %s: %v", state.ID, err)
				httpResponse(w, http.StatusInternalServerError, "")
			} else if !ok {
				log.Warnf("failed to unlock state with id %s: %v", state.ID, err)
				httpResponse(w, http.StatusBadRequest, string(state.Lock))
			} else {
				log.Debugf("state with id %s was unlocked successfully", state.ID)
				httpResponse(w, http.StatusOK, "")
			}
			return
		case http.MethodGet:
			log.Debugf("get state with id %s", state.ID)
			stateID := state.ID
			state, err = stateStore.GetState(state.ID)
			if err != nil {
				log.Warnf("failed to get state with id %s: %v", stateID, err)
				httpResponse(w, http.StatusBadRequest, err.Error())
				return
			}

			if kms != nil && len(state.Data) > 0 {
				state.Data, err = kms.Decrypt(state.Data)
				if err != nil {
					log.Errorf("failed to decrypt state with id %s: %v", state.ID, err)
					httpResponse(w, http.StatusInternalServerError, "")
					return
				}
			}

			httpResponse(w, http.StatusOK, string(state.Data))
			return
		case http.MethodPost:
			log.Debugf("save state with id %s", state.ID)

			state.Data, err = kms.Encrypt(body)
			if err != nil {
				log.Errorf("failed to encrypt state with id %s: %v", state.ID, err)
				httpResponse(w, http.StatusInternalServerError, "")
				return
			}

			err := stateStore.SaveState(state)
			if err != nil {
				log.Warnf("failed to save state with id %s: %v", state.ID, err)
				httpResponse(w, http.StatusBadRequest, err.Error())
				return
			}

			httpResponse(w, http.StatusOK, "")
			return
		case http.MethodDelete:
			log.Debugf("delete state with id %s", state.ID)
			httpResponse(w, http.StatusNotImplemented, "Delete state is not implemented")
			return
		default:
			log.Warnf("unknown method %s called", req.Method)
			httpResponse(w, http.StatusNotImplemented, "Not implemented")
			return
		}
	}
}

func main() {
	viper.AutomaticEnv()
	viper.SetDefault("log_level", "info")
	viper.SetDefault("listen_addr", ":8080")

	level, err := log.ParseLevel(viper.GetString("log_level"))
	if err != nil {
		log.Fatalf("failed to set log level: %v", err)
	}
	log.Infof("set log level to %s", level.String())
	log.SetLevel(level)

	stateStore, err := store.GetStore()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Infof("initialized %s store backend", stateStore.GetName())

	locker, err := lock.GetLocker()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Infof("initialized %s lock backend", locker.GetName())

	kms, err := kms.GetKMS()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Infof("initialized %s KMS backend", kms.GetName())

	authenticator, err := auth.GetAuthenticator()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Infof("initialized %s auth backend", authenticator.GetName())

	addr := viper.GetString("listen_addr")
	log.Printf("listening on %s", addr)
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/state/{project}/{id}", stateHandler(stateStore, locker, kms, authenticator))
	log.Fatalf("failed to listen on %s: %v", addr, http.ListenAndServe(addr, r))
}
