package server

import (
	"math"
	"net/http"
	"time"

	"github.com/nimbolus/terraform-backend/pkg/kms"
	"github.com/nimbolus/terraform-backend/pkg/lock"
	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const Namespace = "tfbackend"

var (
	backendInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "backend_info",
		Help:      "Info about the included backends (1 = enabled, 0 = disabled)",
	}, []string{"backend_type", "backend_name"})
	storedObjects = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "stored_objects",
		Help:      "The total number of stored objects (if supported by the storage backend)",
	})
	requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "request_count",
		Help:      "The total number of requests",
	}, []string{"method", "path", "code"})
)

func RecordMetrics(store storage.Storage, locker lock.Locker, k kms.KMS) {
	go func() {
		for {
			backendInfo.WithLabelValues("storage", store.GetName()).Set(1)
			backendInfo.WithLabelValues("lock", locker.GetName()).Set(1)
			backendInfo.WithLabelValues("kms", k.GetName()).Set(1)

			if c, ok := store.(storage.Countable); ok {
				count, err := c.CountStoredObjects()
				if err != nil {
					logrus.WithError(err).WithField("component", "metrics").Error("counting stored objects")
				}

				storedObjects.Set(float64(count))
			} else {
				// if the storage backend doesn't implement Countable, set NaN
				storedObjects.Set(math.NaN())
			}

			time.Sleep(5 * time.Second)
		}
	}()
}

func MetricsHandler(w http.ResponseWriter, req *http.Request) {
	promhttp.Handler().ServeHTTP(w, req)
}
