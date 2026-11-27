// SPDX-License-Identifier: MIT
// AI.md PART 31: Tor CLI commands
// Tor is configured via server.yml and the CLI only; there is no PUBLIC REST
// API for Tor configuration. Mutating commands reach the server-owned Tor
// process over the internal loopback-only control channel exposed at
// /server/tor/* (src/server/tor_control.go) — never by touching Tor state
// directly, since the running server binary is the sole owner of that
// process.
package app

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
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/config"
)

// errNoRunningServer is the exact message required by AI.md PART 31 for
// every mutating tor subcommand when no running server can be reached.
const errNoRunningServer = "Error: no running server detected — start the server first"

// handleTorCommand dispatches the `tor` CLI subcommands per AI.md PART 31:
// --maintenance tor status | validate | restart | regenerate | vanity start | vanity stop | vanity apply | import-keys <path>
func handleTorCommand(args []string, configDir, dataDir string) int {
	if len(args) == 0 {
		printTorHelp()
		return 1
	}
	yes := false
	if args[len(args)-1] == "--yes" {
		yes = true
		args = args[:len(args)-1]
		if len(args) == 0 {
			printTorHelp()
			return 2
		}
	}

	switch args[0] {
	case "status":
		return torStatus(configDir, dataDir)
	case "validate":
		return torValidate(configDir, dataDir)
	case "restart":
		return torRestart(configDir, dataDir)
	case "regenerate":
		return torRegenerate(configDir, dataDir, yes)
	case "vanity":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: tor vanity {start <prefix>|apply}")
			return 1
		}
		switch args[1] {
		case "start":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Usage: tor vanity start <prefix> [--workers N]")
				return 1
			}
			workers := 0
			for index := 3; index < len(args); index++ {
				value := ""
				if strings.HasPrefix(args[index], "--workers=") {
					value = strings.TrimPrefix(args[index], "--workers=")
				} else if args[index] == "--workers" && index+1 < len(args) {
					index++
					value = args[index]
				} else {
					fmt.Fprintf(os.Stderr, "Unknown vanity start option: %s\n", args[index])
					return 2
				}
				parsed, err := strconv.Atoi(value)
				if err != nil || parsed < 1 {
					fmt.Fprintln(os.Stderr, "workers must be a positive integer")
					return 2
				}
				workers = parsed
			}
			return torVanityStart(configDir, dataDir, args[2], workers)
		case "apply":
			address := ""
			if len(args) > 2 {
				address = args[2]
			}
			return torVanityApply(configDir, dataDir, yes, address)
		case "stop":
			return torVanityStop(configDir, dataDir)
		default:
			fmt.Fprintf(os.Stderr, "Unknown vanity command: %s\nUsage: tor vanity {start <prefix>|apply}\n", args[1])
			return 1
		}
	case "import-keys":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: tor import-keys <path>")
			return 1
		}
		return torImportKeys(configDir, dataDir, args[1], yes)
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
  %s --maintenance tor status              - View Tor hidden service status
  %s --maintenance tor validate            - Validate Tor configuration
  %s --maintenance tor restart             - Restart the Tor process
  %s --maintenance tor regenerate          - Regenerate the .onion address (new keys)
  %s --maintenance tor vanity start <pfx> [--workers N] - Start vanity address search
  %s --maintenance tor vanity stop         - Stop vanity search and keep found candidates
  %s --maintenance tor vanity apply [address] - Apply a pending vanity address
  %s --maintenance tor import-keys <path>  - Import an existing key directory

Tor is configured via server.yml and CLI only.
`, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName)
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

type healthzI2P struct {
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
	Provider string `json:"provider"`
}

// healthzResponse holds the /server/healthz fields the CLI needs for
// `tor status` and `--status`.
type healthzResponse struct {
	Uptime   string `json:"uptime"`
	Mode     string `json:"mode"`
	Features struct {
		Tor healthzTor `json:"tor"`
		I2P healthzI2P `json:"i2p"`
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	_ = conn.Close()
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
			bootstrap, _ := data["bootstrap_percent"].(float64)
			version, _ := data["version"].(string)
			onion, _ := data["onion_address"].(string)

			state := "Disabled"
			switch {
			case running:
				state = "Connected"
			case bootstrap > 0 && bootstrap < 100:
				state = "Starting"
			case onion != "":
				state = "Disconnected"
			}
			fmt.Printf("Tor Hidden Service: %s\n", state)
			if onion != "" {
				fmt.Printf("  Address: %s\n", onion)
			}
			if version != "" {
				fmt.Printf("  Version: %s\n", version)
			}
			if bootstrap > 0 && bootstrap < 100 {
				fmt.Printf("  Bootstrap: %.0f%%\n", bootstrap)
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
func torRegenerate(configDir, dataDir string, yes bool) int {
	if !confirmTorAction(configDir, dataDir, "cli.tor.confirm_regenerate", yes) {
		return 1
	}
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	data, err := torControlRequest(http.MethodPost, addr, "/server/tor/regenerate", map[string]bool{"confirm": true})
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
func torVanityStart(configDir, dataDir, prefix string, requestedWorkers ...int) int {
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	workers := 0
	if len(requestedWorkers) > 0 {
		workers = requestedWorkers[0]
	}
	if _, err := torControlRequest(http.MethodPost, addr, "/server/tor/vanity/start", map[string]interface{}{
		"prefix":  prefix,
		"workers": workers,
	}); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		return 1
	}

	fmt.Printf("Searching for .onion address with prefix %q (Ctrl+C to cancel)...\n", prefix)
	pendingAddress := ""
	for {
		time.Sleep(2 * time.Second)

		data, err := torControlRequest(http.MethodGet, addr, "/server/tor/status", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Lost contact with server: %v\n", err)
			return 1
		}

		vanity, _ := data["vanity"].(map[string]interface{})
		if vanity == nil {
			info, _ := data["info"].(map[string]interface{})
			vanity, _ = info["vanity"].(map[string]interface{})
		}
		active, _ := vanity["active"].(bool)
		if state, _ := vanity["state"].(string); state != "" {
			active = state == "running"
		}
		attempts, _ := vanity["attempts"].(float64)
		if candidates, ok := vanity["candidates"].([]interface{}); ok && len(candidates) > 0 {
			pendingAddress, _ = candidates[0].(string)
		}
		if !active {
			break
		}
		fmt.Printf("  %.0f attempts\n", attempts)
	}

	torDir, _ := torDirs(configDir, dataDir)
	pending := pendingAddress
	if pending == "" {
		pending = readHostnameFile(filepath.Join(torDir, "vanity_pending"))
	}
	if pending == "" {
		fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" Vanity generation finished without a pending address")
		return 1
	}

	fmt.Printf(terminal.StatusIcon(true)+" Found vanity address: %s\n", pending)
	fmt.Printf("Run '%s --maintenance tor vanity apply' to activate it\n", filepath.Base(os.Args[0]))
	return 0
}

// torVanityApply implements `tor vanity apply` per AI.md PART 31. Requires a
// live server per PART 31.
func torVanityApply(configDir, dataDir string, yes bool, address ...string) int {
	if !confirmTorAction(configDir, dataDir, "cli.tor.confirm_vanity_apply", yes) {
		return 1
	}
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	wanted := ""
	if len(address) > 0 {
		wanted = address[0]
	}
	data, err := torControlRequest(http.MethodPost, addr, "/server/tor/vanity/apply", map[string]interface{}{"address": wanted, "confirm": true})
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		return 1
	}

	onion, _ := data["onion_address"].(string)
	fmt.Printf(terminal.StatusIcon(true)+" Vanity address applied: %s\n", onion)
	return 0
}

func torVanityStop(configDir, dataDir string) int {
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}
	data, err := torControlRequest(http.MethodPost, addr, "/server/tor/vanity/stop", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		return 1
	}
	stopped, _ := data["stopped"].(bool)
	if !stopped {
		fmt.Println("No vanity search was running")
		return 0
	}
	fmt.Println(terminal.StatusIcon(true) + " Vanity search stopped")
	return 0
}

// torImportKeys implements `tor import-keys <path>` per AI.md PART 31: keys
// replaced on the running server process, which restarts Tor with the new
// address. Requires a live server per PART 31.
func torImportKeys(configDir, dataDir, keyPath string, yes bool) int {
	if !filepath.IsAbs(keyPath) {
		fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" Key directory path must be absolute")
		return 1
	}
	info, err := os.Stat(keyPath)
	if err != nil || !info.IsDir() {
		fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" Key directory is unavailable")
		return 1
	}
	if !confirmTorAction(configDir, dataDir, "cli.tor.confirm_import", yes) {
		return 1
	}
	addr, ok := requireRunningServer(configDir, dataDir)
	if !ok {
		return 1
	}

	data, err := torControlRequest(http.MethodPost, addr, "/server/tor/import-keys", map[string]interface{}{
		"path":    keyPath,
		"confirm": true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to import keys: %v\n", err)
		return 1
	}

	onion, _ := data["onion_address"].(string)
	fmt.Printf(terminal.StatusIcon(true)+" Keys imported - new address: %s\n", onion)
	return 0
}

func confirmTorAction(configDir, dataDir, key string, yes bool) bool {
	if yes {
		return true
	}
	locale := cliOutputLocale("", configDir, dataDir)
	if !promptYesNo(serviceText(locale, key)) {
		fmt.Fprintln(os.Stderr, serviceText(locale, "cli.tor.confirmation_cancelled"))
		return false
	}
	return true
}
