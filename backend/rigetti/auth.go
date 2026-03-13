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
	defaultTokenURL = "https://auth.qcs.rigetti.com/oauth2/aus8jcovzG0gW2TUG355/v1/token"
	defaultClientID = "0oa3ykoirzDKpkfzk357"
)

// tokenProvider manages Rigetti QCS OAuth2 token lifecycle.
type tokenProvider struct {
	refreshToken string
	tokenURL     string
	clientID     string
	mu           sync.Mutex
	accessToken  string
	expiry       time.Time
	httpClient   *http.Client
}

func newTokenProvider(credPath string) (*tokenProvider, error) {
	tp := &tokenProvider{
		tokenURL:   defaultTokenURL,
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

// oauthTokenResponse is the JSON response from the OAuth2 token endpoint.
type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
	TokenType   string `json:"token_type"`
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

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tp.refreshToken},
		"client_id":     {tp.clientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tp.tokenURL, strings.NewReader(form.Encode()))
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
func readRefreshToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("rigetti: read credentials file %s: %w", path, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
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
				return "", fmt.Errorf("rigetti: empty refresh_token in %s", path)
			}
			return val, nil
		}
	}
	return "", fmt.Errorf("rigetti: refresh_token not found in %s", path)
}
