package auth

import (
	"context"

	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
)

// CredentialStore is the interface for credential storage backends.
// Implementations include keyring (OS keychain), env (environment variables),
// and file (encrypted file).
type CredentialStore interface {
	// Get retrieves the credential for a named profile.
	Get(ctx context.Context, profileName string) (*Credential, error)

	// Set stores a credential for a named profile.
	Set(ctx context.Context, profileName string, cred *Credential) error

	// Delete removes the credential for a named profile.
	Delete(ctx context.Context, profileName string) error

	// List returns all profile names with stored credentials.
	List(ctx context.Context) ([]string, error)
}

// ChainStore tries multiple credential stores in order.
// Used to check env vars first (CI/CD), then keychain (interactive).
type ChainStore struct {
	Stores []CredentialStore
}

func NewChainStore(stores ...CredentialStore) *ChainStore {
	return &ChainStore{Stores: stores}
}

func (c *ChainStore) Get(ctx context.Context, profileName string) (*Credential, error) {
	for _, s := range c.Stores {
		cred, err := s.Get(ctx, profileName)
		if err == nil {
			return cred, nil
		}
	}
	return nil, wkerrors.ErrCredentialNotFound
}

func (c *ChainStore) Set(ctx context.Context, profileName string, cred *Credential) error {
	if len(c.Stores) == 0 {
		return wkerrors.ErrCredentialNotFound
	}
	// Set on the last store (most permanent)
	return c.Stores[len(c.Stores)-1].Set(ctx, profileName, cred)
}

func (c *ChainStore) Delete(ctx context.Context, profileName string) error {
	var lastErr error
	for _, s := range c.Stores {
		if err := s.Delete(ctx, profileName); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (c *ChainStore) List(ctx context.Context) ([]string, error) {
	seen := make(map[string]bool)
	var result []string
	for _, s := range c.Stores {
		names, err := s.List(ctx)
		if err != nil {
			continue
		}
		for _, n := range names {
			if !seen[n] {
				seen[n] = true
				result = append(result, n)
			}
		}
	}
	return result, nil
}


