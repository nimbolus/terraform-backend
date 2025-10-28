//go:build integration || local
// +build integration local

package local

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/kms/util"
)

func init() {
	viper.AutomaticEnv()
}

func TestKMS(t *testing.T) {
	key := "x8DiIkAKRQT7cF55NQLkAZk637W3bGVOUjGeMX5ZGXY="
	k, err := NewKMS(key)
	if err != nil {
		t.Error(err)
	}

	util.KMSTest(t, k)
}
