package auth

import (
	"fmt"
	"net/http"

	"github.com/spf13/viper"

	"github.com/nimbolus/terraform-backend/pkg/auth/basic"
	"github.com/nimbolus/terraform-backend/pkg/auth/jwt"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

type Authenticator interface {
	GetName() string
	Authenticate(secret string, s *terraform.State) (bool, error)
}

func Authenticate(req *http.Request, s *terraform.State) (ok bool, err error) {
	backend, secret, ok := req.BasicAuth()
	if !ok {
		return false, fmt.Errorf("no basic auth header found")
	}

	var authenticator Authenticator
	switch backend {
	case basic.Name:
		viper.SetDefault("auth_basic_enabled", true)
		if !viper.GetBool("auth_basic_enabled") {
			return false, fmt.Errorf("basic auth is not enabled")
		}
		authenticator = basic.NewBasicAuth()
	case jwt.Name:
		issuerURL := viper.GetString("auth_jwt_oidc_issuer_url")
		viper.SetDefault("auth_jwt_oidc_client_id", "terraform-backend")
		clientID := viper.GetString("auth_jwt_oidc_client_id")
		if addr := viper.GetString("vault_addr"); issuerURL != "" && addr != "" {
			issuerURL = fmt.Sprintf("%s/v1/identity/oidc", addr)
		} else {
			return false, fmt.Errorf("jwt auth is not enabled")
		}
		authenticator = jwt.NewJWTAuth(issuerURL, clientID)
	default:
		err = fmt.Errorf("backend is not implemented")
	}
	if err != nil {
		return false, fmt.Errorf("failed to initialize auth backend %s: %v", backend, err)
	}

	return authenticator.Authenticate(secret, s)
}
