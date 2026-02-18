package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type packageService struct {
	client *HTTPClient
}

func (s *packageService) Export(ctx context.Context, folderID int) (int, error) {
	// Step 1: Create an export manifest for the folder.
	// The Workato RLCM API requires a manifest before exporting.
	manifestReq := map[string]any{
		"export_manifest": map[string]any{
			"name":                  fmt.Sprintf("wk-export-%d", time.Now().Unix()),
			"folder_id":            folderID,
			"auto_generate_assets": true,
		},
	}
	var manifestResp struct {
		Result ExportManifest `json:"result"`
	}
	if err := s.client.do(ctx, "POST", "/export_manifests", manifestReq, &manifestResp); err != nil {
		return 0, fmt.Errorf("creating export manifest: %w", err)
	}
	manifest := manifestResp.Result

	// Step 2: Start the package export using the manifest ID.
	var result struct {
		ID int `json:"id"`
	}
	if err := s.client.do(ctx, "POST", fmt.Sprintf("/packages/export/%d", manifest.ID), nil, &result); err != nil {
		return 0, fmt.Errorf("starting package export: %w", err)
	}
	return result.ID, nil
}

func (s *packageService) ExportStatus(ctx context.Context, packageID int) (*Package, error) {
	var pkg Package
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/packages/%d", packageID), nil, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (s *packageService) Download(ctx context.Context, packageID int) ([]byte, error) {
	return s.client.doRaw(ctx, "GET", fmt.Sprintf("/packages/%d/download", packageID))
}

func (s *packageService) Import(ctx context.Context, folderID int, data []byte, restartRecipes bool) (int, error) {
	path := fmt.Sprintf("/packages/import/%d?restart_recipes=%t", folderID, restartRecipes)
	req, err := http.NewRequestWithContext(ctx, "POST", s.client.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.client.token)
	req.Header.Set("User-Agent", "wk-cli/dev")
	req.Header.Set("Content-Type", "application/octet-stream")

	if s.client.verbose {
		fmt.Fprintf(os.Stderr, "[debug] POST %s%s\n", s.client.baseURL, path)
	}

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if s.client.verbose {
		fmt.Fprintf(os.Stderr, "[debug] HTTP %d %s\n", resp.StatusCode, resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}
	return result.ID, nil
}

func (s *packageService) ImportStatus(ctx context.Context, importID int) (*Package, error) {
	var pkg Package
	if err := s.client.do(ctx, "GET", fmt.Sprintf("/packages/%d", importID), nil, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}
