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

func state(project, name string) *terraform.State {
	return &terraform.State{ID: terraform.GetStateID(project, name), Project: project, Name: name}
}

func TestLegacyClaim(t *testing.T) {
	f := newFakeIssuer(t)
	a := NewJWTAuth(f.srv.URL, testAudience, "")

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

	// a token without the legacy claim must not grant access, even if claim mapping would match
	ok, err = a.Authenticate(f.token(t, map[string]any{"repository_id": "42"}), state("42", "prod"))
	require.NoError(t, err)
	require.False(t, ok)
}

func TestProjectClaim(t *testing.T) {
	f := newFakeIssuer(t)
	a := NewJWTAuth(f.srv.URL, testAudience, "repository_id")

	tests := []struct {
		name   string
		claims map[string]any
		state  *terraform.State
		want   bool
	}{
		{"matches", map[string]any{"repository_id": "42"}, state("42", "prod"), true},
		{"any state in project", map[string]any{"repository_id": "42"}, state("42", "other"), true},
		{"other project", map[string]any{"repository_id": "42"}, state("43", "prod"), false},
		{"missing claim", map[string]any{}, state("42", "prod"), false},
		{"empty claim", map[string]any{"repository_id": ""}, state("", "prod"), false},
		{"numeric claim is not accepted", map[string]any{"repository_id": 42}, state("42", "prod"), false},
		{"legacy claim is ignored", map[string]any{"terraform-backend": map[string]any{"project": "42", "state": "*"}}, state("42", "prod"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := a.Authenticate(f.token(t, tt.claims), tt.state)
			require.NoError(t, err)
			require.Equal(t, tt.want, ok)
		})
	}
}

func TestInvalidTokens(t *testing.T) {
	f := newFakeIssuer(t)
	a := NewJWTAuth(f.srv.URL, testAudience, "repository_id")

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
