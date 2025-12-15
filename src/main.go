// SPDX-License-Identifier: MIT
// Vidveil - Privacy-respecting adult video meta search engine

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server"
	"github.com/apimgr/vidveil/src/services/engines"
	"github.com/apimgr/vidveil/src/services/maintenance"
	"github.com/apimgr/vidveil/src/services/service"
)

var (
	Version   = "0.2.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	args := os.Args[1:]

	// Parse arguments manually per TEMPLATE.md spec
	var (
		configDir   string
		dataDir     string
		address     string
		port        string
		mode        string
		serviceCmd  string
		maintCmd    string
		maintArg    string
		updateCmd   string
		updateArg   string
	)

	i := 0
	for i < len(args) {
		arg := args[i]

		switch arg {
		case "--help", "-h":
			printHelp()
			os.Exit(0)

		case "--version", "-v":
			printVersion()
			os.Exit(0)

		case "--status":
			os.Exit(checkStatus())

		case "--config":
			if i+1 < len(args) {
				i++
				configDir = args[i]
			}

		case "--data":
			if i+1 < len(args) {
				i++
				dataDir = args[i]
			}

		case "--address":
			if i+1 < len(args) {
				i++
				address = args[i]
			}

		case "--port":
			if i+1 < len(args) {
				i++
				port = args[i]
			}

		case "--mode":
			if i+1 < len(args) {
				i++
				mode = args[i]
			}

		case "--service":
			if i+1 < len(args) {
				i++
				serviceCmd = args[i]
			}

		case "--update":
			// TEMPLATE.md PART 14: --update [check|yes|branch {stable|beta|daily}]
			updateCmd = "yes" // Default per spec
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				i++
				updateCmd = args[i]
				if updateCmd == "branch" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					i++
					updateArg = args[i]
				}
			}

		case "--maintenance":
			if i+1 < len(args) {
				i++
				maintCmd = args[i]
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					i++
					maintArg = args[i]
				}
			}

		default:
			// Check for --config=value format
			if strings.HasPrefix(arg, "--config=") {
				configDir = strings.TrimPrefix(arg, "--config=")
			} else if strings.HasPrefix(arg, "--data=") {
				dataDir = strings.TrimPrefix(arg, "--data=")
			} else if strings.HasPrefix(arg, "--address=") {
				address = strings.TrimPrefix(arg, "--address=")
			} else if strings.HasPrefix(arg, "--port=") {
				port = strings.TrimPrefix(arg, "--port=")
			} else if strings.HasPrefix(arg, "--mode=") {
				mode = strings.TrimPrefix(arg, "--mode=")
			}
		}
		i++
	}

	// Handle service command
	if serviceCmd != "" {
		handleServiceCommand(serviceCmd)
		return
	}

	// Handle update command (TEMPLATE.md PART 14)
	if updateCmd != "" {
		handleUpdateCommand(updateCmd, updateArg)
		return
	}

	// Handle maintenance command
	if maintCmd != "" {
		// --maintenance update is alias for --update yes per TEMPLATE.md
		if maintCmd == "update" {
			handleUpdateCommand("yes", "")
			return
		}
		handleMaintenanceCommand(maintCmd, maintArg)
		return
	}

	// Check for environment variables (init only per BASE.md)
	if configDir == "" && os.Getenv("CONFIG_DIR") != "" {
		configDir = os.Getenv("CONFIG_DIR")
	}
	if dataDir == "" && os.Getenv("DATA_DIR") != "" {
		dataDir = os.Getenv("DATA_DIR")
	}
	if port == "" && os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	if address == "" && os.Getenv("LISTEN") != "" {
		address = os.Getenv("LISTEN")
	}

	// MODE env var is runtime - always checked per BASE.md
	// Priority: CLI flag > env var > config file
	if mode == "" && os.Getenv("MODE") != "" {
		mode = os.Getenv("MODE")
	}

	// Load configuration
	cfg, configPath, err := config.Load(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Override with command line flags
	if address != "" {
		cfg.Server.Address = address
	}
	if port != "" {
		cfg.Server.Port = port
	}

	// Apply mode (CLI > env > config, normalized)
	if mode != "" {
		cfg.Server.Mode = config.NormalizeMode(mode)
	} else if cfg.Server.Mode == "" {
		cfg.Server.Mode = "production"
	} else {
		cfg.Server.Mode = config.NormalizeMode(cfg.Server.Mode)
	}

	// Initialize search engines
	engineMgr := engines.NewManager(cfg)
	engineMgr.InitializeEngines()

	// Create server
	srv := server.New(cfg, engineMgr)

	// Start live config watcher per TEMPLATE.md PART 1 NON-NEGOTIABLE
	configWatcher := config.NewWatcher(configPath, cfg)
	configWatcher.OnReload(func(newCfg *config.Config) {
		// Config has been reloaded - the shared cfg pointer is already updated
		// Additional reload actions can be added here if needed
	})
	configWatcher.Start()
	defer configWatcher.Stop()

	// Start server in goroutine
	go func() {
		// Build listen address properly handling IPv6
		listenAddr := cfg.Server.Address + ":" + cfg.Server.Port
		displayAddr := getDisplayAddress(cfg)

		fmt.Printf("\n")

		// Mode-specific startup output per BASE.md spec lines 375-392
		if cfg.IsDevelopmentMode() {
			fmt.Printf("🔧 Vidveil v%s [DEVELOPMENT MODE]\n", Version)
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Printf("⚠️  Debug endpoints enabled\n")
			fmt.Printf("⚠️  Verbose error messages enabled\n")
			fmt.Printf("⚠️  Template caching disabled\n")
			fmt.Printf("   Mode: development\n")
		} else {
			fmt.Printf("🚀 Vidveil v%s\n", Version)
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Printf("   Mode: production\n")
		}

		fmt.Printf("🌐 Server:  http://%s\n", displayAddr)
		fmt.Printf("📝 Config:  %s\n", configPath)
		fmt.Printf("📚 Engines: %d enabled\n", engineMgr.EnabledCount())

		if cfg.Search.Tor.Enabled {
			fmt.Printf("🧅 Tor:     %s\n", cfg.Search.Tor.Proxy)
		}

		if cfg.IsDevelopmentMode() {
			fmt.Printf("🔧 Debug:   http://%s/debug/pprof/\n", displayAddr)
		}

		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Press Ctrl+C to stop\n")
		fmt.Printf("\n")

		if err := srv.ListenAndServe(listenAddr); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "❌ Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-quit
	fmt.Printf("\n🛑 Received %v, shutting down gracefully...\n", sig)

	// Graceful shutdown with timeout (30 seconds per BASE.md)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Shutdown error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Server stopped")
}

func printHelp() {
	fmt.Printf(`Vidveil v%s - Privacy-respecting adult video meta search engine

Usage: vidveil [options]

Options:
  --help              Show this help message
  --version           Show version information
  --status            Check server status and health
  --mode <mode>       Set application mode (prod/production or dev/development)
  --config <dir>      Set configuration directory
  --data <dir>        Set data directory
  --address <addr>    Set listen address
  --port <port>       Set port (e.g., 8888 or 80,443)

Update (TEMPLATE.md PART 14):
  --update                Check and perform in-place update with restart
  --update yes            Same as --update (default)
  --update check          Check for updates without installing (no privileges required)
  --update branch <name>  Set update branch (stable, beta, daily)

Service Management:
  --service start         Start the service
  --service stop          Stop the service
  --service restart       Restart the service
  --service reload        Reload configuration
  --service --install     Install as system service
  --service --uninstall   Uninstall system service
  --service --disable     Disable the service
  --service --help        Show service help

Maintenance:
  --maintenance backup [file]     Create backup
  --maintenance restore [file]    Restore from backup
  --maintenance update            Alias for --update yes
  --maintenance mode <on|off>     Enable/disable maintenance mode

Environment Variables:
  MODE                Application mode (runtime, always checked)

  Initialization only (used once on first run):
  CONFIG_DIR          Configuration directory
  DATA_DIR            Data directory
  LOG_DIR             Log directory
  PORT                Server port
  LISTEN              Listen address
  APPLICATION_NAME    Application title
  APPLICATION_TAGLINE Application description

Default behavior:
  Running without arguments initializes (if needed) and starts the server.

Documentation: https://vidveil.apimgr.us
Source: https://github.com/apimgr/vidveil
`, Version)
}

func printVersion() {
	fmt.Printf("Vidveil v%s\n", Version)
	fmt.Printf("Build: %s\n", BuildTime)
	fmt.Printf("Commit: %s\n", GitCommit)
}

func checkStatus() int {
	// Get paths
	paths := config.GetPaths("", "")

	// Try to load config to check if initialized
	cfg, _, err := config.Load("", "")
	if err != nil {
		fmt.Println("❌ Status: Not initialized")
		fmt.Printf("   Config dir: %s\n", paths.Config)
		return 1
	}

	// Try to connect to the server
	addr := net.JoinHostPort("127.0.0.1", cfg.Server.Port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		fmt.Println("⚠️  Status: Stopped")
		fmt.Printf("   Port: %s (not listening)\n", cfg.Server.Port)
		return 1
	}
	conn.Close()

	// Server is running - try health check
	healthURL := fmt.Sprintf("http://%s/healthz", addr)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Println("⚠️  Status: Running (health check failed)")
		fmt.Printf("   Port: %s\n", cfg.Server.Port)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("✅ Status: Running")
		fmt.Printf("   Port: %s\n", cfg.Server.Port)
		fmt.Printf("   FQDN: %s\n", cfg.Server.FQDN)
		return 0
	}

	fmt.Println("⚠️  Status: Running (unhealthy)")
	fmt.Printf("   Port: %s\n", cfg.Server.Port)
	return 1
}

func handleServiceCommand(cmd string) {
	svc, err := service.New("vidveil", "Vidveil", "Privacy-respecting adult video meta search engine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Service error: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "start":
		fmt.Println("Starting Vidveil service...")
		if err := svc.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to start: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Service started")

	case "stop":
		fmt.Println("Stopping Vidveil service...")
		if err := svc.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to stop: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Service stopped")

	case "restart":
		fmt.Println("Restarting Vidveil service...")
		if err := svc.Restart(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to restart: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Service restarted")

	case "reload":
		fmt.Println("Reloading Vidveil configuration...")
		if err := svc.Reload(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to reload: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Configuration reloaded")

	case "--install":
		fmt.Println("Installing Vidveil as system service...")
		if err := svc.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to install: %v\n", err)
			os.Exit(1)
		}

	case "--uninstall":
		fmt.Println("Uninstalling Vidveil system service...")
		if err := svc.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to uninstall: %v\n", err)
			os.Exit(1)
		}

	case "--disable":
		fmt.Println("Disabling Vidveil service...")
		if err := svc.Disable(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to disable: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Service disabled")

	case "--help":
		fmt.Println(`Service Management Commands:

  vidveil --service start         Start the service
  vidveil --service stop          Stop the service
  vidveil --service restart       Restart the service
  vidveil --service reload        Reload configuration
  vidveil --service --install     Install as system service
  vidveil --service --uninstall   Uninstall system service
  vidveil --service --disable     Disable the service

Supported service managers:
  - systemd (Linux)
  - runit (Linux)
  - launchd (macOS)
  - Windows Service Manager
  - BSD rc.d`)

	default:
		fmt.Printf("❌ Unknown service command: %s\n", cmd)
		fmt.Println("   Run 'vidveil --service --help' for available commands")
		os.Exit(1)
	}
}

// handleUpdateCommand implements TEMPLATE.md PART 14 --update command
func handleUpdateCommand(cmd, arg string) {
	maint := maintenance.New("", "", Version)

	switch cmd {
	case "check":
		// Check for updates without installing (no privileges required)
		fmt.Println("Checking for updates...")
		fmt.Printf("Current version: %s\n", Version)

		info, err := maint.CheckUpdate()
		if err != nil {
			// HTTP 404 means no updates available per TEMPLATE.md
			if strings.Contains(err.Error(), "404") {
				fmt.Println("✅ Already up to date (no newer release found)")
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "❌ Update check failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Latest version:  %s\n", info.LatestVersion)

		if info.UpdateAvailable {
			fmt.Println("\n📦 Update available!")
			fmt.Printf("   Release: %s\n", info.ReleaseURL)
			fmt.Println("\n   Run 'vidveil --update' to download and install")
		} else {
			fmt.Println("✅ Already up to date")
		}
		os.Exit(0)

	case "yes", "":
		// Check and perform in-place update with restart
		fmt.Println("Checking for updates...")
		fmt.Printf("Current version: %s\n", Version)

		info, err := maint.CheckUpdate()
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				fmt.Println("✅ Already up to date")
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "❌ Update check failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Latest version:  %s\n", info.LatestVersion)

		if info.UpdateAvailable {
			fmt.Println("\n📦 Update available!")
			fmt.Printf("   Release: %s\n", info.ReleaseURL)

			if info.DownloadURL != "" {
				fmt.Println("\nApplying update...")
				if err := maint.ApplyUpdate(info.DownloadURL); err != nil {
					fmt.Fprintf(os.Stderr, "❌ Update failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("✅ Update successful! Please restart the application.")
			}
		} else {
			fmt.Println("✅ Already up to date")
		}
		os.Exit(0)

	case "branch":
		// Set update branch (stable, beta, daily)
		validBranches := map[string]bool{"stable": true, "beta": true, "daily": true}
		if !validBranches[arg] {
			fmt.Printf("❌ Invalid branch: %s\n", arg)
			fmt.Println("   Valid branches: stable, beta, daily")
			os.Exit(1)
		}

		if err := maint.SetUpdateBranch(arg); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to set branch: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Update branch set to: %s\n", arg)
		os.Exit(0)

	default:
		fmt.Printf("❌ Unknown update command: %s\n", cmd)
		fmt.Println(`
Update Commands (TEMPLATE.md PART 14):
  vidveil --update              Check and perform in-place update with restart
  vidveil --update yes          Same as --update (default)
  vidveil --update check        Check for updates without installing
  vidveil --update branch <name>  Set update branch (stable, beta, daily)

Update Branches:
  stable (default)  Release builds (v*, *.*.*)
  beta              Pre-release builds (*-beta)
  daily             Daily builds (YYYYMMDDHHMM)`)
		os.Exit(1)
	}
}

func handleMaintenanceCommand(cmd, arg string) {
	maint := maintenance.New("", "", Version)

	switch cmd {
	case "backup":
		fmt.Println("Creating backup...")
		if err := maint.Backup(arg); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Backup failed: %v\n", err)
			os.Exit(1)
		}

	case "restore":
		if arg == "" {
			fmt.Println("Restoring from most recent backup...")
		} else {
			fmt.Printf("Restoring from %s...\n", arg)
		}
		if err := maint.Restore(arg); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Restore failed: %v\n", err)
			os.Exit(1)
		}

	case "mode":
		if arg == "" {
			fmt.Println("❌ Missing mode argument")
			fmt.Println("   Usage: vidveil --maintenance mode <on|off>")
			os.Exit(1)
		}

		// Parse boolean per BASE.md (1, yes, true, enable, enabled, on)
		enabled := false
		switch strings.ToLower(arg) {
		case "1", "yes", "true", "enable", "enabled", "on":
			enabled = true
		case "0", "no", "false", "disable", "disabled", "off":
			enabled = false
		default:
			fmt.Printf("❌ Invalid mode value: %s\n", arg)
			fmt.Println("   Valid values: on, off, true, false, yes, no, enable, disable")
			os.Exit(1)
		}

		if err := maint.SetMaintenanceMode(enabled); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("❌ Unknown maintenance command: %s\n", cmd)
		fmt.Println(`
Maintenance Commands:
  vidveil --maintenance backup [file]     Create backup
  vidveil --maintenance restore [file]    Restore from backup
  vidveil --maintenance update            Check and apply updates
  vidveil --maintenance mode <on|off>     Enable/disable maintenance mode`)
		os.Exit(1)
	}
}

func getDisplayAddress(cfg *config.Config) string {
	// Per TEMPLATE.md PART 13: Never show 0.0.0.0, 127.0.0.1, localhost, etc.
	return net.JoinHostPort(config.GetDisplayHost(cfg), cfg.Server.Port)
}
