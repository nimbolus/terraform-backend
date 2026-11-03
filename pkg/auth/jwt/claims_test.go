package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const testAudience = "terraform-backend"

// fakeIssuer is a minimal OIDC provider which signs RS256 tokens.
type fakeIssuer struct {
	key *rsa.PrivateKey
	srv *httptest.Server
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	f := &fakeIssuer{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                f.srv.URL,
			"jwks_uri":                              f.srv.URL + "/.well-known/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/.well-known/keys", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]any{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "test",
			"n": b64(key.N.Bytes()),
			"e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	return f
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// token signs a token with the given extra claims on top of valid default claims.
func (f *fakeIssuer) token(t *testing.T, extra map[string]any) string {
	t.Helper()

	claims := map[string]any{
		"iss": f.srv.URL,
		"aud": testAudience,
		"sub": "repo:owner-1/repo-42:ref:refs/heads/main",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	for k, v := range extra {
		claims[k] = v
	}

	header, err := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": "test"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	signingInput := b64(header) + "." + b64(payload)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	require.NoError(t, err)

	return signingInput + "." + b64(sig)
}

// auth creates an authenticator for tokens of this issuer.
func (f *fakeIssuer) auth(t *testing.T, cfg Config) *JWTAuth {
	t.Helper()

	cfg.IssuerURL = f.srv.URL
	cfg.ClientID = testAudience
	a, err := NewJWTAuth(cfg)
	require.NoError(t, err)

	return a
}

func state(project, name string) *terraform.State {
	return &terraform.State{ID: terraform.GetStateID(project, name), Project: project, Name: name}
}

func TestDefaultClaim(t *testing.T) {
	f := newFakeIssuer(t)
	a := f.auth(t, Config{})

	token := f.token(t, map[string]any{"terraform-backend": map[string]any{"project": "p1", "state": "prod"}})
	wildcard := f.token(t, map[string]any{"terraform-backend": map[string]any{"project": "p1", "state": "*"}})

	ok, err := a.Authenticate(token, state("p1", "prod"))
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = a.Authenticate(token, state("p1", "dev"))
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = a.Authenticate(token, state("p2", "prod"))
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = a.Authenticate(wildcard, state("p1", "dev"))
	require.NoError(t, err)
	require.True(t, ok)

	// a token without the claim must not grant access, even if a project expression would match
	ok, err = a.Authenticate(f.token(t, map[string]any{"repository_id": "42"}), state("42", "prod"))
	require.NoError(t, err)
	require.False(t, ok)
}

func TestInvalidTokens(t *testing.T) {
	f := newFakeIssuer(t)
	a := f.auth(t, Config{ProjectExpression: `claims.repository_id`})

	t.Run("wrong audience", func(t *testing.T) {
		ok, err := a.Authenticate(f.token(t, map[string]any{"repository_id": "42", "aud": "someone-else"}), state("42", "prod"))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("expired", func(t *testing.T) {
		ok, err := a.Authenticate(f.token(t, map[string]any{"repository_id": "42", "exp": time.Now().Add(-time.Hour).Unix()}), state("42", "prod"))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("wrong issuer", func(t *testing.T) {
		ok, err := a.Authenticate(f.token(t, map[string]any{"repository_id": "42", "iss": "https://evil.example.com"}), state("42", "prod"))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("signed by another key", func(t *testing.T) {
		other := newFakeIssuer(t)
		forged := other.token(t, map[string]any{"repository_id": "42", "iss": f.srv.URL})
		ok, err := a.Authenticate(forged, state("42", "prod"))
		require.Error(t, err)
		require.False(t, ok)
	})
}

func TestProjectExpression(t *testing.T) {
	f := newFakeIssuer(t)

	id := `claims.repository_id`
	idString := `string(claims.repository_id)`
	groups := `claims.groups.filter(g, g.startsWith("tf-")).map(g, g.substring(3))`
	tests := []struct {
		name       string
		expression string
		claims     map[string]any
		state      *terraform.State
		want       bool
	}{
		{"matches", id, map[string]any{"repository_id": "42"}, state("42", "prod"), true},
		{"all states by default", id, map[string]any{"repository_id": "42"}, state("42", "other"), true},
		{"other project", id, map[string]any{"repository_id": "42"}, state("43", "prod"), false},
		{"missing claim", id, map[string]any{}, state("42", "prod"), false},
		{"empty claim", id, map[string]any{"repository_id": ""}, state("", "prod"), false},
		{"integer needs conversion", id, map[string]any{"repository_id": 42}, state("42", "prod"), false},
		{"bool needs conversion", id, map[string]any{"repository_id": true}, state("true", "prod"), false},
		{"null claim", id, map[string]any{"repository_id": nil}, state("", "prod"), false},
		{"object claim", id, map[string]any{"repository_id": map[string]any{"id": "42"}}, state("42", "prod"), false},
		{"terraform-backend claim is ignored", id, map[string]any{"terraform-backend": map[string]any{"project": "42", "state": "*"}}, state("42", "prod"), false},
		{"converted string", idString, map[string]any{"repository_id": "42"}, state("42", "prod"), true},
		{"converted integer", idString, map[string]any{"repository_id": 42}, state("42", "prod"), true},
		{"converted integer of other project", idString, map[string]any{"repository_id": 42}, state("43", "prod"), false},
		{"converted large integer keeps precision", idString, map[string]any{"repository_id": uint64(9007199254740993)}, state("9007199254740993", "prod"), true},
		{"converted large integer doesn't match rounded value", idString, map[string]any{"repository_id": uint64(9007199254740993)}, state("9007199254740992", "prod"), false},
		{"converted null", idString, map[string]any{"repository_id": nil}, state("", "prod"), false},
		{"converted object", idString, map[string]any{"repository_id": map[string]any{"id": "42"}}, state("42", "prod"), false},
		{"string functions", `claims.repository.replace("/", "_")`, map[string]any{"repository": "owner/repo"}, state("owner_repo", "prod"), true},
		{"list of projects", groups, map[string]any{"groups": []string{"tf-network", "admins", "tf-dns"}}, state("dns", "prod"), true},
		{"list of projects without match", groups, map[string]any{"groups": []string{"tf-network", "admins"}}, state("admins", "prod"), false},
		{"list with non-string element", `claims.groups`, map[string]any{"groups": []any{"p1", 42}}, state("p1", "prod"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := f.auth(t, Config{ProjectExpression: tt.expression})
			ok, err := a.Authenticate(f.token(t, tt.claims), tt.state)
			require.NoError(t, err)
			require.Equal(t, tt.want, ok)
		})
	}
}

func TestStateExpression(t *testing.T) {
	f := newFakeIssuer(t)

	id := `claims.repository_id`
	tests := []struct {
		name   string
		cfg    Config
		claims map[string]any
		state  *terraform.State
		want   bool
	}{
		{"matches", Config{ProjectExpression: id, StateExpression: `claims.environment`}, map[string]any{"repository_id": "42", "environment": "prod"}, state("42", "prod"), true},
		{"other state", Config{ProjectExpression: id, StateExpression: `claims.environment`}, map[string]any{"repository_id": "42", "environment": "prod"}, state("42", "dev"), false},
		{"other project", Config{ProjectExpression: id, StateExpression: `claims.environment`}, map[string]any{"repository_id": "42", "environment": "prod"}, state("43", "prod"), false},
		{"list of states", Config{ProjectExpression: id, StateExpression: `claims.environments`}, map[string]any{"repository_id": "42", "environments": []string{"dev", "prod"}}, state("42", "dev"), true},
		{"list of states without match", Config{ProjectExpression: id, StateExpression: `claims.environments`}, map[string]any{"repository_id": "42", "environments": []string{"dev", "prod"}}, state("42", "staging"), false},
		{"wildcard", Config{ProjectExpression: id, StateExpression: `claims.environment`}, map[string]any{"repository_id": "42", "environment": "*"}, state("42", "prod"), true},
		{"wildcard in list", Config{ProjectExpression: id, StateExpression: `["dev", "*"]`}, map[string]any{"repository_id": "42"}, state("42", "prod"), true},
		{"expression with literal states", Config{ProjectExpression: id, StateExpression: `claims.ref == "refs/heads/main" ? "*" : "dev"`}, map[string]any{"repository_id": "42", "ref": "refs/heads/feature"}, state("42", "prod"), false},
		{"missing claim", Config{ProjectExpression: id, StateExpression: `claims.environment`}, map[string]any{"repository_id": "42"}, state("42", "prod"), false},
		{"empty claim", Config{ProjectExpression: id, StateExpression: `claims.environment`}, map[string]any{"repository_id": "42", "environment": ""}, state("42", ""), false},
		{"non-string result", Config{ProjectExpression: id, StateExpression: `claims.environment`}, map[string]any{"repository_id": "42", "environment": 1}, state("42", "1"), false},
		{"default project with state expression", Config{StateExpression: `claims.environment`}, map[string]any{"terraform-backend": map[string]any{"project": "42", "state": "other"}, "environment": "prod"}, state("42", "prod"), true},
		{"default project with state expression of other state", Config{StateExpression: `claims.environment`}, map[string]any{"terraform-backend": map[string]any{"project": "42", "state": "prod"}, "environment": "dev"}, state("42", "prod"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := f.auth(t, tt.cfg)
			ok, err := a.Authenticate(f.token(t, tt.claims), tt.state)
			require.NoError(t, err)
			require.Equal(t, tt.want, ok)
		})
	}
}

func TestValidationExpression(t *testing.T) {
	f := newFakeIssuer(t)

	id := `claims.repository_id`
	owner := `claims.repository_owner_id == "123"`
	tests := []struct {
		name       string
		validation string
		cfg        Config
		claims     map[string]any
		want       bool
	}{
		{"fulfilled", owner, Config{ProjectExpression: id}, map[string]any{"repository_id": "42", "repository_owner_id": "123"}, true},
		{"not fulfilled", owner, Config{ProjectExpression: id}, map[string]any{"repository_id": "42", "repository_owner_id": "456"}, false},
		{"missing claim", owner, Config{ProjectExpression: id}, map[string]any{"repository_id": "42"}, false},
		{"missing claim checked with has", `!has(claims.blocked)`, Config{ProjectExpression: id}, map[string]any{"repository_id": "42"}, true},
		{"list claim", `"terraform" in claims.groups`, Config{ProjectExpression: id}, map[string]any{"repository_id": "42", "groups": []string{"devs", "terraform"}}, true},
		{"integer claim", `claims.repository_owner_id in [123, 456]`, Config{ProjectExpression: id}, map[string]any{"repository_id": "42", "repository_owner_id": 456}, true},
		{"regex", `claims.ref.matches("^refs/(heads/main|tags/.*)$")`, Config{ProjectExpression: id}, map[string]any{"repository_id": "42", "ref": "refs/tags/v1"}, true},
		{"non-bool result", id, Config{ProjectExpression: id}, map[string]any{"repository_id": "42"}, false},
		{"applies to terraform-backend claim", owner, Config{}, map[string]any{"terraform-backend": map[string]any{"project": "42", "state": "*"}}, false},
		{"applies to terraform-backend claim fulfilled", owner, Config{}, map[string]any{"terraform-backend": map[string]any{"project": "42", "state": "*"}, "repository_owner_id": "123"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.ValidationExpression = tt.validation
			a := f.auth(t, tt.cfg)
			ok, err := a.Authenticate(f.token(t, tt.claims), state("42", "prod"))
			require.NoError(t, err)
			require.Equal(t, tt.want, ok)
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		err  string
	}{
		{"project expression syntax", Config{ProjectExpression: `claims.`}, "invalid project expression"},
		{"project expression type", Config{ProjectExpression: `1 + 2`}, "expression returns int"},
		{"state expression syntax", Config{StateExpression: `claims.`}, "invalid state expression"},
		{"state expression type", Config{StateExpression: `true`}, "expression returns bool"},
		{"validation expression syntax", Config{ValidationExpression: `claims.a ==`}, "invalid validation expression"},
		{"validation expression type", Config{ValidationExpression: `"yes"`}, "expression returns string"},
		{"unknown variable", Config{ValidationExpression: `token.a == "b"`}, "undeclared reference"},
		{"claim name with dash needs index", Config{ProjectExpression: `claims.terraform-backend.project`}, "undeclared reference"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewJWTAuth(tt.cfg)
			require.ErrorContains(t, err, tt.err)
		})
	}
}

// CEL's string() formats a float without fraction like an integer, so these
// tokens match the same project as the integer 42.
func TestStringConversionOfIntegralFloat(t *testing.T) {
	f := newFakeIssuer(t)
	a := f.auth(t, Config{ProjectExpression: `string(claims.repository_id)`})

	for _, raw := range []string{"42.0", "4.2e1"} {
		ok, err := a.Authenticate(f.token(t, map[string]any{"repository_id": json.RawMessage(raw)}), state("42", "prod"))
		require.NoError(t, err)
		require.True(t, ok, raw)
	}
}
