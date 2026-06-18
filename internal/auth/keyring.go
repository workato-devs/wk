package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"

	wkerrors "github.com/workato-devs/wk/internal/errors"
)

// keyringService is the OS keychain service name under which credentials are
// stored. It is intentionally kept as "wk-cli" (rather than "wk") so existing
// keychain entries remain readable after the wk-cli-beta -> wk rename — the name
// is internal and never surfaced to users.
const keyringService = "wk-cli"

// KeyringStore stores credentials in the OS keychain via go-keyring.
type KeyringStore struct{}

func (k *KeyringStore) Get(_ context.Context, profileName string) (*Credential, error) {
	data, err := keyring.Get(keyringService, profileName)
	if err != nil {
		return nil, wkerrors.ErrCredentialNotFound
	}
	var cred Credential
	if err := json.Unmarshal([]byte(data), &cred); err != nil {
		return nil, wkerrors.ErrCredentialNotFound
	}
	return &cred, nil
}

func (k *KeyringStore) Set(_ context.Context, profileName string, cred *Credential) error {
	data, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	if err := keyring.Set(keyringService, profileName, string(data)); err != nil {
		return err
	}
	return k.addToProfileList(profileName)
}

func (k *KeyringStore) Delete(_ context.Context, profileName string) error {
	_ = keyring.Delete(keyringService, profileName)
	return k.removeFromProfileList(profileName)
}

func (k *KeyringStore) List(_ context.Context) ([]string, error) {
	return k.loadProfileList()
}

// profileListPath returns the path to the keyring profile tracking file.
func (k *KeyringStore) profileListPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".wk")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "keyring_profiles.json"), nil
}

func (k *KeyringStore) loadProfileList() ([]string, error) {
	path, err := k.profileListPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return nil, nil
	}
	return names, nil
}

func (k *KeyringStore) saveProfileList(names []string) error {
	path, err := k.profileListPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(names)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (k *KeyringStore) addToProfileList(name string) error {
	names, _ := k.loadProfileList()
	for _, n := range names {
		if n == name {
			return nil
		}
	}
	names = append(names, name)
	return k.saveProfileList(names)
}

func (k *KeyringStore) removeFromProfileList(name string) error {
	names, _ := k.loadProfileList()
	filtered := names[:0]
	for _, n := range names {
		if n != name {
			filtered = append(filtered, n)
		}
	}
	return k.saveProfileList(filtered)
}
