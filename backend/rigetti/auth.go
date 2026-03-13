package rigetti

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// defaultIssuer is the OIDC issuer URL for QCS authentication.
	// The real qcs-sdk-rust performs OIDC Discovery (fetching
	// {issuer}/.well-known/openid-configuration) to find the token
	// endpoint dynamically. We do the same.
	defaultIssuer   = "https://auth.qcs.rigetti.com/oauth2/aus8jcovzG0gW2TUG355"
	defaultClientID = "0oa3ykoirzDKpkfzk357"
)

// tokenProvider manages Rigetti QCS OAuth2 token lifecycle.
type tokenProvider struct {
	refreshToken string
	issuer       string // OIDC issuer URL
	tokenURL     string // discovered via OIDC, or empty until first use
	clientID     string
	mu           sync.Mutex
	accessToken  string
	expiry       time.Time
	httpClient   *http.Client
}

func newTokenProvider(credPath string) (*tokenProvider, error) {
	tp := &tokenProvider{
		issuer:     defaultIssuer,
		clientID:   defaultClientID,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}

	if credPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("rigetti: determine home directory: %w", err)
		}
		credPath = filepath.Join(home, ".qcs", "secrets.toml")
	}

	refreshToken, err := readRefreshToken(credPath)
	if err != nil {
		return nil, err
	}
	tp.refreshToken = refreshToken
	return tp, nil
}

// newTokenProviderWithToken creates a provider with a pre-fetched access token.
func newTokenProviderWithToken(token string) *tokenProvider {
	return &tokenProvider{
		accessToken: token,
		expiry:      time.Now().Add(24 * time.Hour), // assume long-lived
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

// oidcConfig is the subset of OpenID Connect discovery we need.
type oidcConfig struct {
	TokenEndpoint string `json:"token_endpoint"`
}

// oauthTokenResponse is the JSON response from the OAuth2 token endpoint.
type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
	TokenType   string `json:"token_type"`
}

// discoverTokenURL performs OIDC Discovery to find the token endpoint.
// Fetches {issuer}/.well-known/openid-configuration and extracts token_endpoint.
func (tp *tokenProvider) discoverTokenURL(ctx context.Context) (string, error) {
	if tp.tokenURL != "" {
		return tp.tokenURL, nil
	}

	discoveryURL := tp.issuer + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", fmt.Errorf("rigetti: create OIDC discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := tp.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("rigetti: OIDC discovery request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("rigetti: read OIDC discovery response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Fall back to conventional {issuer}/v1/token if discovery fails.
		tp.tokenURL = tp.issuer + "/v1/token"
		return tp.tokenURL, nil
	}

	var cfg oidcConfig
	if err := json.Unmarshal(body, &cfg); err != nil || cfg.TokenEndpoint == "" {
		// Fall back to conventional path.
		tp.tokenURL = tp.issuer + "/v1/token"
		return tp.tokenURL, nil
	}

	tp.tokenURL = cfg.TokenEndpoint
	return tp.tokenURL, nil
}

// getToken returns a valid bearer token, refreshing if necessary.
func (tp *tokenProvider) getToken(ctx context.Context) (string, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Return cached token if still valid (with 60s buffer).
	if tp.accessToken != "" && time.Now().Before(tp.expiry.Add(-60*time.Second)) {
		return tp.accessToken, nil
	}

	if tp.refreshToken == "" {
		return "", fmt.Errorf("rigetti: no refresh token available and access token expired")
	}

	// Discover token endpoint via OIDC on first use.
	tokenURL, err := tp.discoverTokenURL(ctx)
	if err != nil {
		return "", err
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tp.refreshToken},
		"client_id":     {tp.clientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("rigetti: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tp.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("rigetti: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("rigetti: read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("rigetti: token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp oauthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("rigetti: unmarshal token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("rigetti: token endpoint returned empty access token")
	}

	tp.accessToken = tokenResp.AccessToken
	if tokenResp.ExpiresIn > 0 {
		tp.expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	} else {
		tp.expiry = time.Now().Add(time.Hour)
	}

	return tp.accessToken, nil
}

// readRefreshToken reads the refresh_token from a QCS secrets.toml file.
// Minimal hand-rolled parsing to avoid a TOML library dependency.
//
// The real secrets.toml has a nested structure like:
//
//	[credentials.default.token_payload]
//	refresh_token = "..."
//
// But some users may have a flat file with just:
//
//	refresh_token = "..."
//
// We handle both by scanning for `refresh_token = "..."` anywhere in
// a [credentials.*] section, or as a bare top-level key.
func readRefreshToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("rigetti: read credentials file %s: %w", path, err)
	}

	var (
		inCredSection bool
		token         string
	)

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Track TOML section headers.
		if strings.HasPrefix(line, "[") {
			section := strings.Trim(line, "[] ")
			// Match [credentials], [credentials.default],
			// [credentials.default.token_payload], etc.
			inCredSection = strings.HasPrefix(section, "credentials")
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		if key == "refresh_token" {
			// Strip surrounding quotes.
			val = strings.Trim(val, `"'`)
			if val == "" {
				continue
			}
			// Prefer a token found in a [credentials.*] section.
			if inCredSection {
				return val, nil
			}
			// Keep bare value as fallback.
			if token == "" {
				token = val
			}
		}
	}

	if token != "" {
		return token, nil
	}
	return "", fmt.Errorf("rigetti: refresh_token not found in %s", path)
}
