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

func TestReadRefreshToken(t *testing.T) {
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
		tokenURL:     srv.URL,
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

func TestTokenExchangeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	tp := &tokenProvider{
		refreshToken: "bad-token",
		tokenURL:     srv.URL,
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
