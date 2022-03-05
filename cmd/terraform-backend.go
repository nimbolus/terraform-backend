package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nimbolus/terraform-backend/kms"
	"github.com/nimbolus/terraform-backend/terraform"
	"github.com/nimbolus/terraform-backend/terraform/locker"
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

func stateHandler(stateStore store.Store, locker locker.Locker, kms kms.KMS) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			httpResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		state := &terraform.State{}
		state.ID = getStateID(req)

		log.Infof("%s %s", req.Method, req.URL.Path)
		log.Trace("request: %s %s: %s", req.Method, req.URL.Path, body)

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
			state, err = stateStore.GetState(state.ID)
			if err != nil {
				log.Warnf("failed to get state with id %s: %v", state.ID, err)
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
	viper.SetDefault("log_level", "info")
	viper.SetDefault("listen_addr", ":8080")
	viper.SetDefault("store_backend", "file")
	viper.SetDefault("lock_backend", "local")
	viper.SetDefault("kms_backend", "local")
	viper.AutomaticEnv()

	level, err := log.ParseLevel(viper.GetString("log_level"))
	if err != nil {
		log.Fatalf("failed to set log level: %v", err)
	}
	log.Infof("set log level to %s", level.String())
	log.SetLevel(level)

	// var stateStore terraform.Store
	// switch viper.GetString("store_backend") {
	// case "file":
	// 	stateStore, err = filestore.NewFileStore("./example/states")
	// default:
	// 	log.Fatalf("failed to initialize lock backend: %s is unknown", viper.GetString("store_backend"))
	// }

	// var locker locker.Locker
	// switch viper.GetString("lock_backend") {
	// case "redis":
	// 	log.Println("initializing Redis lock")
	// 	locker = redislock.NewRedisLock()
	// case "local":
	// 	log.Println("initializing local lock")
	// 	locker = locallock.NewLocalLock()
	// default:
	// 	log.Fatalf("failed to initialize lock backend: %s is unknown", viper.GetString("lock_backend"))
	// }

	// var kms kms.KMS
	// switch viper.GetString("kms_backend") {
	// case "transit":
	// 	log.Println("initializing Vault Transit KMS")
	// 	kms, err = vaulttransit.NewVaultTransit(viper.GetString("kms_transit_engine"), viper.GetString("kms_transit_key"))
	// case "local":
	// 	var key string
	// 	if keyPath := viper.GetString("kms_vault_key_path"); keyPath != "" {
	// 		log.Infof("initializing local KMS with key from Vault K/V engine")
	// 		vaultClient, err := vaultclient.NewVaultClient()
	// 		if err != nil {
	// 			log.Fatalf("failed to setup Vault client for local KMS: %v", err)
	// 		}

	// 		key, err = vaultclient.GetKvValue(vaultClient, keyPath, "key")
	// 		if err != nil {
	// 			log.Fatalf("failed to get key for local KMS from Vault: %v", err)
	// 		}
	// 	} else {
	// 		log.Infof("initializing local KMS with key from environment")
	// 		if key = viper.GetString("kms_key"); key == "" {
	// 			key, _ = simplekms.GenerateKey()
	// 			log.Printf("No key defined. Set KMS_KEY to this generated key: %s", key)
	// 			return
	// 		}
	// 	}
	// 	kms, err = simplekms.NewSimpleKMS(key)
	// default:
	// 	log.Fatalf("failed to initialize KMS backend %s: %s is unknown", viper.GetString("kms_backend"), viper.GetString("kms_backend"))
	// }

	stateStore, err := store.GetStore()
	if err != nil {
		log.Fatalf("failed to initialize store backend: %v", err)
	}
	log.Infof("initialized %s store backend", stateStore.GetName())

	locker, err := locker.GetLocker()
	if err != nil {
		log.Fatalf("failed to initialize lock backend: %v", err)
	}
	log.Infof("initialized %s lock backend", locker.GetName())

	kms, err := kms.GetKMS()
	if err != nil {
		log.Fatalf("failed to initialize KMS backend: %v", err)
	}
	log.Infof("initialized %s KMS backend", kms.GetName())

	addr := viper.GetString("listen_addr")
	log.Printf("listening on %s", addr)
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/state/{project}/{id}", stateHandler(stateStore, locker, kms))
	log.Fatalf("failed to listen on %s: %v", addr, http.ListenAndServe(addr, r))
}
