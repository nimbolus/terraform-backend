package jwt

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const Name = "jwt"

type JWTAuth struct {
	issuerURL    string
	clientID     string
	projectClaim string
}

// NewJWTAuth creates a JWT authenticator. If projectClaim is set, access is
// granted to all states of the project given by the value of this top-level
// string claim instead of using the "terraform-backend" claim.
func NewJWTAuth(issuerURL string, clientID string, projectClaim string) *JWTAuth {
	return &JWTAuth{
		issuerURL:    issuerURL,
		clientID:     clientID,
		projectClaim: projectClaim,
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
		ClientID: j.clientID,
	})

	token, err := verifier.Verify(context.Background(), secret)
	if err != nil {
		return false, err
	}

	if j.projectClaim != "" {
		var claims map[string]any
		if err := token.Claims(&claims); err != nil {
			return false, err
		}

		project, _ := claims[j.projectClaim].(string)
		return project != "" && project == s.Project, nil
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
