package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/server"
	"github.com/nimbolus/terraform-backend/pkg/version"
)

func main() {
	viper.AutomaticEnv()
	viper.SetDefault("log_level", "info")

	level, err := log.ParseLevel(viper.GetString("log_level"))
	if err != nil {
		log.Fatalf("failed to set log level: %v", err)
	}
	log.Infof("set log level to %s", level.String())
	log.SetLevel(level)
	log.Infof("terraform-backend %s", version.Get())

	if len(os.Args) > 1 {
		os.Exit(runCommand(os.Args[1:]))
	}

	store, err := server.GetStorage()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Infof("initialized %s storage backend", store.GetName())

	locker, err := server.GetLocker()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Infof("initialized %s lock backend", locker.GetName())

	kms, err := server.GetKMS()
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Infof("initialized %s KMS backend", kms.GetName())

	viper.SetDefault("listen_addr", ":8080")
	addr := viper.GetString("listen_addr")
	tlsKey := viper.GetString("tls_key")
	tlsCert := viper.GetString("tls_cert")

	viper.SetDefault("metrics_listen_addr", ":8081")
	metricsAddr := viper.GetString("metrics_listen_addr")

	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/state/{project}/{name}", server.StateHandler(store, locker, kms))
	r.HandleFunc("/health", server.HealthHandler)

	if viper.GetString("listen_addr") != viper.GetString("metrics_listen_addr") {
		metricsRouter := mux.NewRouter().StrictSlash(true)
		metricsRouter.HandleFunc("/metrics", server.MetricsHandler)

		go func() {
			log.Printf("listening on %s for metrics", metricsAddr)
			err = newHTTPServer(metricsAddr, metricsRouter).ListenAndServe()
			if err != nil {
				log.Fatalf("failed to listen on %s for metrics: %v", metricsAddr, err)
			}
		}()
	} else {
		log.Printf("exposing metrics on default endpoint on %s", addr)
		r.HandleFunc("/metrics", server.MetricsHandler)
	}

	server.RecordMetrics(store, locker, kms)

	srv := newHTTPServer(addr, r)

	if tlsKey != "" && tlsCert != "" {
		log.Printf("listening on %s with tls", addr)
		err = srv.ListenAndServeTLS(tlsCert, tlsKey)
	} else {
		log.Printf("listening on %s", addr)
		err = srv.ListenAndServe()
	}
	log.Fatalf("failed to listen on %s: %v", addr, err)
}

// newHTTPServer returns a server with timeouts, so that slow or idle clients can't hold connections open indefinitely.
// The read and write timeouts are generous to allow transferring large state files.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
}

// runCommand runs a command instead of the server and returns its exit code. Errors exit with 2.
func runCommand(args []string) int {
	switch args[0] {
	case "check-migration":
		return checkMigration()
	default:
		log.Errorf("unknown command %q, the only command is check-migration", args[0])
		return 2
	}
}

// checkMigration lists the states that are still stored with a legacy ID. It exits with 1 if there are any.
// TODO: remove in 1.0.0, together with the migration of legacy states.
func checkMigration() int {
	store, err := server.GetStorage()
	if err != nil {
		log.Error(err.Error())
		return 2
	}

	legacy, err := server.CheckMigration(store, os.Stdout)
	if err != nil {
		log.Error(err.Error())
		return 2
	}

	if legacy > 0 {
		return 1
	}

	return 0
}
