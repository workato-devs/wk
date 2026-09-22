package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// customConnectorsBody is the /custom_connectors envelope as returned by a live
// workspace. It is raw JSON rather than an encoded []Connector on purpose:
// encoding the struct round-trips its own json tags, so a mistyped tag could
// never fail the test.
const customConnectorsBody = `{"result":[
  {"id":1000001,"name":"reporting_connector_1234567_1700000001",
   "title":"Reporting","latest_released_version":4,
   "latest_released_version_note":"add batch action","static_webhook_url":"",
   "released_versions":[
     {"version":4,"version_note":"add batch action","created_at":"2026-09-22T10:00:22.587-07:00","released_at":"2026-09-22T10:00:39.958-07:00"},
     {"version":3,"version_note":"initial","created_at":"2026-09-22T09:45:20.544-07:00","released_at":"2026-09-22T09:45:37.540-07:00"}]},
  {"id":1000002,"name":"crm_sync_connector_1234567_1700000002",
   "title":"Customer Records","latest_released_version":1,"released_versions":[]},
  {"id":1000003,"name":"draft_connector_1234567_1700000003",
   "title":"Never Released","latest_released_version":null,"released_versions":[]}
]}`

func customConnectorsServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		// /integrations is platform connector metadata, not custom connectors.
		// Without ?applications= it returns {"items":[]} with HTTP 200, which
		// is how this command used to report an empty workspace.
		if r.URL.Path != "/custom_connectors" {
			t.Errorf("path = %s, want /custom_connectors", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q, want none; search is applied client-side", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(customConnectorsBody))
	}))
}

func TestConnectorService_List(t *testing.T) {
	srv := customConnectorsServer(t)
	defer srv.Close()

	client := NewHTTPClient(srv.URL, "test-token")
	connectors, err := client.Connectors().List(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(connectors) != 3 {
		t.Fatalf("got %d connectors, want 3", len(connectors))
	}

	amp := connectors[0]
	if amp.ID != 1000001 || amp.Title != "Reporting" {
		t.Errorf("first connector = %+v, want id 1000001 titled Reporting", amp)
	}
	if amp.LatestReleasedVersion == nil || *amp.LatestReleasedVersion != 4 {
		t.Errorf("latest_released_version = %v, want 4", amp.LatestReleasedVersion)
	}
	if len(amp.ReleasedVersions) != 2 || amp.ReleasedVersions[0].Version != 4 {
		t.Errorf("released_versions = %+v, want two entries starting at v4", amp.ReleasedVersions)
	}
	if amp.ReleasedVersions[0].ReleasedAt.IsZero() {
		t.Error("released_at did not parse")
	}

	if never := connectors[2]; never.LatestReleasedVersion != nil {
		t.Errorf("never-released connector has latest_released_version %d, want nil",
			*never.LatestReleasedVersion)
	}
}

func TestConnectorService_ListWithSearch(t *testing.T) {
	srv := customConnectorsServer(t)
	defer srv.Close()
	client := NewHTTPClient(srv.URL, "test-token")

	tests := []struct {
		search string
		want   []int
	}{
		{"reporting", []int{1000001}},                           // lower-case against title
		{"CUSTOMER", []int{1000002}},                            // title only - absent from name, and upper-cased
		{"connector_1234567", []int{1000001, 1000002, 1000003}}, // matches name, not just title
		{"nothing-matches", nil},
	}
	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			got, err := client.Connectors().List(context.Background(), tt.search)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("search %q returned %d connectors, want %d", tt.search, len(got), len(tt.want))
			}
			for i, id := range tt.want {
				if got[i].ID != id {
					t.Errorf("search %q result %d = id %d, want %d", tt.search, i, got[i].ID, id)
				}
			}
		})
	}
}
