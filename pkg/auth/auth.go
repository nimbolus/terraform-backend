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
		issuerURL := jwtIssuerURL()
		if issuerURL == "" {
			return false, fmt.Errorf("jwt auth is not enabled")
		}
		viper.SetDefault("auth_jwt_oidc_client_id", "terraform-backend")
		clientID := viper.GetString("auth_jwt_oidc_client_id")
		authenticator = jwt.NewJWTAuth(issuerURL, clientID)
	default:
		err = fmt.Errorf("backend is not implemented")
	}
	if err != nil {
		return false, fmt.Errorf("failed to initialize auth backend %s: %v", backend, err)
	}

	return authenticator.Authenticate(secret, s)
}

// jwtIssuerURL returns the configured OIDC issuer URL, falling back to the
// Vault identity issuer if only VAULT_ADDR is set. It is empty if JWT auth is disabled.
func jwtIssuerURL() string {
	if issuerURL := viper.GetString("auth_jwt_oidc_issuer_url"); issuerURL != "" {
		return issuerURL
	}
	if addr := viper.GetString("vault_addr"); addr != "" {
		return fmt.Sprintf("%s/v1/identity/oidc", addr)
	}
	return ""
}
