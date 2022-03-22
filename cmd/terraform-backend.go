package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nimbolus/terraform-backend/kms"
	"github.com/nimbolus/terraform-backend/terraform"
	"github.com/nimbolus/terraform-backend/terraform/auth"
	"github.com/nimbolus/terraform-backend/terraform/lock"
	"github.com/nimbolus/terraform-backend/terraform/storage"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func httpResponse(w http.ResponseWriter, code int, body string) {
	log.Tracef("response: %d %s", code, body)
	w.WriteHeader(code)
	fmt.Fprint(w, body)
}

func stateHandler(store storage.Storage, locker lock.Locker, kms kms.KMS) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			httpResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		vars := mux.Vars(req)
		state := &terraform.State{
			ID: terraform.GetStateID(vars["project"], vars["id"]),
		}

		log.Infof("%s %s", req.Method, req.URL.Path)
		log.Trace("request: %s %s: %s", req.Method, req.URL.Path, body)

		if ok, err := auth.Authenticate(req, state); err != nil {
			log.Warnf("failed process authentication for state id %s: %v", state.ID, err)
			httpResponse(w, http.StatusForbidden, err.Error())
			return
		} else if !ok {
			log.Warnf("failed to authenticate request for state id %s", state.ID)
			httpResponse(w, http.StatusForbidden, "Permission denied")
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
			state, err = store.GetState(state.ID)
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

			err := store.SaveState(state)
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

func healthHandler(w http.ResponseWriter, req *http.Request) {
	log.Infof("%s %s", req.Method, req.URL.Path)
	httpResponse(w, http.StatusOK, "")
}

func main() {
	viper.AutomaticEnv()
	viper.SetDefault("log_level", "info")

	level, err := log.ParseLevel(viper.GetString("log_level"))
	if err != nil {
		log.Fatalf("failed to set log level: %v", err)
	}
	log.Infof("set log level to %s", level.String())
	log.SetLevel(level)

	store, err := storage.GetStorage()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Infof("initialized %s storage backend", store.GetName())

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

	viper.SetDefault("listen_addr", ":8080")
	addr := viper.GetString("listen_addr")
	tlsKey := viper.GetString("tls_key")
	tlsCert := viper.GetString("tls_cert")

	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/state/{project}/{id}", stateHandler(store, locker, kms))
	r.HandleFunc("/health", healthHandler)

	if tlsKey != "" && tlsCert != "" {
		log.Printf("listening on %s with tls", addr)
		err = http.ListenAndServeTLS(addr, tlsCert, tlsKey, r)
	} else {
		log.Printf("listening on %s", addr)
		err = http.ListenAndServe(addr, r)
	}
	log.Fatalf("failed to listen on %s: %v", addr, err)
}
