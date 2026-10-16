package auth

import "testing"

// Regression for #100: the duplicate-target check must compare workspace IDs,
// not display names. Separate workspaces can share a display name (e.g. several
// trial workspaces under one account), and blocking those logins left no way
// to register the second profile.
func TestHasDuplicateTarget(t *testing.T) {
	existing := []*Profile{
		{Name: "academy", Workspace: "Zayne Turner", WorkspaceID: 2100005003, Environment: "dev_sbx", Region: "trial"},
		{Name: "legacy", Workspace: "Acme", Environment: "dev", Region: "us"}, // saved before workspace_id was recorded
	}

	tests := []struct {
		name    string
		p       *Profile
		wantDup string // "" means no duplicate
	}{
		{
			name:    "same display name, different workspace ID",
			p:       &Profile{Name: "e2e-test", Workspace: "Zayne Turner", WorkspaceID: 2100000735, Environment: "dev_sbx", Region: "trial"},
			wantDup: "",
		},
		{
			name:    "same workspace ID, environment and region",
			p:       &Profile{Name: "academy-2", Workspace: "Zayne Turner", WorkspaceID: 2100005003, Environment: "dev_sbx", Region: "trial"},
			wantDup: "academy",
		},
		{
			name:    "same workspace ID after a rename",
			p:       &Profile{Name: "renamed", Workspace: "Zayne's Sandbox", WorkspaceID: 2100005003, Environment: "dev_sbx", Region: "trial"},
			wantDup: "academy",
		},
		{
			name:    "same workspace ID, different environment",
			p:       &Profile{Name: "academy-prod", Workspace: "Zayne Turner", WorkspaceID: 2100005003, Environment: "prod", Region: "trial"},
			wantDup: "",
		},
		{
			name:    "same name is an overwrite, not a duplicate",
			p:       &Profile{Name: "academy", Workspace: "Zayne Turner", WorkspaceID: 2100005003, Environment: "dev_sbx", Region: "trial"},
			wantDup: "",
		},
		{
			name:    "existing profile has no workspace ID",
			p:       &Profile{Name: "acme-dev", Workspace: "Acme", WorkspaceID: 42, Environment: "dev", Region: "us"},
			wantDup: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasDuplicateTarget(existing, tt.p)
			gotName := ""
			if got != nil {
				gotName = got.Name
			}
			if gotName != tt.wantDup {
				t.Errorf("hasDuplicateTarget() = %q, want %q", gotName, tt.wantDup)
			}
		})
	}
}
