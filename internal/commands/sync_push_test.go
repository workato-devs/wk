package commands

import (
	"strings"
	"testing"
)

// TestPush_NoCreateAndCreatePathAreMutuallyExclusive guards the flag
// gate that runs before any API work — passing both contradicts each
// other and should error early.
func TestPush_NoCreateAndCreatePathAreMutuallyExclusive(t *testing.T) {
	resetGlobalFlags(t)
	_ = setupIsolatedHome(t)
	writeProjectSkel(t, ".", nil)

	root := NewRootCmd()
	root.AddCommand(newPushCmd())
	root.SetArgs([]string{"push", "--no-create", "--create-path"})
	err := root.Execute()
	if err == nil {
		t.Fatal("err = nil, want error (--no-create and --create-path are exclusive)")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("err = %v, want 'mutually exclusive'", err)
	}
}
