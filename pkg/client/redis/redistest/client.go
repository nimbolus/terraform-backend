package redistest

import (
	"os"
	"testing"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/client/redis"
)

func NewPoolIfIntegrationTest(t testing.TB) *redigo.Pool {
	t.Helper()

	if v := os.Getenv("INTEGRATION_TEST"); v == "" {
		t.Skip("env var INTEGRATION_TEST not set")
	}

	t.Setenv("REDIS_ADDR", "localhost:6379")

	viper.AutomaticEnv()

	return redis.NewPool()
}
