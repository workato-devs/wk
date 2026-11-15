package telemetry

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// hmacKey is application-specific. Two tools using the same technique produce
// different device IDs for the same machine, preventing cross-tool correlation.
// This value must not change — a change invalidates all existing device IDs.
const hmacKey = "wk-v1"

var (
	deviceIDOnce sync.Once
	deviceIDVal  string
)

// DeviceID returns a stable, non-reversible identifier for the current machine.
// It is computed once per process and cached. See ADR-010 Decision 2 for the
// full rationale and HMAC construction.
func DeviceID() string {
	deviceIDOnce.Do(func() {
		raw := readMachineID()
		if raw == "" {
			// Fall back to a generated UUID stored in ~/.wk/telemetry.json.
			// This survives reinstalls on the same machine only if the config file persists.
			raw = FallbackDeviceID()
		}
		mac := hmac.New(sha256.New, []byte(hmacKey))
		mac.Write([]byte(raw))
		deviceIDVal = fmt.Sprintf("%x", mac.Sum(nil))
	})
	return deviceIDVal
}

// readMachineID reads the OS-provided stable machine identifier.
// Returns empty string if unavailable — caller falls back to FallbackDeviceID.
func readMachineID() string {
	switch runtime.GOOS {
	case "linux":
		return readLinuxMachineID()
	case "darwin":
		return readDarwinMachineID()
	case "windows":
		return readWindowsMachineID()
	default:
		return ""
	}
}

func readLinuxMachineID() string {
	for _, path := range []string{"/var/lib/dbus/machine-id", "/etc/machine-id"} {
		data, err := os.ReadFile(path)
		if err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				return id
			}
		}
	}
	return ""
}

func readDarwinMachineID() string {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return ""
	}
	// Look for: "IOPlatformUUID" = "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX"
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "IOPlatformUUID") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				id := strings.TrimSpace(parts[1])
				id = strings.Trim(id, `"`)
				if id != "" {
					return id
				}
			}
		}
	}
	return ""
}

func readWindowsMachineID() string {
	out, err := exec.Command(
		"reg", "query",
		`HKLM\SOFTWARE\Microsoft\Cryptography`,
		"/v", "MachineGuid",
	).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "MachineGuid") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return strings.TrimSpace(fields[len(fields)-1])
			}
		}
	}
	return ""
}
