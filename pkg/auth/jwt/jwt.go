package jwt

import (
	"context"
	"fmt"
	"slices"

	"cel.dev/cel-go/cel"
	"github.com/coreos/go-oidc/v3/oidc"
	log "github.com/sirupsen/logrus"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const (
	Name = "jwt"

	// allStates grants access to all states of a project.
	allStates = "*"
)

// Config configures the JWT authentication. All expressions are CEL expressions
// over the token claims.
type Config struct {
	IssuerURL string
	ClientID  string
	// ProjectExpression returns the project or a list of projects the token
	// grants access to. Defaults to the project of the "terraform-backend" claim.
	ProjectExpression string
	// StateExpression returns the state or a list of states the token grants
	// access to in these projects, "*" being all states. Defaults to the state of
	// the "terraform-backend" claim, or to all states if ProjectExpression is set.
	StateExpression string
	// ValidationExpression must return true for a token to be accepted at all.
	ValidationExpression string
}

type defaultClaim struct {
	TerraformBackend struct {
		Project string `json:"project"`
		State   string `json:"state"`
	} `json:"terraform-backend"`
}

type JWTAuth struct {
	issuerURL            string
	clientID             string
	projectExpression    cel.Program
	stateExpression      cel.Program
	validationExpression cel.Program
}

// NewJWTAuth creates a JWT authenticator and compiles the expressions of the config.
func NewJWTAuth(cfg Config) (*JWTAuth, error) {
	j := &JWTAuth{
		issuerURL: cfg.IssuerURL,
		clientID:  cfg.ClientID,
	}

	// Lists built from claims have the type list(dyn), their elements are checked on evaluation.
	stringsTypes := []*cel.Type{cel.StringType, cel.ListType(cel.StringType), cel.ListType(cel.DynType)}

	var err error
	if cfg.ProjectExpression != "" {
		j.projectExpression, err = compileExpression(cfg.ProjectExpression, stringsTypes...)
		if err != nil {
			return nil, fmt.Errorf("invalid project expression: %w", err)
		}
	}
	if cfg.StateExpression != "" {
		j.stateExpression, err = compileExpression(cfg.StateExpression, stringsTypes...)
		if err != nil {
			return nil, fmt.Errorf("invalid state expression: %w", err)
		}
	}
	if cfg.ValidationExpression != "" {
		j.validationExpression, err = compileExpression(cfg.ValidationExpression, cel.BoolType)
		if err != nil {
			return nil, fmt.Errorf("invalid validation expression: %w", err)
		}
	}

	return j, nil
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

	// Without expressions, only the "terraform-backend" claim is decoded, so other claims can't affect a token.
	var claims map[string]any
	if j.projectExpression != nil || j.stateExpression != nil || j.validationExpression != nil {
		claims, err = decodeClaims(token)
		if err != nil {
			return false, err
		}
	}

	if j.validationExpression != nil {
		ok, err := evalBool(j.validationExpression, claims)
		if err != nil {
			log.Debugf("jwt: failed to evaluate validation expression: %v", err)
			return false, nil
		}

		if !ok {
			log.Debugf("jwt: token doesn't fulfill the validation expression")
			return false, nil
		}
	}

	var backendClaim defaultClaim
	if j.projectExpression == nil {
		if err := token.Claims(&backendClaim); err != nil {
			return false, err
		}
	}

	projects := []string{backendClaim.TerraformBackend.Project}
	if j.projectExpression != nil {
		projects, err = evalStrings(j.projectExpression, claims)
		if err != nil {
			log.Debugf("jwt: failed to evaluate project expression: %v", err)
			return false, nil
		}
	}
	if s.Project == "" || !slices.Contains(projects, s.Project) {
		log.Debugf("jwt: projects %q of token don't include project %q", projects, s.Project)
		return false, nil
	}

	states := []string{backendClaim.TerraformBackend.State}
	if j.stateExpression != nil {
		states, err = evalStrings(j.stateExpression, claims)
		if err != nil {
			log.Debugf("jwt: failed to evaluate state expression: %v", err)
			return false, nil
		}
	} else if j.projectExpression != nil {
		states = []string{allStates}
	}
	if !slices.Contains(states, allStates) && (s.Name == "" || !slices.Contains(states, s.Name)) {
		log.Debugf("jwt: states %q of token don't include state %q", states, s.Name)
		return false, nil
	}

	return true, nil
}
