package google

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// tokenProvider wraps an oauth2.TokenSource with caching and thread safety.
type tokenProvider struct {
	mu  sync.Mutex
	src oauth2.TokenSource
}

// newTokenProviderFromDefault creates a tokenProvider using Application Default Credentials.
func newTokenProviderFromDefault(ctx context.Context) (*tokenProvider, error) {
	src, err := google.DefaultTokenSource(ctx, cloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("google: default credentials: %w", err)
	}
	return &tokenProvider{src: oauth2.ReuseTokenSource(nil, src)}, nil
}

// newTokenProviderFromJSON creates a tokenProvider from a service account JSON key.
func newTokenProviderFromJSON(ctx context.Context, jsonKey []byte) (*tokenProvider, error) {
	creds, err := google.CredentialsFromJSON(ctx, jsonKey, cloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("google: credentials from JSON: %w", err)
	}
	return &tokenProvider{src: oauth2.ReuseTokenSource(nil, creds.TokenSource)}, nil
}

// newTokenProviderFromSource creates a tokenProvider from a custom oauth2.TokenSource.
func newTokenProviderFromSource(src oauth2.TokenSource) *tokenProvider {
	return &tokenProvider{src: oauth2.ReuseTokenSource(nil, src)}
}

// getToken returns a valid bearer token, refreshing if necessary.
func (tp *tokenProvider) getToken(ctx context.Context) (string, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	tok, err := tp.src.Token()
	if err != nil {
		return "", fmt.Errorf("google: get token: %w", err)
	}
	return tok.AccessToken, nil
}
