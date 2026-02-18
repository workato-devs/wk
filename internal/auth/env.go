package auth

import (
	"context"
	"os"
	"time"

	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
)

// EnvStore reads credentials from environment variables.
// WK_TOKEN provides the API token. WK_REGION optionally sets the region.
type EnvStore struct{}

func (e *EnvStore) Get(_ context.Context, profileName string) (*Credential, error) {
	if profileName != "" && profileName != "env" {
		return nil, wkerrors.ErrCredentialNotFound
	}
	token := os.Getenv("WK_TOKEN")
	if token == "" {
		return nil, wkerrors.ErrCredentialNotFound
	}
	region := Region(os.Getenv("WK_REGION"))
	if !region.IsValid() {
		region = RegionUS
	}
	return &Credential{
		Token:     token,
		Region:    region,
		StoreType: StoreEnv,
		CreatedAt: time.Now(),
	}, nil
}

func (e *EnvStore) Set(_ context.Context, _ string, _ *Credential) error {
	return nil
}

func (e *EnvStore) Delete(_ context.Context, _ string) error {
	return nil
}

func (e *EnvStore) List(_ context.Context) ([]string, error) {
	if os.Getenv("WK_TOKEN") != "" {
		return []string{"env"}, nil
	}
	return nil, nil
}
