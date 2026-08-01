package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/workato-devs/wk/internal/api"
	"github.com/workato-devs/wk/internal/auth"
	"github.com/workato-devs/wk/internal/config"
)

// Refusal bodies below are verbatim PUT/DELETE responses recorded from a
// trial workspace (2026-07-02/03). The platform returns HTTP 200 for these.
const (
	activationBlockedBody = `{"success":false,"code_errors":[[4,[["Response body/Error",null,"can't be blank","response.error"]]]],"config_errors":[]}`
	deleteRefusedBody     = `{"success":false,"errors":{"running":["Can't change the recipe state: invalid state running"]}}`
)

// writeLifecycleProject creates .wk/wk.toml plus a file-store profiles.env
// whose BASE_URL points at the test server, so resolveAPIClient succeeds
// end-to-end (same shape as writeDiscoverProject).
func writeLifecycleProject(t *testing.T, cwd string, entries []config.SyncEntry, baseURL string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(cwd, config.ProjectDir), 0755); err != nil {
		t.Fatalf("mkdir .wk: %v", err)
	}
	cfg := &config.Config{Name: "lifecycle-test", Profile: "ci", Sync: entries}
	if err := config.Save(config.ProjectConfigPath(cwd), cfg); err != nil {
		t.Fatalf("save cfg: %v", err)
	}
	body := "NAME=ci\nREGION=us\nWORKSPACE=acme\nENVIRONMENT=dev\nBASE_URL=" + baseURL + "\nTOKEN=tok-test\n"
	if err := os.WriteFile(auth.NewFileStore(cwd).Path, []byte(body), 0600); err != nil {
		t.Fatalf("writing profiles.env: %v", err)
	}
}

func TestRecipesStart_ActivationBlocked_ErrorsWithoutPolling(t *testing.T) {
	resetGlobalFlags(t)
	cwd := setupIsolatedHome(t)
	flagStoreType = string(auth.StoreFile)
	t.Cleanup(func() { flagStoreType = "" })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT" && r.URL.Path == "/api/recipes/42/start":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(activationBlockedBody))
		case r.Method == "GET":
			// A blocked activation must not fall through to the poll loop.
			t.Errorf("unexpected poll: GET %s", r.URL.Path)
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	writeLifecycleProject(t, cwd, nil, srv.URL)

	root := NewRootCmd()
	root.AddCommand(newRecipesCmd())
	root.SetArgs([]string{"recipes", "start", "42", "--store-type", "file", "--profile", "ci"})
	root.SilenceErrors = true
	err := root.Execute()
	if err == nil {
		t.Fatal("expected activation error, got nil")
	}
	if !strings.Contains(err.Error(), "can't be blank") {
		t.Errorf("error = %q, want the platform step detail included", err)
	}
}

func TestRecipesStart_Bulk_ContinuesPastBlockedRecipes(t *testing.T) {
	resetGlobalFlags(t)
	cwd := setupIsolatedHome(t)
	flagStoreType = string(auth.StoreFile)
	t.Cleanup(func() { flagStoreType = "" })

	var starts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/start") {
			starts.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(activationBlockedBody))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	writeLifecycleProject(t, cwd, nil, srv.URL)

	root := NewRootCmd()
	root.AddCommand(newRecipesCmd())
	root.SetArgs([]string{"recipes", "start", "41", "42", "--store-type", "file", "--profile", "ci"})
	root.SilenceErrors = true
	err := root.Execute()
	if err == nil {
		t.Fatal("expected joined activation errors, got nil")
	}
	if got := starts.Load(); got != 2 {
		t.Errorf("start requests = %d, want 2 (one blocked recipe must not abandon the batch)", got)
	}
	for _, id := range []string{"41", "42"} {
		if !strings.Contains(err.Error(), "recipe "+id) {
			t.Errorf("error should report recipe %s: %q", id, err)
		}
	}
}

func TestRecipesDelete_RefusedWhileRunning_PreservesLocalFiles(t *testing.T) {
	resetGlobalFlags(t)
	cwd := setupIsolatedHome(t)
	flagStoreType = string(auth.StoreFile)
	t.Cleanup(func() { flagStoreType = "" })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/recipes/42":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(api.Recipe{ID: 42, Name: "slack_bot", Running: true})
		case r.Method == "DELETE" && r.URL.Path == "/api/recipes/42":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(deleteRefusedBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	writeLifecycleProject(t, cwd, []config.SyncEntry{{ServerPath: "Recipes", LocalPath: "./recipes"}}, srv.URL)
	assetPath, metaPath := seedRecipeLocals(t, cwd, "recipes", "slack_bot")

	root := NewRootCmd()
	root.AddCommand(newRecipesCmd())
	root.SetArgs([]string{"recipes", "delete", "42", "--store-type", "file", "--profile", "ci"})
	root.SilenceErrors = true
	err := root.Execute()
	if err == nil {
		t.Fatal("expected refusal error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid state running") {
		t.Errorf("error = %q, want the platform refusal reason", err)
	}
	for _, p := range []string{assetPath, metaPath} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Errorf("local file %s should be preserved on a refused server delete: %v", p, statErr)
		}
	}
}
