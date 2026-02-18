package auth

import (
	"testing"
)

func TestRegionIsValid(t *testing.T) {
	tests := []struct {
		region Region
		valid  bool
	}{
		{RegionUS, true},
		{RegionEU, true},
		{RegionJP, true},
		{RegionAU, true},
		{RegionSG, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.region.IsValid(); got != tt.valid {
			t.Errorf("Region(%q).IsValid() = %v, want %v", tt.region, got, tt.valid)
		}
	}
}

func TestValidRegions(t *testing.T) {
	regions := ValidRegions()
	if len(regions) != 6 {
		t.Errorf("ValidRegions() len = %d, want 6", len(regions))
	}
}

func TestProfileManager(t *testing.T) {
	dir := t.TempDir()
	pm := &ProfileManager{Dir: dir}

	// List should be empty
	profiles, err := pm.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected empty profiles, got %d", len(profiles))
	}

	// Save a profile
	p := &Profile{
		Name:      "test",
		Region:    RegionUS,
		StoreType: StoreKeychain,
		BaseURL:   "https://www.workato.com",
	}
	if err := pm.SaveProfile(p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	// Get the profile
	got, err := pm.GetProfile("test")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("Name = %q, want %q", got.Name, "test")
	}

	// Set active
	if err := pm.SetActiveProfile("test"); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}
	active, err := pm.GetActiveProfile()
	if err != nil {
		t.Fatalf("GetActiveProfile: %v", err)
	}
	if active != "test" {
		t.Errorf("active = %q, want %q", active, "test")
	}

	// Delete
	if err := pm.DeleteProfile("test"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	profiles, _ = pm.ListProfiles()
	if len(profiles) != 0 {
		t.Errorf("expected empty after delete, got %d", len(profiles))
	}
}

func TestEnvStore(t *testing.T) {
	store := &EnvStore{}

	// Without env vars, Get should fail
	_, err := store.Get(nil, "any")
	if err == nil {
		t.Error("expected error without WK_TOKEN set")
	}

	// Set/Delete should be no-ops
	if err := store.Set(nil, "x", &Credential{}); err != nil {
		t.Errorf("Set should be no-op: %v", err)
	}
	if err := store.Delete(nil, "x"); err != nil {
		t.Errorf("Delete should be no-op: %v", err)
	}

	// List without env
	names, err := store.List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}
