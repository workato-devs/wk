package telemetry

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/term"
)

const configFile = "telemetry.json"

// TelemetryConfig is persisted to ~/.wk/telemetry.json.
type TelemetryConfig struct {
	Disabled         bool   `json:"disabled"`
	Ack              bool   `json:"ack"`                        // first-run notice shown
	FallbackDeviceID string `json:"fallback_device_id,omitempty"` // used when OS machine ID is unavailable
}

// configPath returns the path to ~/.wk/telemetry.json.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wk", configFile), nil
}

// ReadConfig reads ~/.wk/telemetry.json. Returns a zero-value config on any
// read or parse error — telemetry defaults to enabled, unacked.
func ReadConfig() TelemetryConfig {
	path, err := configPath()
	if err != nil {
		return TelemetryConfig{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return TelemetryConfig{} // file missing is normal on first run
	}
	var cfg TelemetryConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return TelemetryConfig{}
	}
	return cfg
}

// WriteConfig atomically writes cfg to ~/.wk/telemetry.json.
// Creates ~/.wk/ if it does not exist.
func WriteConfig(cfg TelemetryConfig) error {
	path, err := configPath()
	if err != nil {
		return fmt.Errorf("resolving config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	// Write atomically via temp file + rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return os.Rename(tmp, path)
}

// ShowFirstRunNotice prints a one-time telemetry disclosure to stderr if:
//   - telemetry is not disabled
//   - the ack flag is not set in the config
//   - stdin is a terminal (not CI or piped input)
//
// After printing, it sets Ack=true in the config so the notice never appears again.
func ShowFirstRunNotice() {
	if IsDisabled() {
		return
	}
	cfg := ReadConfig()
	if cfg.Ack {
		return
	}
	// Only show in interactive terminal sessions.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	fmt.Fprintln(os.Stderr, "Attention: wk collects anonymous usage telemetry.")
	fmt.Fprintln(os.Stderr, "Run `wk telemetry status` to see what is collected, or `wk telemetry disable` to opt out.")
	cfg.Ack = true
	_ = WriteConfig(cfg) // best-effort; a failed write just shows the notice again next time
}

// FallbackDeviceID returns a persistent generated identifier for environments
// where the OS machine ID is not accessible. The ID is generated once,
// stored in the telemetry config, and reused on subsequent calls.
func FallbackDeviceID() string {
	cfg := ReadConfig()
	if cfg.FallbackDeviceID != "" {
		return cfg.FallbackDeviceID
	}
	// Generate a deterministic-ish ID from time + pid. Not a real UUID,
	// but good enough for a fallback — no external uuid library needed.
	raw := fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	sum := sha256.Sum256([]byte(raw))
	id := fmt.Sprintf("%x", sum[:16])
	cfg.FallbackDeviceID = id
	_ = WriteConfig(cfg)
	return id
}

// ConfigFilePath returns the path to the telemetry config file for display
// in `wk telemetry status`. Returns the path even if the file does not exist.
func ConfigFilePath() string {
	path, err := configPath()
	if err != nil {
		return "~/.wk/" + configFile
	}
	return path
}

// disabledByEnv returns true if any opt-out environment variable is set.
func disabledByEnv() bool {
	return os.Getenv("DO_NOT_TRACK") == "1" ||
		os.Getenv("WK_TELEMETRY_DISABLED") == "1"
}

// disabledByConfig returns true if the persisted config has Disabled=true.
func disabledByConfig() bool {
	return ReadConfig().Disabled
}

// parseIntStr is a small helper used by FallbackDeviceID.
var _ = strconv.Itoa // ensure strconv is used
