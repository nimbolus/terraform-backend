package jwt

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/nimbolus/terraform-backend/terraform"
)

type JWTAuth struct {
	issuerURL string
}

func NewJWTAuth(issuerURL string) *JWTAuth {
	return &JWTAuth{
		issuerURL: issuerURL,
	}
}

func (l *JWTAuth) GetName() string {
	return "jwt"
}

func (b *JWTAuth) Authenticate(secret string, s *terraform.State) (bool, error) {
	provider, err := oidc.NewProvider(context.Background(), b.issuerURL)
	if err != nil {
		return false, err
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	token, err := verifier.Verify(context.Background(), secret)
	if err != nil {
		return false, err
	}

	var claims struct {
		TerraformBackend struct {
			Project string `json:"project"`
			State   string `json:"state"`
		} `json:"terraform-backend"`
	}
	if err := token.Claims(&claims); err != nil {
		return false, err
	}

	tokenID := terraform.GetStateID(claims.TerraformBackend.Project, claims.TerraformBackend.State)
	if s.ID == tokenID {
		return true, nil
	}

	return false, nil
}
