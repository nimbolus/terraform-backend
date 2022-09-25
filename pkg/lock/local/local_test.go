package local

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/lock/util"
)

func init() {
	viper.AutomaticEnv()
}

func TestLock(t *testing.T) {
	l := NewLocalLock()

	util.LockTest(t, l)
}
