package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"github.com/stretchr/testify/require"

	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

const testAudience = "terraform-backend"

// claims are the claims of a token.
type claims = map[string]any

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

// withRawDefaultClaim returns token claims with the given value as the
// "terraform-backend" claim.
func withRawDefaultClaim(value any) claims {
	return claims{"terraform-backend": value}
}

// withDefaultClaim returns token claims with the "terraform-backend" claim and
// any other given claims.
func withDefaultClaim(project, stateName any, other ...claims) claims {
	c := withRawDefaultClaim(claims{"project": project, "state": stateName})
	for _, o := range other {
		maps.Copy(c, o)
	}
	return c
}

// repo returns the claims of a token with the "repository_id" claim.
func repo(id any) claims {
	return claims{"repository_id": id}
}

func TestDefaultClaim(t *testing.T) {
	f := newFakeIssuer(t)
	a := f.auth(t, Config{})

	token := f.token(t, withDefaultClaim("p1", "prod"))
	wildcard := f.token(t, withDefaultClaim("p1", "*"))

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
	ok, err = a.Authenticate(f.token(t, repo("42")), state("42", "prod"))
	require.NoError(t, err)
	require.False(t, ok)
}

// Tokens using the "terraform-backend" claim must behave the same as before
// expressions were added.
func TestDefaultClaimCompatibility(t *testing.T) {
	f := newFakeIssuer(t)
	a := f.auth(t, Config{})

	huge := json.RawMessage("18446744073709551616")
	withUnknownField := func(value any) claims {
		return withRawDefaultClaim(claims{"project": "p1", "state": "prod", "x": value})
	}
	tests := []struct {
		name    string
		claims  claims
		want    bool
		wantErr bool
	}{
		{name: "strings", claims: withDefaultClaim("p1", "prod"), want: true},
		{name: "wildcard", claims: withDefaultClaim("p1", "*"), want: true},
		{name: "unknown field in the claim", claims: withUnknownField(1), want: true},
		{name: "huge integer in another claim", claims: withDefaultClaim("p1", "prod", claims{"other": huge}), want: true},
		{name: "huge integer in the claim", claims: withUnknownField(huge), want: true},
		{
			name:   "huge float in another claim",
			claims: withDefaultClaim("p1", "prod", claims{"other": json.RawMessage("1e999")}),
			want:   true,
		},
		{name: "capitalized fields", claims: withRawDefaultClaim(claims{"Project": "p1", "STATE": "prod"}), want: true},
		{
			name:   "capitalized claim name",
			claims: claims{"Terraform-Backend": withDefaultClaim("p1", "prod")["terraform-backend"]},
			want:   true,
		},
		{name: "wildcard project", claims: withDefaultClaim("*", "prod")},
		{name: "null state", claims: withDefaultClaim("p1", nil)},
		{name: "missing state", claims: withRawDefaultClaim(claims{"project": "p1"})},
		{name: "no claim", claims: claims{}},
		{name: "list of projects", claims: withDefaultClaim([]string{"p1", "p2"}, "prod"), wantErr: true},
		{name: "list of states", claims: withDefaultClaim("p1", []string{"prod"}), wantErr: true},
		{name: "list with wildcard state", claims: withDefaultClaim("p1", []string{"*"}), wantErr: true},
		{name: "integer project", claims: withDefaultClaim(1, "prod"), wantErr: true},
		{name: "claim is not an object", claims: withRawDefaultClaim("x"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := a.Authenticate(f.token(t, tt.claims), state("p1", "prod"))
			require.Equal(t, tt.wantErr, err != nil, "error: %v", err)
			require.Equal(t, tt.want, ok)
		})
	}
}

func TestListsFromExpressions(t *testing.T) {
	f := newFakeIssuer(t)

	projectOfClaim := `claims["terraform-backend"].project`
	stateOfClaim := `claims["terraform-backend"].state`
	environments := Config{StateExpression: `claims.environments`}
	tests := []struct {
		name    string
		cfg     Config
		claims  claims
		state   *terraform.State
		want    bool
		wantErr bool
	}{
		{
			name:   "project expression",
			cfg:    Config{ProjectExpression: `claims.projects`},
			claims: claims{"projects": []string{"p1", "p2"}},
			state:  state("p2", "prod"),
			want:   true,
		},
		{
			name:   "explicit expressions on the claim",
			cfg:    Config{ProjectExpression: projectOfClaim, StateExpression: stateOfClaim},
			claims: withDefaultClaim([]string{"p1", "p2"}, []string{"prod"}),
			state:  state("p2", "prod"),
			want:   true,
		},
		{
			name:   "explicit expressions on the claim with other project",
			cfg:    Config{ProjectExpression: projectOfClaim, StateExpression: stateOfClaim},
			claims: withDefaultClaim([]string{"p1", "p2"}, []string{"prod"}),
			state:  state("p3", "prod"),
		},
		{
			name:   "explicit expressions on the claim with other state",
			cfg:    Config{ProjectExpression: projectOfClaim, StateExpression: stateOfClaim},
			claims: withDefaultClaim([]string{"p1", "p2"}, []string{"prod"}),
			state:  state("p2", "dev"),
		},
		{
			name:   "state expression with default project",
			cfg:    environments,
			claims: withDefaultClaim("p1", nil, claims{"environments": []string{"dev", "prod"}}),
			state:  state("p1", "prod"),
			want:   true,
		},
		{
			name:    "state expression doesn't allow a list as default project",
			cfg:     environments,
			claims:  withDefaultClaim([]string{"p1"}, nil, claims{"environments": []string{"prod"}}),
			state:   state("p1", "prod"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := f.auth(t, tt.cfg)
			ok, err := a.Authenticate(f.token(t, tt.claims), tt.state)
			require.Equal(t, tt.wantErr, err != nil, "error: %v", err)
			require.Equal(t, tt.want, ok)
		})
	}
}

func TestInvalidTokens(t *testing.T) {
	f := newFakeIssuer(t)
	a := f.auth(t, Config{ProjectExpression: `claims.repository_id`})

	t.Run("wrong audience", func(t *testing.T) {
		token := f.token(t, claims{"repository_id": "42", "aud": "someone-else"})
		ok, err := a.Authenticate(token, state("42", "prod"))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("expired", func(t *testing.T) {
		token := f.token(t, claims{"repository_id": "42", "exp": time.Now().Add(-time.Hour).Unix()})
		ok, err := a.Authenticate(token, state("42", "prod"))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("wrong issuer", func(t *testing.T) {
		token := f.token(t, claims{"repository_id": "42", "iss": "https://evil.example.com"})
		ok, err := a.Authenticate(token, state("42", "prod"))
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("signed by another key", func(t *testing.T) {
		other := newFakeIssuer(t)
		forged := other.token(t, claims{"repository_id": "42", "iss": f.srv.URL})
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
	filtered := `claims.groups.filter(g, g.startsWith("tf-"))`
	replaced := `claims.repository.replace("/", "_")`
	tfGroups := claims{"groups": []string{"tf-network", "admins", "tf-dns"}}
	large := uint64(9007199254740993)
	tests := []struct {
		name       string
		expression string
		claims     claims
		state      *terraform.State
		want       bool
	}{
		{"matches", id, repo("42"), state("42", "prod"), true},
		{"all states by default", id, repo("42"), state("42", "other"), true},
		{"other project", id, repo("42"), state("43", "prod"), false},
		{"missing claim", id, claims{}, state("42", "prod"), false},
		{"empty claim", id, repo(""), state("", "prod"), false},
		{"integer needs conversion", id, repo(42), state("42", "prod"), false},
		{"bool needs conversion", id, repo(true), state("true", "prod"), false},
		{"null claim", id, repo(nil), state("", "prod"), false},
		{"object claim", id, repo(claims{"id": "42"}), state("42", "prod"), false},
		{"terraform-backend claim is ignored", id, withDefaultClaim("42", "*"), state("42", "prod"), false},

		{"converted string", idString, repo("42"), state("42", "prod"), true},
		{"converted integer", idString, repo(42), state("42", "prod"), true},
		{"converted integer of other project", idString, repo(42), state("43", "prod"), false},
		{"converted large integer keeps precision", idString, repo(large), state("9007199254740993", "prod"), true},
		{
			"converted large integer doesn't match rounded value",
			idString,
			repo(large),
			state("9007199254740992", "prod"),
			false,
		},
		{"converted null", idString, repo(nil), state("", "prod"), false},
		{"converted object", idString, repo(claims{"id": "42"}), state("42", "prod"), false},

		{"string functions", replaced, claims{"repository": "owner/repo"}, state("owner_repo", "prod"), true},

		{"list of projects", groups, tfGroups, state("dns", "prod"), true},
		{"list of projects without match", groups, tfGroups, state("admins", "prod"), false},
		{"wildcard doesn't grant all projects", id, repo("*"), state("42", "prod"), false},
		{"wildcard in list doesn't grant all projects", `[claims.repository_id]`, repo("*"), state("42", "prod"), false},
		{"list with non-string element", `claims.groups`, claims{"groups": []any{"p1", 42}}, state("p1", "prod"), false},
		{"filtered list", filtered, tfGroups, state("tf-network", "prod"), true},
		{"filtered list without match", filtered, tfGroups, state("admins", "prod"), false},
		{"list literal of a claim", `[claims.repository_id]`, repo("42"), state("42", "prod"), true},
		{"list literal with non-string element", `[claims.repository_id]`, repo(42), state("42", "prod"), false},
		{"mixed list literal with non-string element", `["p1", claims.repository_id]`, repo(42), state("p1", "prod"), false},
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
	environment := Config{ProjectExpression: id, StateExpression: `claims.environment`}
	environments := Config{ProjectExpression: id, StateExpression: `claims.environments`}
	nonProd := Config{ProjectExpression: id, StateExpression: `claims.environments.filter(e, e != "prod")`}
	listInclWildcard := Config{ProjectExpression: id, StateExpression: `["dev", "*"]`}
	devAndProd := claims{"repository_id": "42", "environments": []string{"dev", "prod"}}
	defaultProject := Config{StateExpression: `claims.environment`}
	tests := []struct {
		name   string
		cfg    Config
		claims claims
		state  *terraform.State
		want   bool
	}{
		{"matches", environment, claims{"repository_id": "42", "environment": "prod"}, state("42", "prod"), true},
		{"other state", environment, claims{"repository_id": "42", "environment": "prod"}, state("42", "dev"), false},
		{"other project", environment, claims{"repository_id": "42", "environment": "prod"}, state("43", "prod"), false},
		{"list of states", environments, devAndProd, state("42", "dev"), true},
		{"list of states without match", environments, devAndProd, state("42", "staging"), false},
		{"wildcard", environment, claims{"repository_id": "42", "environment": "*"}, state("42", "prod"), true},
		{"wildcard in list", listInclWildcard, repo("42"), state("42", "prod"), true},
		{"filtered list of states", nonProd, devAndProd, state("42", "dev"), true},
		{"filtered list of states without match", nonProd, devAndProd, state("42", "prod"), false},
		{
			"expression with literal states",
			Config{ProjectExpression: id, StateExpression: `claims.ref == "refs/heads/main" ? "*" : "dev"`},
			claims{"repository_id": "42", "ref": "refs/heads/feature"},
			state("42", "prod"),
			false,
		},
		{"missing claim", environment, repo("42"), state("42", "prod"), false},
		{"empty claim", environment, claims{"repository_id": "42", "environment": ""}, state("42", ""), false},
		{"non-string result", environment, claims{"repository_id": "42", "environment": 1}, state("42", "1"), false},
		{
			"default project with state expression",
			defaultProject,
			withDefaultClaim("42", "other", claims{"environment": "prod"}),
			state("42", "prod"),
			true,
		},
		{
			"default project with state expression of other state",
			defaultProject,
			withDefaultClaim("42", "prod", claims{"environment": "dev"}),
			state("42", "prod"),
			false,
		},
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
	project := Config{ProjectExpression: id}
	tests := []struct {
		name       string
		validation string
		cfg        Config
		claims     claims
		want       bool
	}{
		{"fulfilled", owner, project, claims{"repository_id": "42", "repository_owner_id": "123"}, true},
		{"not fulfilled", owner, project, claims{"repository_id": "42", "repository_owner_id": "456"}, false},
		{"missing claim", owner, project, repo("42"), false},
		{"missing claim checked with has", `!has(claims.blocked)`, project, repo("42"), true},
		{
			"list claim",
			`"terraform" in claims.groups`,
			project,
			claims{"repository_id": "42", "groups": []string{"devs", "terraform"}},
			true,
		},
		{
			"integer claim",
			`claims.repository_owner_id in [123, 456]`,
			project,
			claims{"repository_id": "42", "repository_owner_id": 456},
			true,
		},
		{
			"regex",
			`claims.ref.matches("^refs/(heads/main|tags/.*)$")`,
			project,
			claims{"repository_id": "42", "ref": "refs/tags/v1"},
			true,
		},
		{"non-bool result", id, project, repo("42"), false},
		{"applies to terraform-backend claim", owner, Config{}, withDefaultClaim("42", "*"), false},
		{
			"applies to terraform-backend claim fulfilled",
			owner,
			Config{},
			withDefaultClaim("42", "*", claims{"repository_owner_id": "123"}),
			true,
		},
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

func TestExpressionCostLimit(t *testing.T) {
	f := newFakeIssuer(t)

	expression := `claims.groups.all(a, claims.groups.all(b, claims.groups.all(c, a == b || a != b)))`
	a := f.auth(t, Config{ProjectExpression: `claims.repository_id`, ValidationExpression: expression})

	groups := func(n int) claims {
		return claims{"repository_id": "42", "groups": slices.Repeat([]string{"group"}, n)}
	}

	t.Run("below the limit", func(t *testing.T) {
		ok, err := a.Authenticate(f.token(t, groups(2)), state("42", "prod"))
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("above the limit", func(t *testing.T) {
		ok, err := a.Authenticate(f.token(t, groups(200)), state("42", "prod"))
		require.NoError(t, err)
		require.False(t, ok)

		prg, err := compileExpression(expression, cel.BoolType)
		require.NoError(t, err)
		_, err = evalBool(prg, groups(200))
		require.ErrorContains(t, err, "cost limit")
	})
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
		{"project expression list type", Config{ProjectExpression: `[1, 2]`}, "expression returns list(int)"},
		{"state expression list type", Config{StateExpression: `[true]`}, "expression returns list(bool)"},
		{"validation expression syntax", Config{ValidationExpression: `claims.a ==`}, "invalid validation expression"},
		{"validation expression type", Config{ValidationExpression: `"yes"`}, "expression returns string"},
		{"unknown variable", Config{ValidationExpression: `token.a == "b"`}, "undeclared reference"},
		{
			"claim name with dash needs index",
			Config{ProjectExpression: `claims.terraform-backend.project`},
			"undeclared reference",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewJWTAuth(tt.cfg)
			require.ErrorContains(t, err, tt.err)
		})
	}
}

func TestNumbersOutOfRange(t *testing.T) {
	f := newFakeIssuer(t)
	a := f.auth(t, Config{ProjectExpression: `string(claims.repository_id)`})

	raw := func(number string) json.RawMessage { return json.RawMessage(number) }
	tooLarge := raw("18446744073709551616")

	rejected := []struct {
		name   string
		claims claims
	}{
		{"integer above uint64", repo(tooLarge)},
		{"integer below int64", repo(raw("-9223372036854775809"))},
		{"float above float64", repo(raw("1e999"))},
		{"in a list", repo([]any{tooLarge})},
		{"in an object", repo(claims{"id": tooLarge})},
		{"in another claim", claims{"repository_id": "42", "other": tooLarge}},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := a.Authenticate(f.token(t, tt.claims), state("42", "prod"))
			require.ErrorContains(t, err, "out of range")
			require.False(t, ok)
		})
	}

	accepted := []struct {
		name   string
		number string
	}{
		{"largest uint64", "18446744073709551615"},
		{"smallest int64", "-9223372036854775808"},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := a.Authenticate(f.token(t, repo(raw(tt.number))), state(tt.number, "prod"))
			require.NoError(t, err)
			require.True(t, ok)
		})
	}

	t.Run("float in another claim", func(t *testing.T) {
		ok, err := a.Authenticate(f.token(t, claims{"repository_id": "42", "other": raw("42.5")}), state("42", "prod"))
		require.NoError(t, err)
		require.True(t, ok)
	})
}

// CEL's string() formats a float without fraction like an integer, so these
// tokens match the same project as the integer 42.
func TestStringConversionOfIntegralFloat(t *testing.T) {
	f := newFakeIssuer(t)
	a := f.auth(t, Config{ProjectExpression: `string(claims.repository_id)`})

	for _, raw := range []string{"42.0", "4.2e1"} {
		ok, err := a.Authenticate(f.token(t, repo(json.RawMessage(raw))), state("42", "prod"))
		require.NoError(t, err)
		require.True(t, ok, raw)
	}
}
