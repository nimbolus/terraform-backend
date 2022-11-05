package main

import (
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/server"
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
			err = http.ListenAndServe(metricsAddr, metricsRouter)
			if err != nil {
				log.Fatalf("failed to listen on %s for metrics: %v", metricsAddr, err)
			}
		}()
	} else {
		log.Printf("exposing metrics on default endpoint on %s", addr)
		r.HandleFunc("/metrics", server.MetricsHandler)
	}

	server.RecordMetrics(store, locker, kms)

	if tlsKey != "" && tlsCert != "" {
		log.Printf("listening on %s with tls", addr)
		err = http.ListenAndServeTLS(addr, tlsCert, tlsKey, r)
	} else {
		log.Printf("listening on %s", addr)
		err = http.ListenAndServe(addr, r)
	}
	log.Fatalf("failed to listen on %s: %v", addr, err)
}
