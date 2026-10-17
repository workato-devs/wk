package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/workato-devs/wk/internal/api"
)

func TestClientCreateCmd_RejectsBadInput(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no name", []string{"--type", "api_key"}, "--name is required"},
		{"unknown type", []string{"--name", "N", "--type", "basic"}, "--type must be one of api_key, oauth"},
		{"oauth without redirect", []string{"--name", "N", "--type", "oauth"}, "--redirect-url is required"},
		{"redirect on api_key", []string{"--name", "N", "--redirect-url", "https://x"}, "applies only to --type oauth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runSilently(newClientCreateCmd(), tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
	err := runSilently(newClientUpdateCmd(), []string{"gincl-1", "--ip-allow-list", "10.0.0.1", "--clear-ip-allow-list"})
	if err == nil || !strings.Contains(err.Error(), "not both") {
		t.Errorf("update both ip flags: err = %v", err)
	}
}

func runSilently(cmd *cobra.Command, args []string) error {
	cmd.SetArgs(args)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	return cmd.Execute()
}

func TestCheckClientUpdate(t *testing.T) {
	ips := []string{"10.0.0.1"}
	if err := checkClientUpdate(api.GenieClientUpdate{IPAllowList: &ips}, &api.GenieClient{IPAllowList: []string{"10.0.0.1"}}); err != nil {
		t.Errorf("applied: %v", err)
	}
	// Clearing: the API reports [] and the request sent [].
	if err := checkClientUpdate(api.GenieClientUpdate{IPAllowList: &[]string{}}, &api.GenieClient{IPAllowList: []string{}}); err != nil {
		t.Errorf("cleared: %v", err)
	}
	urls := []string{"https://a"}
	err := checkClientUpdate(api.GenieClientUpdate{OAuthRedirectURLs: &urls}, &api.GenieClient{})
	if err == nil || !strings.Contains(err.Error(), "redirect URLs are [], not [https://a]") {
		t.Errorf("ignored: %v", err)
	}
}

func TestClientInUseError(t *testing.T) {
	msg := clientInUseError("gincl-1", []string{"gin-a"}).Error()
	for _, want := range []string{"attached to genie gin-a", "wk agentic clients detach gin-a gincl-1", "wk agentic clients delete gincl-1 --force"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}

// The API answers both kinds of conflict with 409 and an unhelpful title
// (captured live 2026-10-17); the client's genie_ids say which one it was.
func TestExplainAttachConflict(t *testing.T) {
	conflict := &api.APIError{StatusCode: 409, Message: "Genie has already been taken"}
	tests := []struct {
		name, genieIDs, wantOutcome, wantErr string
	}{
		{"already on this genie", `["gin-1"]`, outcomeAlreadyAttached, ""},
		{"on another genie", `["gin-2"]`, "", "wk agentic clients detach gin-2 gincl-1"},
		{"attached nowhere", `[]`, "", "Genie has already been taken"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"data":{"client_id":"gincl-1","genie_ids":` + tt.genieIDs + `}}`))
			}))
			defer srv.Close()
			outcome, err := explainAttachConflict(context.Background(), api.NewHTTPClient(srv.URL, "tok"), "gin-1", "gincl-1", conflict)
			if outcome != tt.wantOutcome {
				t.Errorf("outcome = %q, want %q", outcome, tt.wantOutcome)
			}
			if (tt.wantErr == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
