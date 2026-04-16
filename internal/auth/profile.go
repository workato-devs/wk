package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	wkerrors "github.com/workato-devs/wk-cli-beta/internal/errors"
)

// hasDuplicateTarget checks whether any existing profile (with a different
// name) already targets the same (workspace, environment, region) tuple.
// Profiles with empty workspace or environment are skipped (backward compat).
func hasDuplicateTarget(profiles []*Profile, p *Profile) *Profile {
	if p.Workspace == "" || p.Environment == "" {
		return nil
	}
	for _, existing := range profiles {
		if existing.Name == p.Name {
			continue
		}
		if existing.Workspace == "" || existing.Environment == "" {
			continue
		}
		if existing.Workspace == p.Workspace &&
			existing.Environment == p.Environment &&
			existing.Region == p.Region {
			return existing
		}
	}
	return nil
}

// ProfileManager handles reading and writing profile metadata on disk.
type ProfileManager struct {
	Dir string // defaults to ~/.wk/
}

// NewProfileManager creates a ProfileManager with the default directory ~/.wk/.
func NewProfileManager() *ProfileManager {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".wk")
	_ = os.MkdirAll(dir, 0700)
	return &ProfileManager{Dir: dir}
}

func (pm *ProfileManager) profilesPath() string {
	return filepath.Join(pm.Dir, "profiles.json")
}

func (pm *ProfileManager) activeProfilePath() string {
	return filepath.Join(pm.Dir, "active_profile")
}

func (pm *ProfileManager) loadProfiles() ([]*Profile, error) {
	data, err := os.ReadFile(pm.profilesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var profiles []*Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parsing profiles.json: %w", err)
	}
	return profiles, nil
}

func (pm *ProfileManager) saveProfiles(profiles []*Profile) error {
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pm.profilesPath(), data, 0600)
}

// SaveProfile adds or updates a profile in profiles.json.
// Returns ErrDuplicateTarget if another profile already targets the same
// (workspace, environment, region) tuple.
func (pm *ProfileManager) SaveProfile(p *Profile) error {
	profiles, err := pm.loadProfiles()
	if err != nil {
		return err
	}

	if dup := hasDuplicateTarget(profiles, p); dup != nil {
		return fmt.Errorf("%w: profile %q already targets %s/%s (%s)",
			wkerrors.ErrDuplicateTarget, dup.Name, dup.Workspace, dup.Environment, dup.Region)
	}

	found := false
	for i, existing := range profiles {
		if existing.Name == p.Name {
			profiles[i] = p
			found = true
			break
		}
	}
	if !found {
		profiles = append(profiles, p)
	}
	return pm.saveProfiles(profiles)
}

// GetProfile returns a profile by name.
func (pm *ProfileManager) GetProfile(name string) (*Profile, error) {
	profiles, err := pm.loadProfiles()
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, wkerrors.ErrProfileNotFound
}

// ListProfiles returns all saved profiles.
func (pm *ProfileManager) ListProfiles() ([]*Profile, error) {
	return pm.loadProfiles()
}

// DeleteProfile removes a profile by name.
func (pm *ProfileManager) DeleteProfile(name string) error {
	profiles, err := pm.loadProfiles()
	if err != nil {
		return err
	}
	filtered := make([]*Profile, 0, len(profiles))
	for _, p := range profiles {
		if p.Name != name {
			filtered = append(filtered, p)
		}
	}
	return pm.saveProfiles(filtered)
}

// SetActiveProfile writes the active profile name to disk.
func (pm *ProfileManager) SetActiveProfile(name string) error {
	return os.WriteFile(pm.activeProfilePath(), []byte(name), 0600)
}

// GetActiveProfile reads the active profile name from disk.
func (pm *ProfileManager) GetActiveProfile() (string, error) {
	data, err := os.ReadFile(pm.activeProfilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", wkerrors.ErrNoActiveProfile
		}
		return "", err
	}
	name := string(data)
	if name == "" {
		return "", wkerrors.ErrNoActiveProfile
	}
	return name, nil
}
