package jwt

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "jwt"

type JWTAuth struct {
	issuerURL string
}

func NewJWTAuth(issuerURL string) *JWTAuth {
	return &JWTAuth{
		issuerURL: issuerURL,
	}
}

func (j *JWTAuth) GetName() string {
	return Name
}

func (j *JWTAuth) Authenticate(secret string, s *terraform.State) (bool, error) {
	provider, err := oidc.NewProvider(context.Background(), j.issuerURL)
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

	if s.Project == claims.TerraformBackend.Project && claims.TerraformBackend.State == "*" {
		return true, nil
	} else if s.Project == claims.TerraformBackend.Project && s.Name == claims.TerraformBackend.State {
		return true, nil
	}

	return false, nil
}
