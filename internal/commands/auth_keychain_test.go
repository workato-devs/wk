package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/workato-devs/wk/internal/auth"
	wkerrors "github.com/workato-devs/wk/internal/errors"
)

// A login whose token can't be stored must not leave a profile with no token
// behind: a new profile is removed, and an overwritten one is restored.
func TestKeychainSetError_RollsBackProfile(t *testing.T) {
	setErr := errors.New("exit status 154")

	t.Run("new profile is removed", func(t *testing.T) {
		pm := &auth.ProfileManager{Dir: t.TempDir()}
		p := &auth.Profile{Name: "new", Workspace: "Acme", WorkspaceID: 1, Environment: "dev", Region: auth.RegionUS}
		if err := pm.SaveProfile(p); err != nil {
			t.Fatalf("SaveProfile: %v", err)
		}

		err := keychainSetError(pm, "new", nil, setErr)

		if _, gerr := pm.GetProfile("new"); !errors.Is(gerr, wkerrors.ErrProfileNotFound) {
			t.Errorf("profile still present after failed login (GetProfile err = %v)", gerr)
		}
		for _, want := range []string{"exit status 154", `profile "new" was not saved`, "--store-type file"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q missing %q", err, want)
			}
		}
	})

	t.Run("overwritten profile is restored", func(t *testing.T) {
		pm := &auth.ProfileManager{Dir: t.TempDir()}
		previous := &auth.Profile{Name: "dev", Workspace: "Acme", WorkspaceID: 1, Environment: "dev", Region: auth.RegionUS, BaseURL: "https://old"}
		if err := pm.SaveProfile(previous); err != nil {
			t.Fatalf("SaveProfile(previous): %v", err)
		}
		replacement := &auth.Profile{Name: "dev", Workspace: "Acme", WorkspaceID: 1, Environment: "dev", Region: auth.RegionUS, BaseURL: "https://new"}
		if err := pm.SaveProfile(replacement); err != nil {
			t.Fatalf("SaveProfile(replacement): %v", err)
		}

		err := keychainSetError(pm, "dev", previous, setErr)

		got, gerr := pm.GetProfile("dev")
		if gerr != nil {
			t.Fatalf("GetProfile: %v", gerr)
		}
		if got.BaseURL != "https://old" {
			t.Errorf("BaseURL = %q, want the previous profile restored (https://old)", got.BaseURL)
		}
		if !strings.Contains(err.Error(), `profile "dev" was left unchanged`) {
			t.Errorf("error %q should say the profile was left unchanged", err)
		}
	})
}
