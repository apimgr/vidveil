// SPDX-License-Identifier: MIT
// AI.md PART 31: Tor CLI commands
// Tor is configured via server.yml and the CLI only; there is no PUBLIC REST
// API for Tor configuration. Mutating commands reach the server-owned Tor
// process over the internal loopback-only control channel exposed at
// /server/tor/* (src/server/tor_control.go) — never by touching Tor state
// directly, since the running server binary is the sole owner of that
// process.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/config"
)

// errNoRunningServer is the exact message required by AI.md PART 31 for
// every mutating tor subcommand when no running server can be reached.
const errNoRunningServer = "Error: no running server detected — start the server first"

// handleTorCommand dispatches the `tor` CLI subcommands per AI.md PART 31:
// status | validate | restart | regenerate | vanity start | vanity apply | import-keys <path>
func handleTorCommand(args []string, configDir, dataDir string) int {
	if len(args) == 0 {
		printTorHelp()
		return 1
	}

	switch args[0] {
	case "status":
		return torStatus(configDir, dataDir)
	case "validate":
		return torValidate(configDir, dataDir)
	case "restart":
		return torRestart(configDir, dataDir)
	case "regenerate":
		return torRegenerate(configDir, dataDir)
	case "vanity":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: tor vanity {start <prefix>|apply}")
			return 1
		}
		switch args[1] {
		case "start":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Usage: tor vanity start <prefix>")
				return 1
			}
			return torVanityStart(configDir, dataDir, args[2])
		case "apply":
			return torVanityApply(configDir, dataDir)
		default:
			fmt.Fprintf(os.Stderr, "Unknown vanity command: %s\nUsage: tor vanity {start <prefix>|apply}\n", args[1])
			return 1
		}
	case "import-keys":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: tor import-keys <path>")
			return 1
		}
		return torImportKeys(configDir, dataDir, args[1])
	case "help", "--help", "-h":
		printTorHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown tor command: %s\n\n", args[0])
		printTorHelp()
		return 1
	}
}

// printTorHelp prints the tor command usage per AI.md PART 31 CLI table
func printTorHelp() {
	binaryName := filepath.Base(os.Args[0])
	fmt.Printf(`Tor Hidden Service Commands:
  %s tor status              - View Tor hidden service status
  %s tor validate            - Validate Tor configuration
  %s tor restart             - Restart the Tor process
  %s tor regenerate          - Regenerate the .onion address (new keys)
  %s tor vanity start <pfx>  - Start vanity address search (prefix a-z, 2-7, max 6 chars)
  %s tor vanity apply        - Apply the pending vanity address
  %s tor import-keys <path>  - Import an existing hs_ed25519_secret_key

Tor is configured via server.yml and CLI only.
`, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName)
}

// torDirs resolves the tor data directory paths used by the CLI
func torDirs(configDir, dataDir string) (torDir, siteDir string) {
	paths := config.GetAppPaths(configDir, dataDir)
	torDir = filepath.Join(paths.Data, "tor")
	siteDir = filepath.Join(torDir, "site")
	return torDir, siteDir
}

// readHostnameFile reads the stored .onion address from {data_dir}/tor/site/hostname
func readHostnameFile(siteDir string) string {
	data, err := os.ReadFile(filepath.Join(siteDir, "hostname"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// resolveServerAddr resolves the running server's loopback address the same
// way --status resolves it: configured port, overridden by the same env
// vars main.go honors (VIDVEIL_PORT > PORT), falling back to server.yml.
func resolveServerAddr(configDir, dataDir string) string {
	cfg, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return ""
	}

	port := cfg.Server.Port
	if envPort := os.Getenv("VIDVEIL_PORT"); envPort != "" {
		port = envPort
	} else if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	if port == "" {
		return ""
	}
	return net.JoinHostPort("127.0.0.1", port)
}

// healthzTor holds the Tor fields parsed from the running server's public
// /server/healthz JSON (never the internal /server/tor/* control channel).
type healthzTor struct {
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
}

// healthzResponse holds the /server/healthz fields the CLI needs for
// `tor status` and `--status`.
type healthzResponse struct {
	Uptime   string `json:"uptime"`
	Mode     string `json:"mode"`
	Features struct {
		Tor healthzTor `json:"tor"`
	} `json:"features"`
}

// queryHealthz fetches the public /server/healthz JSON from the running
// server, or nil if the server is unreachable or the response can't be
// parsed. Healthz is public/unauthenticated per AI.md PART 13-15 — it is
// intentionally separate from the loopback-only /server/tor/* control
// channel used for mutating Tor operations.
func queryHealthz(configDir, dataDir string) *healthzResponse {
	addr := resolveServerAddr(configDir, dataDir)
	if addr == "" {
		return nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/server/healthz", addr), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var health healthzResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil
	}
	return &health
}

// controlEnvelope mirrors the canonical {ok,data}/{ok,error,message} envelope
// returned by every /server/tor/* internal control endpoint.
type controlEnvelope struct {
	OK      bool                   `json:"ok"`
	Data    map[string]interface{} `json:"data"`
	Error   string                 `json:"error"`
	Message string                 `json:"message"`
}

// torControlRequest issues a request against the running server's internal
// loopback-only Tor control channel (src/server/tor_control.go) and decodes
// the canonical response envelope. A network-level error (connection
// refused, timeout) means no server is reachable at all; callers distinguish
// that from an in-envelope {"ok":false,...} application error.
func torControlRequest(method, addr, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("http://%s%s", addr, path), reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var envelope controlEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if !envelope.OK {
		return nil, fmt.Errorf("%s", envelope.Message)
	}
	return envelope.Data, nil
}

// serverReachable reports whether a running server answers on its resolved
// loopback address at all (regardless of what the Tor control channel says),
// distinguishing "no server running" from "server running, tor unavailable".
func serverReachable(addr string) bool {
	if addr == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// requireRunningServer resolves the server address and enforces the
// mandatory "no running server" exit-1 error for mutating tor subcommands
// per AI.md PART 31.
func requireRunningServer(configDir, dataDir string) (string, bool) {
	addr := resolveServerAddr(configDir, dataDir)
	if !serverReachable(addr) {
		fmt.Fprintln(os.Stderr, errNoRunningServer)
		return "", false
	}
	return addr, true
}

// torStatus implements `tor status` per AI.md PART 31
// Queries the running server's internal /server/tor/status control endpoint;
// falls back to on-disk key state when no server is running.
func torStatus(configDir, dataDir string) int {
	_, siteDir := torDirs(configDir, dataDir)

	if addr := resolveServerAddr(configDir, dataDir); serverReachable(addr) {
		data, err := torControlRequest(http.MethodGet, addr, "/server/tor/status", nil)
		if err == nil {
			running, _ := data["running"].(bool)
			starting, _ := data["starting"].(bool)
			enabled, _ := data["enabled"].(bool)
			statusStr, _ := data["status_string"].(string)
			onion, _ := data["onion_address"].(string)

			state := "Disabled"
			switch {
			case running:
				state = "Connected"
			case starting:
				state = "Starting"
			case enabled:
				state = "Disconnected"
			}
			fmt.Printf("Tor Hidden Service: %s\n", state)
			if onion != "" {
				fmt.Printf("  Address: %s\n", onion)
			}
			if statusStr != "" {
				fmt.Printf("  Status: %s\n", statusStr)
			}
			return 0
		}
		// Server reachable but tor control unavailable (e.g. not wired up) -
		// fall through to on-disk reporting below.
	}

	// Server not running - report on-disk state
	fmt.Println("Tor Hidden Service: Stopped (server not running)")
	if hostname := readHostnameFile(siteDir); hostname != "" {
		fmt.Printf("  Address: %s\n", hostname)
		return 0
	}
	fmt.Println("  Address: (none - generated on first server start)")
	return 0
}

// torValidate implements `tor validate` per AI.md PART 31
// Checks the tor binary, config, and on-disk key material; when a server is
// running, also cross-checks the live connection via the internal control
// channel.
func torValidate(configDir, dataDir string) int {
	torDir, siteDir := torDirs(configDir, dataDir)
	failures := 0

	cfg, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Config: failed to load: %v\n", err)
		return 1
	}
	fmt.Printf(terminal.StatusIcon(true)+" Config: %s\n", configPath)

	// Tor binary: explicit path from config or auto-detect from PATH
	binary := cfg.Server.Tor.Binary
	if binary != "" {
		if _, err := os.Stat(binary); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Tor binary: %s not found\n", binary)
			failures++
		} else {
			fmt.Printf(terminal.StatusIcon(true)+" Tor binary: %s\n", binary)
		}
	} else if path, err := exec.LookPath("tor"); err != nil {
		fmt.Println(terminal.WarningIcon() + " Tor binary: not found in PATH (server runs without Tor)")
	} else {
		fmt.Printf(terminal.StatusIcon(true)+" Tor binary: %s (auto-detected)\n", path)
	}

	// Data directory and key material
	if _, err := os.Stat(torDir); err != nil {
		fmt.Printf(terminal.WarningIcon()+" Tor data dir: %s (created on first server start)\n", torDir)
	} else {
		fmt.Printf(terminal.StatusIcon(true)+" Tor data dir: %s\n", torDir)
		if _, err := os.Stat(filepath.Join(siteDir, "hs_ed25519_secret_key")); err != nil {
			fmt.Println(terminal.WarningIcon() + " Keys: not generated yet")
		} else if hostname := readHostnameFile(siteDir); hostname == "" {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" Keys: secret key exists but hostname file is missing")
			failures++
		} else {
			fmt.Printf(terminal.StatusIcon(true)+" Keys: %s\n", hostname)
		}
	}

	// Live connection cross-check when a server is running
	if addr := resolveServerAddr(configDir, dataDir); serverReachable(addr) {
		data, err := torControlRequest(http.MethodPost, addr, "/server/tor/validate", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Live connection test: %v\n", err)
			failures++
		} else {
			connected, _ := data["connected"].(bool)
			message, _ := data["message"].(string)
			if connected {
				fmt.Printf(terminal.StatusIcon(true)+" Live connection test: %s\n", message)
			} else {
				fmt.Printf(terminal.WarningIcon()+" Live connection test: %s\n", message)
			}
		}
	}

	if failures > 0 {
		return 1
	}
	fmt.Println("Tor configuration is valid")
	return 0
}

// torRestart implements `tor restart` per AI.md PART 31: instructs the
// running server to restart its own Tor process over the internal control
// channel. Requires a live server per PART 31.
func torRestart(configDir, dataDir string) int {
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	if _, err := torControlRequest(http.MethodPost, addr, "/server/tor/restart", nil); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to restart tor: %v\n", err)
		return 1
	}

	fmt.Println(terminal.StatusIcon(true) + " Tor restarted")
	return 0
}

// torRegenerate implements `tor regenerate` per AI.md PART 31: old keys
// deleted, new .onion generated by the running server process. Requires a
// live server per PART 31.
func torRegenerate(configDir, dataDir string) int {
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	data, err := torControlRequest(http.MethodPost, addr, "/server/tor/regenerate", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to regenerate address: %v\n", err)
		return 1
	}

	onion, _ := data["onion_address"].(string)
	fmt.Printf(terminal.StatusIcon(true)+" New .onion address: %s\n", onion)
	return 0
}

// torVanityStart implements `tor vanity start` per AI.md PART 31: starts the
// search on the running server process and polls status until it finishes.
// Requires a live server per PART 31.
func torVanityStart(configDir, dataDir, prefix string) int {
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	if _, err := torControlRequest(http.MethodPost, addr, "/server/tor/vanity/start", map[string]string{
		"prefix": prefix,
	}); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		return 1
	}

	fmt.Printf("Searching for .onion address with prefix %q (Ctrl+C to cancel)...\n", prefix)
	for {
		time.Sleep(2 * time.Second)

		data, err := torControlRequest(http.MethodGet, addr, "/server/tor/status", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Lost contact with server: %v\n", err)
			return 1
		}

		info, _ := data["info"].(map[string]interface{})
		vanity, _ := info["vanity"].(map[string]interface{})
		active, _ := vanity["active"].(bool)
		attempts, _ := vanity["attempts"].(float64)
		if !active {
			break
		}
		fmt.Printf("  %.0f attempts\n", attempts)
	}

	torDir, _ := torDirs(configDir, dataDir)
	pending := readHostnameFile(filepath.Join(torDir, "vanity_pending"))
	if pending == "" {
		fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" Vanity generation finished without a pending address")
		return 1
	}

	fmt.Printf(terminal.StatusIcon(true)+" Found vanity address: %s\n", pending)
	fmt.Printf("Run '%s tor vanity apply' to activate it\n", filepath.Base(os.Args[0]))
	return 0
}

// torVanityApply implements `tor vanity apply` per AI.md PART 31. Requires a
// live server per PART 31.
func torVanityApply(configDir, dataDir string) int {
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	data, err := torControlRequest(http.MethodPost, addr, "/server/tor/vanity/apply", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		return 1
	}

	onion, _ := data["onion_address"].(string)
	fmt.Printf(terminal.StatusIcon(true)+" Vanity address applied: %s\n", onion)
	return 0
}

// torImportKeys implements `tor import-keys <path>` per AI.md PART 31: keys
// replaced on the running server process, which restarts Tor with the new
// address. Requires a live server per PART 31.
func torImportKeys(configDir, dataDir, keyPath string) int {
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	secretKey, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to read key file: %v\n", err)
		return 1
	}

	data, err := torControlRequest(http.MethodPost, addr, "/server/tor/import-keys", map[string]interface{}{
		"secret_key": secretKey,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to import keys: %v\n", err)
		return 1
	}

	onion, _ := data["onion_address"].(string)
	fmt.Printf(terminal.StatusIcon(true)+" Keys imported - new address: %s\n", onion)
	return 0
}
