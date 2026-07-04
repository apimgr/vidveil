// SPDX-License-Identifier: MIT
// AI.md PART 31: Tor CLI commands
// Tor is configured via server.yml and CLI only. No REST API for Tor configuration.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

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

// healthzTor holds the Tor fields parsed from the running server's /healthz JSON
type healthzTor struct {
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
}

// healthzResponse holds the /healthz fields the tor CLI needs
type healthzResponse struct {
	Uptime   string `json:"uptime"`
	Mode     string `json:"mode"`
	Features struct {
		Tor healthzTor `json:"tor"`
	} `json:"features"`
}

// queryHealthz fetches /healthz JSON from the running server, or nil if not running
func queryHealthz(configDir, dataDir string) *healthzResponse {
	cfg, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return nil
	}

	addr := net.JoinHostPort("127.0.0.1", cfg.Server.Port)
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/healthz", addr), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var health healthzResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil
	}
	return &health
}

// readHostnameFile reads the stored .onion address from {data_dir}/tor/site/hostname
func readHostnameFile(siteDir string) string {
	data, err := os.ReadFile(filepath.Join(siteDir, "hostname"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// torStatus implements `tor status` per AI.md PART 31
// Queries the running server's /healthz; falls back to on-disk key state
func torStatus(configDir, dataDir string) int {
	_, siteDir := torDirs(configDir, dataDir)

	if health := queryHealthz(configDir, dataDir); health != nil {
		t := health.Features.Tor
		state := "Disabled"
		if t.Running {
			state = "Connected"
		} else if t.Status == "starting" {
			state = "Starting"
		} else if t.Enabled {
			state = "Disconnected"
		}
		fmt.Printf("Tor Hidden Service: %s\n", state)
		if t.Hostname != "" {
			fmt.Printf("  Address: %s\n", t.Hostname)
		}
		fmt.Printf("  Status: %s\n", t.Status)
		return 0
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
// Checks the tor binary, config, and on-disk key material
func torValidate(configDir, dataDir string) int {
	torDir, siteDir := torDirs(configDir, dataDir)
	failures := 0

	cfg, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Config: failed to load: %v\n", err)
		return 1
	}
	fmt.Printf("✅ Config: %s\n", configPath)

	// Tor binary: explicit path from config or auto-detect from PATH
	binary := cfg.Server.Tor.Binary
	if binary != "" {
		if _, err := os.Stat(binary); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Tor binary: %s not found\n", binary)
			failures++
		} else {
			fmt.Printf("✅ Tor binary: %s\n", binary)
		}
	} else if path, err := exec.LookPath("tor"); err != nil {
		fmt.Println("⚠️  Tor binary: not found in PATH (server runs without Tor)")
	} else {
		fmt.Printf("✅ Tor binary: %s (auto-detected)\n", path)
	}

	// Data directory and key material
	if _, err := os.Stat(torDir); err != nil {
		fmt.Printf("⚠️  Tor data dir: %s (created on first server start)\n", torDir)
	} else {
		fmt.Printf("✅ Tor data dir: %s\n", torDir)
		if _, err := os.Stat(filepath.Join(siteDir, "hs_ed25519_secret_key")); err != nil {
			fmt.Println("⚠️  Keys: not generated yet")
		} else if hostname := readHostnameFile(siteDir); hostname == "" {
			fmt.Fprintln(os.Stderr, "❌ Keys: secret key exists but hostname file is missing")
			failures++
		} else {
			fmt.Printf("✅ Keys: %s\n", hostname)
		}
	}

	if failures > 0 {
		return 1
	}
	fmt.Println("Tor configuration is valid")
	return 0
}

// signalTorProcess sends SIGTERM to the Tor PID from {data_dir}/tor/tor.pid
// The server's process monitor restarts Tor automatically within 30 seconds
func signalTorProcess(torDir string) error {
	data, err := os.ReadFile(filepath.Join(torDir, "tor.pid"))
	if err != nil {
		return fmt.Errorf("tor.pid not found (is the server running with Tor?): %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid tor.pid contents: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("tor process %d not found: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Windows does not support SIGTERM - fall back to Kill
		if killErr := proc.Kill(); killErr != nil {
			return fmt.Errorf("failed to stop tor process %d: %w", pid, err)
		}
	}
	return nil
}

// torRestart implements `tor restart` per AI.md PART 31
func torRestart(configDir, dataDir string) int {
	torDir, _ := torDirs(configDir, dataDir)

	if err := signalTorProcess(torDir); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		return 1
	}

	fmt.Println("✅ Tor process stopped - server monitor restarts it within 30 seconds")
	return 0
}

// restartTorIfRunning signals the running Tor process after a key change so the
// server monitor restarts it with the new keys; no-op when Tor is not running
func restartTorIfRunning(torDir string) {
	if _, err := os.Stat(filepath.Join(torDir, "tor.pid")); err != nil {
		return
	}
	if err := signalTorProcess(torDir); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not restart Tor: %v\n", err)
		return
	}
	fmt.Println("Tor restarting - server monitor applies the new address within 30 seconds")
}

// torRegenerate implements `tor regenerate` per AI.md PART 31:
// old keys deleted from {data_dir}/tor/site/, new .onion generated
func torRegenerate(configDir, dataDir string) int {
	paths := config.GetAppPaths(configDir, dataDir)
	torDir, siteDir := torDirs(configDir, dataDir)

	if err := os.MkdirAll(siteDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create %s: %v\n", siteDir, err)
		return 1
	}

	svc := tor.NewTorService(paths.Data, nil)
	if err := svc.RegenerateAddress(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to regenerate address: %v\n", err)
		return 1
	}

	fmt.Printf("✅ New .onion address: %s\n", svc.GetOnionAddress())
	restartTorIfRunning(torDir)
	return 0
}

// torVanityStart implements `tor vanity start` per AI.md PART 31
// Runs the search in the foreground and saves matching keys to vanity_pending/
func torVanityStart(configDir, dataDir, prefix string) int {
	paths := config.GetAppPaths(configDir, dataDir)
	_, siteDir := torDirs(configDir, dataDir)

	if err := os.MkdirAll(siteDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create %s: %v\n", siteDir, err)
		return 1
	}

	svc := tor.NewTorService(paths.Data, nil)
	if err := svc.GenerateVanityAddress(prefix); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		return 1
	}

	fmt.Printf("Searching for .onion address with prefix %q (Ctrl+C to cancel)...\n", prefix)
	for {
		time.Sleep(2 * time.Second)
		status := svc.GetVanityStatus()
		if status == nil {
			fmt.Fprintln(os.Stderr, "❌ Vanity generation stopped unexpectedly")
			return 1
		}
		if !status.Active {
			break
		}
		fmt.Printf("  %d attempts (%s elapsed)\n", status.Attempts, time.Since(status.StartTime).Round(time.Second))
	}

	torDir, _ := torDirs(configDir, dataDir)
	pending := readHostnameFile(filepath.Join(torDir, "vanity_pending"))
	if pending == "" {
		fmt.Fprintln(os.Stderr, "❌ Vanity generation finished without a pending address")
		return 1
	}

	fmt.Printf("✅ Found vanity address: %s\n", pending)
	fmt.Printf("Run '%s tor vanity apply' to activate it\n", filepath.Base(os.Args[0]))
	return 0
}

// torVanityApply implements `tor vanity apply` per AI.md PART 31
func torVanityApply(configDir, dataDir string) int {
	paths := config.GetAppPaths(configDir, dataDir)
	torDir, siteDir := torDirs(configDir, dataDir)

	svc := tor.NewTorService(paths.Data, nil)
	if err := svc.ApplyVanityAddress(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		return 1
	}

	fmt.Printf("✅ Vanity address applied: %s\n", readHostnameFile(siteDir))
	restartTorIfRunning(torDir)
	return 0
}

// torImportKeys implements `tor import-keys <path>` per AI.md PART 31:
// keys replaced in {data_dir}/tor/site/, Tor restarts with new address
func torImportKeys(configDir, dataDir, keyPath string) int {
	paths := config.GetAppPaths(configDir, dataDir)
	torDir, siteDir := torDirs(configDir, dataDir)

	secretKey, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to read key file: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(siteDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create %s: %v\n", siteDir, err)
		return 1
	}

	svc := tor.NewTorService(paths.Data, nil)
	if err := svc.ImportKeys(secretKey); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to import keys: %v\n", err)
		return 1
	}

	fmt.Printf("✅ Keys imported - new address: %s\n", svc.GetOnionAddress())
	restartTorIfRunning(torDir)
	return 0
}
