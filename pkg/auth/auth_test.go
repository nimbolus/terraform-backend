package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func setJWTConfig(t *testing.T, issuerURL, vaultAddr string) {
	viper.Set("auth_jwt_oidc_issuer_url", issuerURL)
	viper.Set("vault_addr", vaultAddr)
	t.Cleanup(func() {
		viper.Set("auth_jwt_oidc_issuer_url", "")
		viper.Set("vault_addr", "")
	})
}

func TestJWTIssuerURL(t *testing.T) {
	tests := []struct {
		name      string
		issuerURL string
		vaultAddr string
		want      string
	}{
		{name: "nothing configured", want: ""},
		{name: "explicit issuer", issuerURL: "https://token.actions.githubusercontent.com", want: "https://token.actions.githubusercontent.com"},
		{name: "vault address", vaultAddr: "https://vault.example.com", want: "https://vault.example.com/v1/identity/oidc"},
		{name: "explicit issuer takes precedence", issuerURL: "https://token.actions.githubusercontent.com", vaultAddr: "https://vault.example.com", want: "https://token.actions.githubusercontent.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setJWTConfig(t, tt.issuerURL, tt.vaultAddr)
			require.Equal(t, tt.want, jwtIssuerURL())
		})
	}
}

func TestAuthenticateJWTDisabled(t *testing.T) {
	setJWTConfig(t, "", "")

	req := httptest.NewRequest("GET", "/state/p/n", nil)
	req.SetBasicAuth("jwt", "token")

	ok, err := Authenticate(req, &terraform.State{Project: "p", Name: "n"})
	require.False(t, ok)
	require.EqualError(t, err, "jwt auth is not enabled")
}

func TestCheckConfig(t *testing.T) {
	t.Cleanup(func() { viper.Set("auth_jwt_validation_expression", "") })

	setJWTConfig(t, "", "")
	viper.Set("auth_jwt_validation_expression", "invalid ==")
	require.NoError(t, CheckConfig(), "jwt auth is disabled")

	setJWTConfig(t, "https://token.actions.githubusercontent.com", "")
	require.ErrorContains(t, CheckConfig(), "invalid validation expression")

	viper.Set("auth_jwt_validation_expression", `claims.repository_owner_id == "123"`)
	require.NoError(t, CheckConfig())

	first, err := jwtAuthenticator(jwtIssuerURL())
	require.NoError(t, err)
	second, err := jwtAuthenticator(jwtIssuerURL())
	require.NoError(t, err)
	require.Same(t, first, second, "authenticator is reused while the config is unchanged")

	viper.Set("auth_jwt_validation_expression", `claims.repository_owner_id == "456"`)
	third, err := jwtAuthenticator(jwtIssuerURL())
	require.NoError(t, err)
	require.NotSame(t, first, third, "authenticator is recreated after a config change")
}
