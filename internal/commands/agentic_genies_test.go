package commands

import (
	"strings"
	"testing"
)

// The API ignores any --state value but active/inactive and returns every
// genie, so wk rejects the rest instead of letting the filter quietly not
// filter.
func TestGenieStateFilter(t *testing.T) {
	for _, ok := range []string{"", "active", "inactive"} {
		if err := genieStateFilter(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for v, want := range map[string]string{
		"started": "use --state active",
		"stopped": "use --state inactive",
		"running": "valid: active, inactive",
	} {
		if err := genieStateFilter(v); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", v, err, want)
		}
	}
}
