package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func TestAuthenticateJWTIssuer(t *testing.T) {
	tests := []struct {
		name       string
		issuer     string
		vaultAddr  string
		wantEnable bool
	}{
		{name: "nothing configured"},
		{name: "explicit issuer", issuer: "http://127.0.0.1:1", wantEnable: true},
		{name: "vault address", vaultAddr: "http://127.0.0.1:1", wantEnable: true},
		{name: "explicit issuer and vault address", issuer: "http://127.0.0.1:1", vaultAddr: "http://127.0.0.1:1", wantEnable: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("auth_jwt_oidc_issuer_url", tt.issuer)
			viper.Set("vault_addr", tt.vaultAddr)
			t.Cleanup(func() {
				viper.Set("auth_jwt_oidc_issuer_url", "")
				viper.Set("vault_addr", "")
			})

			req := httptest.NewRequest("GET", "/state/p/n", nil)
			req.SetBasicAuth("jwt", "token")

			// the issuer is unreachable, so an enabled JWT auth fails while contacting it
			ok, err := Authenticate(req, &terraform.State{Project: "p", Name: "n"})
			require.False(t, ok)
			require.Error(t, err)
			if tt.wantEnable {
				require.NotContains(t, err.Error(), "not enabled")
			} else {
				require.Contains(t, err.Error(), "not enabled")
			}
		})
	}
}
