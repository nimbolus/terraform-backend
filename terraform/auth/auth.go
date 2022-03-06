package auth

import (
	"fmt"
	"net/http"

	"github.com/nimbolus/terraform-backend/terraform"
	"github.com/nimbolus/terraform-backend/terraform/auth/basic"
	"github.com/spf13/viper"
)

type Authenticator interface {
	GetName() string
	Authenticate(*http.Request, *terraform.State) (bool, error)
}

func GetAuthenticator() (a Authenticator, err error) {
	viper.SetDefault("auth_backend", "basic")
	backend := viper.GetString("auth_backend")

	switch backend {
	case "basic":
		a = basic.NewBasicAuth()
	default:
		err = fmt.Errorf("backend is not implemented")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize auth backend %s: %v", backend, err)
	}
	return
}
