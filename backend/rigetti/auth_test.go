package rigetti

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRefreshTokenNestedTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.toml")

	// Real QCS secrets.toml format with nested credentials section.
	content := `# QCS Secrets
[credentials.default.token_payload]
refresh_token = "nested-refresh-token-abc123"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	token, err := readRefreshToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if token != "nested-refresh-token-abc123" {
		t.Errorf("token = %q, want nested-refresh-token-abc123", token)
	}
}

func TestReadRefreshTokenCredentialsSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.toml")

	content := `# QCS Secrets
[credentials]
refresh_token = "test-refresh-token-abc123"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	token, err := readRefreshToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if token != "test-refresh-token-abc123" {
		t.Errorf("token = %q, want test-refresh-token-abc123", token)
	}
}

func TestReadRefreshTokenBareValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.toml")

	content := `refresh_token = "bare-token"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	token, err := readRefreshToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if token != "bare-token" {
		t.Errorf("token = %q, want bare-token", token)
	}
}

func TestReadRefreshTokenNestedPreferredOverBare(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.toml")

	// If both bare and nested exist, prefer the nested one.
	content := `refresh_token = "bare-token"

[credentials.default.token_payload]
refresh_token = "nested-token"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	token, err := readRefreshToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if token != "nested-token" {
		t.Errorf("token = %q, want nested-token (nested should take priority)", token)
	}
}

func TestReadRefreshTokenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.toml")

	content := `# Empty file
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := readRefreshToken(path)
	if err == nil {
		t.Fatal("expected error for missing refresh_token")
	}
}

func TestReadRefreshTokenFileNotFound(t *testing.T) {
	_, err := readRefreshToken("/nonexistent/secrets.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestTokenExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle OIDC discovery request.
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			w.Header().Set("Content-Type", "application/json")
			// Return the token endpoint pointing back to this test server.
			json.NewEncoder(w).Encode(map[string]string{
				"token_endpoint": "http://" + r.Host + "/v1/token",
			})
			return
		}

		// Handle token exchange request.
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.FormValue("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", got)
		}
		if got := r.FormValue("refresh_token"); got != "test-refresh" {
			t.Errorf("refresh_token = %q, want test-refresh", got)
		}
		if got := r.FormValue("client_id"); got != defaultClientID {
			t.Errorf("client_id = %q, want %s", got, defaultClientID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(oauthTokenResponse{
			AccessToken: "fresh-access-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	}))
	defer srv.Close()

	tp := &tokenProvider{
		refreshToken: "test-refresh",
		issuer:       srv.URL,
		clientID:     defaultClientID,
		httpClient:   http.DefaultClient,
	}

	token, err := tp.getToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "fresh-access-token" {
		t.Errorf("token = %q, want fresh-access-token", token)
	}

	// Second call should use cached token.
	token2, err := tp.getToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token2 != "fresh-access-token" {
		t.Errorf("cached token = %q, want fresh-access-token", token2)
	}
}

func TestTokenExchangeOIDCFallback(t *testing.T) {
	// Test that OIDC discovery failure falls back to {issuer}/v1/token.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			// Return 404 to simulate missing OIDC config.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Expect the fallback path /v1/token.
		if r.URL.Path != "/v1/token" {
			t.Errorf("unexpected path %q, want /v1/token", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(oauthTokenResponse{
			AccessToken: "fallback-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	}))
	defer srv.Close()

	tp := &tokenProvider{
		refreshToken: "test-refresh",
		issuer:       srv.URL,
		clientID:     defaultClientID,
		httpClient:   http.DefaultClient,
	}

	token, err := tp.getToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "fallback-token" {
		t.Errorf("token = %q, want fallback-token", token)
	}
}

func TestTokenExchangeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"token_endpoint": "http://" + r.Host + "/v1/token",
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	tp := &tokenProvider{
		refreshToken: "bad-token",
		issuer:       srv.URL,
		clientID:     defaultClientID,
		httpClient:   http.DefaultClient,
	}

	_, err := tp.getToken(context.Background())
	if err == nil {
		t.Fatal("expected error for failed token exchange")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention 401: %v", err)
	}
}

func TestPreFetchedToken(t *testing.T) {
	tp := newTokenProviderWithToken("pre-fetched-token")
	token, err := tp.getToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "pre-fetched-token" {
		t.Errorf("token = %q, want pre-fetched-token", token)
	}
}
