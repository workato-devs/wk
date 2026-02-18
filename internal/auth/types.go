package auth

import "time"

// Region represents a Workato data center region.
type Region string

const (
	RegionUS Region = "us"
	RegionEU Region = "eu"
	RegionJP Region = "jp"
	RegionAU Region = "au"
	RegionSG    Region = "sg"
	RegionTrial Region = "trial"
)

// ValidRegions returns all supported regions.
func ValidRegions() []Region {
	return []Region{RegionUS, RegionEU, RegionJP, RegionAU, RegionSG, RegionTrial}
}

// IsValid checks if a region string is a supported region.
func (r Region) IsValid() bool {
	for _, v := range ValidRegions() {
		if v == r {
			return true
		}
	}
	return false
}

// StoreType identifies the credential storage backend.
type StoreType string

const (
	StoreKeychain StoreType = "keychain"
	StoreEnv      StoreType = "env"
	StoreFile     StoreType = "file"
	StoreVault    StoreType = "vault"
)

// Credential holds an API token and its metadata.
type Credential struct {
	Token     string     `json:"token"`
	Region    Region     `json:"region"`
	StoreType StoreType  `json:"store_type"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Profile represents a named workspace authentication profile.
type Profile struct {
	Name       string     `json:"name"`
	Region     Region     `json:"region"`
	StoreType  StoreType  `json:"store_type"`
	BaseURL    string     `json:"base_url"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
