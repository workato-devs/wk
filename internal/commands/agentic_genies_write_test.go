package commands

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

// The API's 200 on assign/remove doesn't mean every ID was applied, so the
// outcome comes from membership before and after (ADR-009 Decision 6).
func TestReconcileAssignment(t *testing.T) {
	m := func(found, attached bool) membership { return membership{found: found, attached: attached} }
	tests := []struct {
		name          string
		assign        bool
		before, after membership
		want          string
		ok            bool
	}{
		{"assign attaches", true, m(true, false), m(true, true), outcomeAssigned, true},
		{"assign, already there", true, m(true, true), m(true, true), outcomeAlreadyAssigned, true},
		{"assign unknown ID (silently ignored)", true, m(false, false), m(false, false), outcomeNotFound, false},
		{"assign that didn't take", true, m(true, false), m(true, false), outcomeNotApplied, false},
		{"remove detaches", false, m(true, true), m(true, false), outcomeRemoved, true},
		{"remove, wasn't attached", false, m(true, false), m(true, false), outcomeNotAssigned, true},
		{"remove unknown ID", false, m(false, false), m(false, false), outcomeNotFound, false},
		{"remove that didn't take", false, m(true, true), m(true, true), outcomeNotApplied, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := reconcileAssignment("gin-1", "x", "knowledge_bases", []string{"id"},
				map[string]membership{"id": tt.before}, map[string]membership{"id": tt.after}, tt.assign)
			if r.Results[0].Outcome != tt.want || r.OK != tt.ok {
				t.Errorf("outcome = %s ok = %v, want %s ok = %v", r.Results[0].Outcome, r.OK, tt.want, tt.ok)
			}
		})
	}
}

// update sends only the fields the caller set, so it can't blank others.
func TestGenieInputFlags_OnlyChangedFields(t *testing.T) {
	var f genieInputFlags
	cmd := &cobra.Command{Use: "update", RunE: func(*cobra.Command, []string) error { return nil }}
	f.register(cmd)
	cmd.SetArgs([]string{"--name", "New", "--matrix=false"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	in, err := f.input(cmd)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(in)
	if string(body) != `{"name":"New","matrix":false}` {
		t.Errorf("body = %s, want only the set fields", body)
	}
}
