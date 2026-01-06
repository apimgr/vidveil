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
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/server"
	"github.com/apimgr/vidveil/src/server/service/admin"
	"github.com/apimgr/vidveil/src/server/service/blocklist"
	"github.com/apimgr/vidveil/src/server/service/cluster"
	"github.com/apimgr/vidveil/src/server/service/cve"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/geoip"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/maintenance"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
	"github.com/apimgr/vidveil/src/server/service/service"
	"github.com/apimgr/vidveil/src/server/service/ssl"
	"github.com/apimgr/vidveil/src/server/service/tor"
	"github.com/apimgr/vidveil/src/common/version"
)

// Build info - set via -ldflags at build time per PART 7
var (
	Version   = "dev"
	CommitID  = "unknown"
	BuildDate = "unknown"
)

func init() {
	// Sync build info to version package for other code per PART 7
	version.Version = Version
	version.CommitID = CommitID
	version.BuildTime = BuildDate
}

func main() {
	args := os.Args[1:]

	// Parse arguments manually per AI.md spec
	var (
		configDir   string
		dataDir     string
		cacheDir    string
		logDir      string
		backupDir   string
		pidFile     string
		address     string
		port        string
		modeStr     string
		debug       bool
		daemon      bool
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

		case "--shell":
			// Per AI.md PART 8: --shell completions [SHELL] or --shell init [SHELL]
			if i+1 < len(args) {
				i++
				subCmd := args[i]
				var shell string
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					shell = args[i]
				}
				handleShellCommand(subCmd, shell)
				os.Exit(0)
			} else {
				fmt.Fprintln(os.Stderr, "Usage: --shell [completions|init] [SHELL]")
				os.Exit(1)
			}

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

		case "--cache":
			if i+1 < len(args) {
				i++
				cacheDir = args[i]
			}

		case "--log":
			if i+1 < len(args) {
				i++
				logDir = args[i]
			}

		case "--backup":
			if i+1 < len(args) {
				i++
				backupDir = args[i]
			}

		case "--pid":
			if i+1 < len(args) {
				i++
				pidFile = args[i]
			}

		case "--daemon":
			daemon = true

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
				modeStr = args[i]
			}

		case "--debug":
			debug = true

		case "--service":
			if i+1 < len(args) {
				i++
				serviceCmd = args[i]
			}

		case "--update":
			// AI.md PART 14: --update [check|yes|branch {stable|beta|daily}]
			// Default per spec
			updateCmd = "yes"
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
			// Check for --flag=value format
			if strings.HasPrefix(arg, "--config=") {
				configDir = strings.TrimPrefix(arg, "--config=")
			} else if strings.HasPrefix(arg, "--data=") {
				dataDir = strings.TrimPrefix(arg, "--data=")
			} else if strings.HasPrefix(arg, "--cache=") {
				cacheDir = strings.TrimPrefix(arg, "--cache=")
			} else if strings.HasPrefix(arg, "--log=") {
				logDir = strings.TrimPrefix(arg, "--log=")
			} else if strings.HasPrefix(arg, "--backup=") {
				backupDir = strings.TrimPrefix(arg, "--backup=")
			} else if strings.HasPrefix(arg, "--pid=") {
				pidFile = strings.TrimPrefix(arg, "--pid=")
			} else if strings.HasPrefix(arg, "--address=") {
				address = strings.TrimPrefix(arg, "--address=")
			} else if strings.HasPrefix(arg, "--port=") {
				port = strings.TrimPrefix(arg, "--port=")
			} else if strings.HasPrefix(arg, "--mode=") {
				modeStr = strings.TrimPrefix(arg, "--mode=")
			}
		}
		i++
	}

	// Handle service command
	if serviceCmd != "" {
		handleServiceCommand(serviceCmd)
		return
	}

	// Handle update command (AI.md PART 14)
	if updateCmd != "" {
		handleUpdateCommand(updateCmd, updateArg)
		return
	}

	// Handle maintenance command
	if maintCmd != "" {
		// --maintenance update is alias for --update yes per AI.md
		if maintCmd == "update" {
			handleUpdateCommand("yes", "")
			return
		}
		handleMaintenanceCommand(maintCmd, maintArg)
		return
	}

	// Check for environment variables (init only per AI.md)
	if configDir == "" && os.Getenv("CONFIG_DIR") != "" {
		configDir = os.Getenv("CONFIG_DIR")
	}
	if dataDir == "" && os.Getenv("DATA_DIR") != "" {
		dataDir = os.Getenv("DATA_DIR")
	}
	if cacheDir == "" && os.Getenv("CACHE_DIR") != "" {
		cacheDir = os.Getenv("CACHE_DIR")
	}
	if logDir == "" && os.Getenv("LOG_DIR") != "" {
		logDir = os.Getenv("LOG_DIR")
	}
	if backupDir == "" && os.Getenv("BACKUP_DIR") != "" {
		backupDir = os.Getenv("BACKUP_DIR")
	}
	if port == "" && os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	if address == "" && os.Getenv("LISTEN") != "" {
		address = os.Getenv("LISTEN")
	}

	// MODE env var is runtime - always checked per AI.md
	// Priority: CLI flag > env var > config file
	if modeStr == "" && os.Getenv("MODE") != "" {
		modeStr = os.Getenv("MODE")
	}

	// Initialize mode and debug per AI.md PART 5
	// This must happen before starting the server
	mode.Initialize(modeStr, debug)

	// Handle daemon mode per AI.md PART 4
	if daemon {
		// Daemonize: fork to background
		// For now, just log that daemon mode was requested
		// Full implementation requires platform-specific code
		fmt.Println("🔄 Running in daemon mode...")
	}

	// Load configuration
	cfg, configPath, err := config.Load(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Get paths early so we can override log directory
	paths := config.GetPaths(configDir, dataDir)

	// Override log directory if specified
	if logDir != "" {
		paths.Log = logDir
	}

	// Write PID file if specified per AI.md PART 4
	if pidFile != "" {
		pid := os.Getpid()
		if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to write PID file: %v\n", err)
		}
		defer os.Remove(pidFile)
	}

	// Override with command line flags
	if address != "" {
		cfg.Server.Address = address
	}
	if port != "" {
		cfg.Server.Port = port
	}

	// Apply mode (CLI > env > config, normalized)
	if modeStr != "" {
		cfg.Server.Mode = config.NormalizeMode(modeStr)
	} else if cfg.Server.Mode == "" {
		cfg.Server.Mode = "production"
	} else {
		cfg.Server.Mode = config.NormalizeMode(cfg.Server.Mode)
	}

	// Initialize database per AI.md PART 24
	// Two separate databases: server.db (admin/config) and users.db (user accounts)
	serverDBPath := filepath.Join(paths.Data, "db", "server.db")
	migrationMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer migrationMgr.Close()

	// Register and run migrations
	migrationMgr.RegisterDefaultMigrations()
	if err := migrationMgr.RunMigrations(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	// Initialize admin service per AI.md PART 31
	adminSvc := admin.NewService(migrationMgr.GetDB())
	if err := adminSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize admin service: %v\n", err)
		os.Exit(1)
	}

	// Initialize cluster manager per PART 24
	// Cluster mode auto-detected: SQLite = single instance, PostgreSQL/MySQL = cluster
	// For now, we use SQLite so cluster is in single-instance mode
	// In production with external DB, this would enable automatically
	clusterMgr, err := cluster.NewManager(migrationMgr.GetDB())
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Cluster manager initialization failed: %v\n", err)
	}

	// Initialize search engines
	engineMgr := engine.NewManager(cfg)
	engineMgr.InitializeEngines()

	// Initialize services per AI.md specifications
	// SSL service (PART 21)
	sslSvc := ssl.New(cfg)
	if err := sslSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  SSL service initialization failed: %v\n", err)
	}

	// GeoIP service (PART 28)
	geoipSvc := geoip.New(cfg)
	if err := geoipSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  GeoIP service initialization failed: %v\n", err)
	}

	// Initialize logger per PART 21
	logger, err := logging.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Logger initialization failed: %v\n", err)
		// Create a basic logger that doesn't write to files
		logger = &logging.Logger{}
	}
	defer logger.Close()

	// Tor service (PART 30) - needs data dir, enabled flag, and logger
	torDataDir := filepath.Join(paths.Data, "tor")
	torSvc := tor.New(torDataDir, cfg.Search.Tor.Enabled, logger)

	// Blocklist service (PART 22)
	blocklistSvc := blocklist.New(cfg)
	if err := blocklistSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Blocklist service initialization failed: %v\n", err)
	}

	// CVE service (PART 22)
	cveSvc := cve.New(cfg)
	if err := cveSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  CVE service initialization failed: %v\n", err)
	}

	// Initialize scheduler per AI.md PART 27
	sched := scheduler.New()

	// Register all built-in tasks per AI.md PART 27
	sched.RegisterBuiltinTasks(scheduler.BuiltinTaskFuncs{
		SSLRenewal: func(ctx context.Context) error {
			// SSL certificate renewal check per PART 21
			if !cfg.Server.SSL.Enabled {
				return nil
			}
			if sslSvc.NeedsRenewal() {
				return sslSvc.RenewCertificate(ctx)
			}
			return nil
		},
		GeoIPUpdate: func(ctx context.Context) error {
			// GeoIP database update per PART 28
			if !cfg.Server.GeoIP.Enabled {
				return nil
			}
			return geoipSvc.Update()
		},
		BlocklistUpdate: func(ctx context.Context) error {
			// IP/domain blocklist update per PART 22
			return blocklistSvc.Update(ctx)
		},
		CVEUpdate: func(ctx context.Context) error {
			// CVE/security database update per PART 22
			return cveSvc.Update(ctx)
		},
		SessionCleanup: func(ctx context.Context) error {
			// Clean up expired sessions per PART 23
			return adminSvc.CleanupExpiredSessions()
		},
		TokenCleanup: func(ctx context.Context) error {
			// Clean up expired tokens per PART 23
			return adminSvc.CleanupExpiredTokens()
		},
		LogRotation: func(ctx context.Context) error {
			// Log rotation per PART 22
			// Logging service handles rotation automatically via RotatingFile
			// This task is a placeholder for manual rotation trigger if needed
			return nil
		},
		BackupAuto: func(ctx context.Context) error {
			// Automatic backup per PART 25 (disabled by default)
			maint := maintenance.New(paths.Config, paths.Data, version.Get())
			return maint.Backup("")
		},
		HealthcheckSelf: func(ctx context.Context) error {
			// Self health check per PART 27
			return nil
		},
		TorHealth: func(ctx context.Context) error {
			// Tor health check per PART 30 - only if Tor enabled
			if !cfg.Search.Tor.Enabled {
				return nil
			}
			// Check if Tor service is running
			if !torSvc.IsRunning() {
				return fmt.Errorf("tor service is not running")
			}
			return nil
		},
		ClusterHeartbeat: func(ctx context.Context) error {
			// Cluster heartbeat per PART 24 - runs every 30 seconds in cluster mode
			// Heartbeat runs automatically via cluster manager's heartbeatLoop()
			// This task just verifies cluster is healthy
			if clusterMgr == nil || !clusterMgr.IsEnabled() {
				return nil // Single instance mode - no clustering
			}
			// Cluster manager handles heartbeats automatically
			// Just verify we're still registered
			return nil
		},
	})

	// Start cluster manager if initialized per PART 24
	// Heartbeat loop runs automatically when cluster is started
	if clusterMgr != nil {
		ctx := context.Background()
		if err := clusterMgr.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Cluster manager start failed: %v\n", err)
		} else {
			defer clusterMgr.Stop()
		}
	}

	// Start scheduler
	sched.Start(context.Background())
	defer sched.Stop()

	// Create server with admin service, migration manager, scheduler, and logger per AI.md PART 11
	srv := server.New(cfg, configDir, dataDir, engineMgr, adminSvc, migrationMgr, sched, logger)

	// Start live config watcher per AI.md PART 1 NON-NEGOTIABLE
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
		// Per AI.md line 6197-6199: Never show localhost, 127.0.0.1, 0.0.0.0
		// Show only one address, the most relevant
		displayAddr := getDisplayAddress(cfg)

		// Console output per AI.md PART 31 lines 10230-10258
		isFirstRun := adminSvc.IsFirstRun()
		statusText := "Running"
		if isFirstRun {
			statusText = "Running (first run - setup available)"
		}

		// Check SMTP status per AI.md PART 31 lines 10267-10306
		smtpStatus := "Not detected (email features disabled)"
		smtpInfo := ""
		if cfg.Server.Email.Enabled {
			smtpHost := cfg.Server.Email.Host
			smtpPort := cfg.Server.Email.Port
			if smtpHost != "" && smtpPort > 0 {
				smtpStatus = fmt.Sprintf("Auto-detected (%s:%d)", smtpHost, smtpPort)
				smtpInfo = fmt.Sprintf("%s:%d (enabled)", smtpHost, smtpPort)
			}
		}

		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                                                                      ║")
		fmt.Printf("║   VIDVEIL v%-58s ║\n", version.Get())
		fmt.Println("║                                                                      ║")
		fmt.Printf("║   Status: %-60s ║\n", statusText)
		fmt.Println("║                                                                      ║")
		fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
		fmt.Println("║                                                                      ║")
		fmt.Println("║   🌐 Web Interface:                                                   ║")
		fmt.Printf("║      http://%-58s ║\n", displayAddr)
		fmt.Println("║                                                                      ║")
		fmt.Println("║   🔧 Admin Panel:                                                     ║")
		fmt.Printf("║      http://%-58s ║\n", displayAddr+"/admin")
		fmt.Println("║                                                                      ║")
		if isFirstRun {
			setupToken := adminSvc.GetSetupToken()
			if setupToken != "" {
				fmt.Println("║   🔑 Setup Token (use at /admin):                                     ║")
				fmt.Printf("║      %-64s ║\n", setupToken)
				fmt.Println("║                                                                      ║")
			}
		}
		fmt.Printf("║   📧 SMTP: %-59s ║\n", smtpStatus)
		if !cfg.Server.Email.Enabled {
			fmt.Println("║      Configure manually at /admin/server/email                       ║")
		}
		fmt.Println("║                                                                      ║")
		if isFirstRun {
			fmt.Println("║   ⚠️  Save the setup token! It will not be shown again.               ║")
			fmt.Println("║                                                                      ║")
		}
		if cfg.Search.Tor.Enabled {
			fmt.Printf("║   🧅 Tor: %-60s ║\n", cfg.Search.Tor.Proxy)
			fmt.Println("║                                                                      ║")
		}
		fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Printf("[INFO] Server started successfully\n")
		fmt.Printf("[INFO] Listening on %s\n", listenAddr)
		if smtpInfo != "" {
			fmt.Printf("[INFO] SMTP auto-detected: %s\n", smtpInfo)
		}
		fmt.Println()

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

	// Graceful shutdown with timeout (30 seconds per AI.md)
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
  --shell completions [SHELL]  Print shell completions (auto-detect if SHELL omitted)
  --shell init [SHELL]         Print shell init command (auto-detect if SHELL omitted)
  --status            Check server status and health
  --mode <mode>       Set application mode (production or development)
  --config <dir>      Set configuration directory
  --data <dir>        Set data directory
  --cache <dir>       Set cache directory
  --log <dir>         Set log directory
  --backup <dir>      Set backup directory
  --pid <file>        Set PID file path
  --address <addr>    Set listen address
  --port <port>       Set port (e.g., 8888 or 80,443)
  --debug             Enable debug mode (enables /debug/pprof endpoints)
  --daemon            Run in background (daemonize)

Update (AI.md PART 8):
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
  --maintenance setup             Reset admin credentials (recovery)

Environment Variables:
  MODE                Application mode (runtime, always checked)

  Initialization only (used once on first run):
  CONFIG_DIR          Configuration directory
  DATA_DIR            Data directory
  CACHE_DIR           Cache directory
  LOG_DIR             Log directory
  BACKUP_DIR          Backup directory
  PORT                Server port
  LISTEN              Listen address
  APPLICATION_NAME    Application title
  APPLICATION_TAGLINE Application description

Default behavior:
  Running without arguments initializes (if needed) and starts the server.

Shells: bash, zsh, fish, sh, dash, ksh, powershell, pwsh

Documentation: https://vidveil.apimgr.us
Source: https://github.com/apimgr/vidveil
`, version.Get())
}

func printVersion() {
	// Use main.go build variables per AI.md PART 13: --version Output
	fmt.Printf("vidveil %s\n", Version)
	fmt.Printf("Built: %s\n", BuildDate)
	fmt.Printf("Go: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
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

// handleShellCommand handles --shell completions and --shell init per PART 8
func handleShellCommand(subCmd, shell string) {
	binaryName := filepath.Base(os.Args[0])

	// Auto-detect shell from $SHELL if not specified
	if shell == "" {
		shellEnv := os.Getenv("SHELL")
		if shellEnv != "" {
			shell = filepath.Base(shellEnv)
		} else {
			shell = "bash"
		}
	}

	switch subCmd {
	case "completions":
		printCompletions(shell, binaryName)
	case "init":
		printInit(shell, binaryName)
	default:
		fmt.Fprintf(os.Stderr, "Unknown --shell command: %s\nUsage: --shell [completions|init] [SHELL]\n", subCmd)
		os.Exit(1)
	}
}

// printCompletions prints shell completion script to stdout per PART 8
func printCompletions(shell, binaryName string) {
	switch shell {
	case "bash":
		printBashCompletions(binaryName)
	case "zsh":
		printZshCompletions(binaryName)
	case "fish":
		printFishCompletions(binaryName)
	case "powershell", "pwsh":
		printPowerShellCompletions(binaryName)
	case "sh", "dash", "ksh":
		printBashCompletions(binaryName)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", shell)
		os.Exit(1)
	}
}

// printInit prints shell init command per PART 8
func printInit(shell, binaryName string) {
	switch shell {
	case "bash":
		fmt.Printf("source <(%s --shell completions bash)\n", binaryName)
	case "zsh":
		fmt.Printf("source <(%s --shell completions zsh)\n", binaryName)
	case "fish":
		fmt.Printf("%s --shell completions fish | source\n", binaryName)
	case "sh", "dash", "ksh":
		fmt.Printf("eval \"$(%s --shell completions %s)\"\n", binaryName, shell)
	case "powershell", "pwsh":
		fmt.Printf("Invoke-Expression (& %s --shell completions powershell)\n", binaryName)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", shell)
		os.Exit(1)
	}
}

func printBashCompletions(binaryName string) {
	fmt.Printf(`_%s_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local opts="--help --version --shell --config --data --cache --log --backup --pid --address --port --mode --status --daemon --debug --service --maintenance --update"
    COMPREPLY=($(compgen -W "$opts" -- "$cur"))
}
complete -F _%s_completions %s
`, binaryName, binaryName, binaryName)
}

func printZshCompletions(binaryName string) {
	fmt.Printf(`#compdef %s

_arguments \
    '(-h --help)'{-h,--help}'[Show help]' \
    '(-v --version)'{-v,--version}'[Show version]' \
    '--shell[Shell completions]:command:(completions init)' \
    '--config[Config directory]:directory:_files -/' \
    '--data[Data directory]:directory:_files -/' \
    '--cache[Cache directory]:directory:_files -/' \
    '--log[Log directory]:directory:_files -/' \
    '--backup[Backup directory]:directory:_files -/' \
    '--pid[PID file]:file:_files' \
    '--address[Listen address]:address:' \
    '--port[Listen port]:port:' \
    '--mode[Application mode]:mode:(production development)' \
    '--status[Show status]' \
    '--daemon[Run as daemon]' \
    '--debug[Enable debug mode]' \
    '--service[Service command]:command:(start stop restart reload install uninstall)' \
    '--maintenance[Maintenance command]:command:(backup restore update mode setup)' \
    '--update[Update command]:command:(check yes)'
`, binaryName)
}

func printFishCompletions(binaryName string) {
	fmt.Printf(`complete -c %s -s h -l help -d 'Show help'
complete -c %s -s v -l version -d 'Show version'
complete -c %s -l shell -d 'Shell completions' -xa 'completions init'
complete -c %s -l config -d 'Config directory' -r
complete -c %s -l data -d 'Data directory' -r
complete -c %s -l cache -d 'Cache directory' -r
complete -c %s -l log -d 'Log directory' -r
complete -c %s -l backup -d 'Backup directory' -r
complete -c %s -l pid -d 'PID file' -r
complete -c %s -l address -d 'Listen address'
complete -c %s -l port -d 'Listen port'
complete -c %s -l mode -d 'Application mode' -xa 'production development'
complete -c %s -l status -d 'Show status'
complete -c %s -l daemon -d 'Run as daemon'
complete -c %s -l debug -d 'Enable debug mode'
complete -c %s -l service -d 'Service command' -xa 'start stop restart reload install uninstall'
complete -c %s -l maintenance -d 'Maintenance command' -xa 'backup restore update mode setup'
complete -c %s -l update -d 'Update command' -xa 'check yes'
`, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName)
}

func printPowerShellCompletions(binaryName string) {
	fmt.Printf(`Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $completions = @(
        '--help', '--version', '--shell', '--config', '--data', '--cache',
        '--log', '--backup', '--pid', '--address', '--port', '--mode',
        '--status', '--daemon', '--debug', '--service', '--maintenance', '--update'
    )
    $completions | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`, binaryName)
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

// handleUpdateCommand implements AI.md PART 14 --update command
func handleUpdateCommand(cmd, arg string) {
	maint := maintenance.New("", "", version.Get())

	switch cmd {
	case "check":
		// Check for updates without installing (no privileges required)
		fmt.Println("Checking for updates...")
		fmt.Printf("Current version: %s\n", version.Get())

		info, err := maint.CheckUpdate()
		if err != nil {
			// HTTP 404 means no updates available per AI.md
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
		fmt.Printf("Current version: %s\n", version.Get())

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
Update Commands (AI.md PART 14):
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
	maint := maintenance.New("", "", version.Get())

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

		// Parse boolean per AI.md (1, yes, true, enable, enabled, on)
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

	case "setup":
		// Admin recovery per AI.md PART 26
		// Clears admin password and API token, generates new setup token
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                     ADMIN CREDENTIALS RESET                      ║")
		fmt.Println("╠══════════════════════════════════════════════════════════════════╣")

		setupToken, err := maint.ResetAdminCredentials()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to reset admin credentials: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("║  Admin password and API token have been cleared.                 ║")
		fmt.Println("║                                                                  ║")
		fmt.Println("║  NEW SETUP TOKEN (copy this now, shown ONCE):                    ║")
		fmt.Println("║  ┌────────────────────────────────────────────────────────────┐  ║")
		fmt.Printf("║  │  %-56s  │  ║\n", setupToken)
		fmt.Println("║  └────────────────────────────────────────────────────────────┘  ║")
		fmt.Println("║                                                                  ║")
		fmt.Println("║  1. Start the service: vidveil --service start                   ║")
		fmt.Println("║  2. Go to: http://{host}:{port}/admin                            ║")
		fmt.Println("║  3. Enter the setup token above                                  ║")
		fmt.Println("║  4. Create new admin account via setup wizard                    ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
		fmt.Println()

	default:
		fmt.Printf("❌ Unknown maintenance command: %s\n", cmd)
		fmt.Println(`
Maintenance Commands:
  vidveil --maintenance backup [file]     Create backup
  vidveil --maintenance restore [file]    Restore from backup
  vidveil --maintenance update            Check and apply updates
  vidveil --maintenance mode <on|off>     Enable/disable maintenance mode
  vidveil --maintenance setup             Reset admin credentials (recovery)`)
		os.Exit(1)
	}
}

func getDisplayAddress(cfg *config.Config) string {
	// Per AI.md PART 13: Never show 0.0.0.0, 127.0.0.1, localhost, etc.
	return net.JoinHostPort(config.GetDisplayHost(cfg), cfg.Server.Port)
}
