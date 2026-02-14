// SPDX-License-Identifier: MIT
// Vidveil - Privacy-respecting adult video meta search engine

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server"
	daemonpkg "github.com/apimgr/vidveil/src/server/daemon"
	"github.com/apimgr/vidveil/src/server/service/admin"
	"github.com/apimgr/vidveil/src/server/service/blocklist"
	"github.com/apimgr/vidveil/src/server/service/cluster"
	"github.com/apimgr/vidveil/src/server/service/cve"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/email"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/geoip"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/maintenance"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
	signalpkg "github.com/apimgr/vidveil/src/server/signal"
	"github.com/apimgr/vidveil/src/server/service/ssl"
	"github.com/apimgr/vidveil/src/server/service/system"
	"github.com/apimgr/vidveil/src/server/service/tor"
	"github.com/apimgr/vidveil/src/common/banner"
	"github.com/apimgr/vidveil/src/common/version"
)

// Build info - set via -ldflags at build time per PART 7
// OfficialSite: Empty = users must use --server flag for CLI client
var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildDate    = "unknown"
	OfficialSite = ""
)

func init() {
	// Sync build info to version package for other code per PART 7
	version.Version = Version
	version.CommitID = CommitID
	version.BuildTime = BuildDate
	version.OfficialSite = OfficialSite
}

func main() {
	args := os.Args[1:]

	// Parse arguments manually per AI.md spec
	var (
		configDir    string
		dataDir      string
		cacheDir     string
		logDir       string
		backupDir    string
		pidFile      string
		address      string
		port         string
		modeStr      string
		debug        bool
		daemon       bool
		// Per AI.md PART 8: --color flag (always, never, auto)
		colorFlag    string
		serviceCmd   string
		maintCmd     string
		maintArg string
		// Per AI.md PART 22: encryption password for backup/restore
		maintPassword string
		updateCmd     string
		updateArg    string
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

		case "--color":
			// Per AI.md PART 8: --color {always|never|auto}
			if i+1 < len(args) {
				i++
				colorFlag = args[i]
			}

		case "--service":
			if i+1 < len(args) {
				i++
				serviceCmd = args[i]
			}

		case "--update":
			// AI.md PART 23: --update [check|yes|branch {stable|beta|daily}]
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
				// Parse remaining args for maintenance command
				for i+1 < len(args) {
					nextArg := args[i+1]
					if nextArg == "--password" && i+2 < len(args) {
						// Per AI.md PART 22: --password for backup/restore encryption
						i += 2
						maintPassword = args[i]
					} else if !strings.HasPrefix(nextArg, "--") && maintArg == "" {
						i++
						maintArg = args[i]
					} else {
						break
					}
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
			} else if strings.HasPrefix(arg, "--color=") {
				colorFlag = strings.TrimPrefix(arg, "--color=")
			}
		}
		i++
	}

	// Per AI.md PART 8: Initialize color mode early (before any output)
	// Priority: CLI flag > config > NO_COLOR env > auto-detect
	if colorFlag != "" {
		terminal.SetColorMode(terminal.ParseColorFlag(colorFlag))
	}

	// Handle service command
	if serviceCmd != "" {
		handleServiceCommand(serviceCmd)
		return
	}

	// Handle update command (AI.md PART 23)
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
		handleMaintenanceCommand(maintCmd, maintArg, maintPassword)
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
	if pidFile == "" && os.Getenv("PID_FILE") != "" {
		pidFile = os.Getenv("PID_FILE")
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

	// Initialize mode and debug per AI.md PART 6
	// This must happen before starting the server
	mode.InitializeAppMode(modeStr, debug)

	// Handle daemon mode per AI.md PART 8
	if daemon {
		if err := daemonpkg.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to daemonize: %v\n", err)
			os.Exit(1)
		}
		// If we get here, we're either the child or daemonization failed
	}

	// Load configuration
	appConfig, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Get paths early so we can override log directory
	paths := config.GetAppPaths(configDir, dataDir)

	// Ensure system user/group and set directory ownership per AI.md PART 27
	// "Binary handles EVERYTHING else: directories, permissions, user/group, Tor, etc."
	appName := filepath.Base(os.Args[0])
	if appName == "" {
		appName = "vidveil"
	}
	// Remove any extension from binary name
	if ext := filepath.Ext(appName); ext != "" {
		appName = strings.TrimSuffix(appName, ext)
	}
	// Normalize to base name (vidveil, vidveil-agent, vidveil-cli)
	if strings.Contains(appName, "-") && !strings.HasPrefix(appName, "vidveil-") {
		appName = "vidveil"
	}

	// Create user and chown directories (only if running as root)
	// Include db subdirectory for SQLite database
	dbDir := filepath.Join(paths.Data, "db")
	dirsToOwn := []string{paths.Config, paths.Data, dbDir, paths.Cache, paths.Log}
	uid, gid, err := system.EnsureSystemUser(appName, dirsToOwn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Failed to ensure system user: %v\n", err)
	} else if system.IsRunningAsRoot() && uid > 0 {
		fmt.Printf("👤 Running as user %s (uid=%d, gid=%d)\n", appName, uid, gid)
	}

	// Override log directory if specified
	if logDir != "" {
		paths.Log = logDir
	}

	// Write PID file if specified per AI.md PART 8
	// Uses signal package which handles stale PID detection per AI.md PART 8
	// - Checks if PID file exists and process is running
	// - Verifies process is actually our binary (not PID reuse)
	// - Removes stale PID files automatically
	if pidFile != "" {
		if err := signalpkg.WritePIDFile(pidFile, appName); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		defer signalpkg.RemovePIDFile(pidFile)
	}

	// Override with command line flags
	if address != "" {
		appConfig.Server.Address = address
	}
	if port != "" {
		appConfig.Server.Port = port
	}

	// Apply mode (CLI > env > config, normalized)
	if modeStr != "" {
		appConfig.Server.Mode = config.NormalizeMode(modeStr)
	} else if appConfig.Server.Mode == "" {
		appConfig.Server.Mode = "production"
	} else {
		appConfig.Server.Mode = config.NormalizeMode(appConfig.Server.Mode)
	}

	// Initialize database per AI.md PART 10
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

	// Initialize admin service per AI.md PART 17
	adminSvc := admin.NewAdminService(migrationMgr.GetDB())
	if err := adminSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize admin service: %v\n", err)
		os.Exit(1)
	}

	// Initialize cluster manager per PART 24
	// Cluster mode auto-detected: SQLite = single instance, PostgreSQL/MySQL/MSSQL = cluster
	// For now, we use SQLite so cluster is in single-instance mode
	// In production with external DB, this would enable automatically
	clusterMgr, err := cluster.NewClusterManager(migrationMgr.GetDB())
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Cluster manager initialization failed: %v\n", err)
	}

	// Initialize search engines
	engineMgr := engine.NewEngineManager(appConfig)
	engineMgr.InitializeEngines()
	
	// Set custom autocomplete terms from config (adds to built-in suggestions)
	if len(appConfig.Search.CustomTerms) > 0 {
		engine.SetCustomTerms(appConfig.Search.CustomTerms)
	}

	// Initialize services per AI.md specifications
	// SSL service (PART 15)
	sslSvc := ssl.NewSSLManager(appConfig)
	if err := sslSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  SSL service initialization failed: %v\n", err)
	}

	// GeoIP service (PART 20)
	geoipSvc := geoip.NewGeoIPService(appConfig)
	if err := geoipSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  GeoIP service initialization failed: %v\n", err)
	}

	// Initialize logger per PART 11
	logger, err := logging.NewAppLogger(appConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Logger initialization failed: %v\n", err)
		// Create a basic logger that doesn't write to files
		logger = &logging.AppLogger{}
	}
	defer logger.Close()

	// Tor hidden service (PART 32) - auto-enabled if tor binary is found
	// Per PART 32: Also supports outbound network routing for engine queries
	torDataDir := filepath.Join(paths.Data, "tor")
	torSvc := tor.NewTorService(torDataDir, logger)
	torSvc.SetConfig(&appConfig.Server.Tor) // Pass Tor config for outbound network settings

	// Blocklist service (PART 11)
	blocklistSvc := blocklist.NewBlocklistService(appConfig)
	if err := blocklistSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Blocklist service initialization failed: %v\n", err)
	}

	// CVE service (PART 11)
	cveSvc := cve.NewCVEService(appConfig)
	if err := cveSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  CVE service initialization failed: %v\n", err)
	}

	// Initialize scheduler with database persistence per AI.md PART 19
	// Task state (run_count, fail_count, last_run) survives restarts
	sched := scheduler.NewSchedulerWithDB(migrationMgr.GetDB())

	// Set catch-up window per AI.md PART 19
	// Missed tasks within this window will run on startup
	if appConfig.Server.Schedule.CatchUpWindow != "" {
		if catchUpDuration, err := time.ParseDuration(appConfig.Server.Schedule.CatchUpWindow); err == nil {
			sched.SetCatchUpWindow(catchUpDuration)
		}
	}

	// Register all built-in tasks per AI.md PART 19
	sched.RegisterBuiltinTasks(scheduler.BuiltinTaskFuncs{
		SSLRenewal: func(ctx context.Context) error {
			// SSL certificate renewal check per PART 15
			if !appConfig.Server.SSL.Enabled {
				return nil
			}
			if sslSvc.NeedsRenewal() {
				return sslSvc.RenewCertificate(ctx)
			}
			return nil
		},
		GeoIPUpdate: func(ctx context.Context) error {
			// GeoIP database update per PART 20
			if !appConfig.Server.GeoIP.Enabled {
				return nil
			}
			return geoipSvc.Update()
		},
		BlocklistUpdate: func(ctx context.Context) error {
			// IP/domain blocklist update per PART 11
			return blocklistSvc.Update(ctx)
		},
		CVEUpdate: func(ctx context.Context) error {
			// CVE/security database update per PART 11
			return cveSvc.Update(ctx)
		},
		SessionCleanup: func(ctx context.Context) error {
			// Clean up expired sessions per PART 11
			return adminSvc.CleanupExpiredSessions()
		},
		TokenCleanup: func(ctx context.Context) error {
			// Clean up expired tokens per PART 11
			return adminSvc.CleanupExpiredTokens()
		},
		LogRotation: func(ctx context.Context) error {
			// Log rotation per AI.md PART 19: trigger log file reopen/rotation
			logger.Reopen()
			return nil
		},
		BackupAuto: func(ctx context.Context) error {
			// Automatic backup per PART 22 (disabled by default)
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			return maint.Backup("")
		},
		HealthcheckSelf: func(ctx context.Context) error {
			// Self health check per PART 13
			return nil
		},
		TorHealth: func(ctx context.Context) error {
			// Tor health check per PART 32 - only if hidden service enabled
			// Per PART 32: Tor supports hidden service and optional outbound network routing
			if torSvc == nil {
				return nil
			}
			// Check if Tor service is running
			if !torSvc.IsRunning() {
				return fmt.Errorf("tor service is not running")
			}
			return nil
		},
		ClusterHeartbeat: func(ctx context.Context) error {
			// Cluster heartbeat per PART 10 - runs every 30 seconds in cluster mode
			// Heartbeat runs automatically via cluster manager's heartbeatLoop()
			// This task just verifies cluster is healthy
			if clusterMgr == nil || !clusterMgr.IsEnabled() {
				// Single instance mode - no clustering
				return nil
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

	// Set Tor provider for engine manager per PART 32
	// This enables Tor outbound network for anonymized engine queries when UseNetwork is true
	engineMgr.SetTorProvider(torSvc)

	// Start Tor hidden service per PART 32 (in background to not block HTTP server)
	// Auto-enabled if tor binary is installed - no enable flag needed
	// Per PART 32: Uses Unix socket on Unix, high TCP port (63000+) on Windows
	// Start Tor in background goroutine - HTTP server should be available immediately
	go func() {
		torCtx := context.Background()
		// localPort=0 means Tor uses Unix socket on Unix, or finds available port on Windows
		if err := torSvc.Start(torCtx, 0); err != nil {
			// PART 32: Tor errors are WARN level, server continues without Tor
			fmt.Fprintf(os.Stderr, "⚠️  Tor hidden service: %v\n", err)
		} else if torSvc.UseNetworkEnabled() && torSvc.OutboundEnabled() {
			fmt.Println("[INFO] Tor outbound network enabled - engine queries are anonymized")
		}
	}()
	defer torSvc.Stop()

	// Load scheduler history from database per AI.md PART 19
	if err := sched.LoadHistoryFromDB(100); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Failed to load scheduler history: %v\n", err)
	}

	// Start scheduler
	sched.Start(context.Background())
	defer sched.Stop()

	// Create server with admin service, migration manager, scheduler, and logger per AI.md PART 11
	srv := server.NewServer(appConfig, configDir, dataDir, engineMgr, adminSvc, migrationMgr, sched, logger)

	// Set Tor service for handlers per AI.md PART 32
	srv.SetTorService(torSvc)

	// Set GeoIP service for content restriction checks
	srv.SetGeoIPService(geoipSvc)

	// Start live config watcher per AI.md PART 1 NON-NEGOTIABLE
	configWatcher := config.NewWatcher(configPath, appConfig)
	configWatcher.OnReload(func(newCfg *config.AppConfig) {
		// Config has been reloaded - the shared appConfig pointer is already updated
		// Additional reload actions can be added here if needed
	})
	configWatcher.Start()
	defer configWatcher.Stop()

	// Start server in goroutine
	go func() {
		// Build listen address properly handling IPv6
		listenAddr := appConfig.Server.Address + ":" + appConfig.Server.Port
		// Per AI.md PART 13: Display Rules
		// - Never show: 0.0.0.0, 127.0.0.1, localhost
		// - Show only: One address, the most relevant
		displayAddr := getDisplayAddress(appConfig)

		// Console output per AI.md PART 7
		isFirstRun := adminSvc.IsFirstRun()

		// Check SMTP status per AI.md PART 18
		smtpInfo := ""
		if appConfig.Server.Email.Enabled {
			smtpHost := appConfig.Server.Email.Host
			smtpPort := appConfig.Server.Email.Port

			if smtpHost != "" && smtpPort > 0 {
				// Per PART 18: Test configured SMTP on every startup
				if err := email.TestSMTPConfig(smtpHost, smtpPort); err == nil {
					smtpInfo = fmt.Sprintf("%s:%d", smtpHost, smtpPort)
				}
			} else {
				// Per PART 18: Auto-detect on first run if no host configured
				detectedHost, detectedPort := email.AutodetectSMTP(
					appConfig.Server.Email.AutodetectHost,
					appConfig.Server.Email.AutodetectPort,
				)
				if detectedHost != "" && detectedPort > 0 {
					smtpInfo = fmt.Sprintf("%s:%d (auto)", detectedHost, detectedPort)
				}
			}
		}

		// Build URL per AI.md PART 13:
		// - NEVER show localhost, 127.0.0.1, 0.0.0.0
		// - Show only one address, the most relevant
		// - Strip :80 and :443 from URLs
		port := appConfig.Server.Port
		displayURL := "http://" + displayAddr
		if port == "80" {
			displayURL = "http://" + config.GetDisplayHost(appConfig)
		} else if port == "443" {
			displayURL = "https://" + config.GetDisplayHost(appConfig)
		}

		// Get setup token for first run
		var setupToken string
		if isFirstRun {
			setupToken = adminSvc.GetSetupToken()
		}

		// Print responsive startup banner per AI.md PART 7
		banner.PrintStartupBanner(banner.BannerConfig{
			AppName:    "VidVeil",
			Version:    version.GetVersion(),
			AppMode:    appConfig.Server.Mode,
			Debug:      mode.IsDebugEnabled(),
			URLs:       []string{displayURL},
			ShowSetup:  isFirstRun,
			SetupToken: setupToken,
		})

		// Log INFO lines per AI.md PART 11
		fmt.Printf("[INFO] Server started successfully\n")
		fmt.Printf("[INFO] Listening on %s\n", listenAddr)
		if smtpInfo != "" {
			fmt.Printf("[INFO] SMTP configured: %s\n", smtpInfo)
		}
		fmt.Println()

		if err := srv.ListenAndServe(listenAddr); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "❌ Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Start serving on Tor listener when available (per PART 32)
	// This allows the same HTTP handler to serve both clearnet and .onion traffic
	go func() {
		// Wait a bit for Tor to initialize
		time.Sleep(5 * time.Second)

		// Check periodically if Tor listener is available
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C
			if torListener := torSvc.GetListener(); torListener != nil {
				fmt.Println("[INFO] Serving HTTP on Tor hidden service")
				if err := srv.Serve(torListener); err != nil && err != http.ErrServerClosed {
					// Tor listener closed, this is normal during shutdown
					fmt.Fprintf(os.Stderr, "⚠️  Tor HTTP server: %v\n", err)
				}
				return
			}
		}
	}()

	// Configure signal handlers per AI.md PART 8
	// SIGUSR1 (10) → Reopen logs (log rotation)
	// SIGUSR2 (12) → Status dump
	signalpkg.SetLogReopenFunc(func() {
		logger.Reopen()
	})
	signalpkg.SetStatusDumpFunc(func() {
		// Dump status to stderr
		fmt.Fprintf(os.Stderr, "[STATUS] Server running on %s:%s\n", appConfig.Server.Address, appConfig.Server.Port)
		fmt.Fprintf(os.Stderr, "[STATUS] Mode: %s, Debug: %v\n", appConfig.Server.Mode, mode.IsDebugEnabled())
		fmt.Fprintf(os.Stderr, "[STATUS] Uptime: %v\n", time.Since(time.Now()))
	})

	// Wait for shutdown signal per AI.md PART 8
	// Handles: SIGTERM(15), SIGINT(2), SIGQUIT(3), SIGRTMIN+3(37)
	// Ignores: SIGHUP(1) - config auto-reloads via file watcher
	sig := signalpkg.WaitForShutdown(context.Background())
	fmt.Printf("\n%s Received %v, shutting down gracefully...\n", terminal.StopIcon(), sig)

	// Graceful shutdown with timeout (30 seconds per AI.md PART 8)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Shutdown error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s Server stopped\n", terminal.StatusIcon(true))
}

func printHelp() {
	// Per AI.md PART 8: Exact --help output format
	binaryName := filepath.Base(os.Args[0])
	fmt.Printf(`%s %s - Privacy-respecting adult video meta search engine

Usage:
  %s [flags]

Information:
  -h, --help                        Show help (--help for any command shows its help)
  -v, --version                     Show version
      --status                      Show server status and health

Shell Integration:
      --shell completions [SHELL]   Print shell completions
      --shell init [SHELL]          Print shell init command
      --shell --help                Show shell help

Server Configuration:
      --mode {production|development}  Application mode (default: production)
      --config DIR                  Config directory
      --data DIR                    Data directory
      --cache DIR                   Cache directory
      --log DIR                     Log directory
      --backup DIR                  Backup directory
      --pid FILE                    PID file path
      --address ADDR                Listen address (default: 0.0.0.0)
      --port PORT                   Listen port (default: random 64xxx, 80 in container)
      --daemon                      Run as daemon (detach from terminal)
      --debug                       Enable debug mode
      --color {always|never|auto}   Color output (default: auto, respects NO_COLOR)

Service Management:
      --service CMD                 Service management (--service --help for details)
      --maintenance CMD             Maintenance operations (--maintenance --help for details)
      --update [CMD]                Check/perform updates (--update --help for details)

Run '%s <command> --help' for detailed help on any command.
`, binaryName, version.GetVersion(), binaryName, binaryName)
}

func printVersion() {
	// Use main.go build variables per AI.md PART 13: --version Output
	// Per AI.md PART 8: Use actual binary name, not hardcoded
	binaryName := filepath.Base(os.Args[0])
	fmt.Printf("%s %s\n", binaryName, Version)
	fmt.Printf("Built: %s\n", BuildDate)
	fmt.Printf("Go: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func checkStatus() int {
	// Get paths
	appPaths := config.GetAppPaths("", "")

	// Try to load config to check if initialized
	statusConfig, _, err := config.LoadAppConfig("", "")
	if err != nil {
		fmt.Println("❌ Status: Not initialized")
		fmt.Printf("   Config dir: %s\n", appPaths.Config)
		return 1
	}

	// Try to connect to the server
	addr := net.JoinHostPort("127.0.0.1", statusConfig.Server.Port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		fmt.Println("⚠️  Status: Stopped")
		fmt.Printf("   Port: %s (not listening)\n", statusConfig.Server.Port)
		return 1
	}
	conn.Close()

	// Server is running - try health check
	healthURL := fmt.Sprintf("http://%s/healthz", addr)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Println("⚠️  Status: Running (health check failed)")
		fmt.Printf("   Port: %s\n", statusConfig.Server.Port)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("✅ Status: Running")
		fmt.Printf("   Port: %s\n", statusConfig.Server.Port)
		fmt.Printf("   FQDN: %s\n", statusConfig.Server.FQDN)
		return 0
	}

	fmt.Println("⚠️  Status: Running (unhealthy)")
	fmt.Printf("   Port: %s\n", statusConfig.Server.Port)
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
	case "--help", "help", "-h":
		// Per AI.md PART 8: --shell --help prints help and exits 0
		fmt.Println(`Shell Integration Commands:
  vidveil --shell completions [SHELL]   Print shell completions script
  vidveil --shell init [SHELL]          Print shell init command for eval

Supported Shells:
  bash, zsh, fish, powershell, pwsh, sh, dash, ksh

Examples:
  # Add to ~/.bashrc or ~/.zshrc
  eval "$(vidveil --shell init)"

  # Or source completions directly
  source <(vidveil --shell completions bash)`)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown --shell command: %s\nUsage: --shell [completions|init|--help] [SHELL]\n", subCmd)
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
    local opts="--help --version --shell --config --data --cache --log --backup --pid --address --port --mode --status --daemon --debug --color --service --maintenance --update"
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
    '--color[Color output]:color:(always never auto)' \
    '--service[Service command]:command:(start stop restart reload status install uninstall)' \
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
complete -c %s -l color -d 'Color output' -xa 'always never auto'
complete -c %s -l service -d 'Service command' -xa 'start stop restart reload status install uninstall'
complete -c %s -l maintenance -d 'Maintenance command' -xa 'backup restore update mode setup'
complete -c %s -l update -d 'Update command' -xa 'check yes'
`, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName)
}

func printPowerShellCompletions(binaryName string) {
	fmt.Printf(`Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $completions = @(
        '--help', '--version', '--shell', '--config', '--data', '--cache',
        '--log', '--backup', '--pid', '--address', '--port', '--mode',
        '--status', '--daemon', '--debug', '--color', '--service', '--maintenance', '--update'
    )
    $completions | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`, binaryName)
}

func handleServiceCommand(cmd string) {
	// Per AI.md PART 24 and PART 25: Use system.NewServiceManager which creates system user
	// Get binary path
	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to get executable path: %v\n", err)
		os.Exit(1)
	}
	
	// Get default paths per AI.md PART 4
	isRoot := os.Geteuid() == 0
	configDir := paths.GetDefaultConfigDir(isRoot)
	dataDir := paths.GetDefaultDataDir(isRoot)
	
	// Use system.NewServiceManager which handles user creation per AI.md PART 4
	svc := system.NewServiceManager("vidveil", binaryPath, configDir, dataDir)

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

	case "status":
		// Per AI.md PART 25: Show service status
		status, err := svc.GetServiceStatus()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to get status: %v\n", err)
			os.Exit(1)
		}
		switch status {
		case "running":
			fmt.Println("✅ Vidveil service is running")
		case "stopped":
			fmt.Println("⏹️ Vidveil service is stopped")
		default:
			fmt.Printf("❓ Vidveil service status: %s\n", status)
		}

	case "--install":
		// Per AI.md PART 24: Check escalation before service install
		if err := system.HandleEscalation("Service installation"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Installing Vidveil as system service...")
		if err := svc.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to install: %v\n", err)
			os.Exit(1)
		}

	case "--uninstall":
		// Per AI.md PART 24: Confirmation required before destructive action
		fmt.Println("⚠️  WARNING: This will:")
		fmt.Println("   • Stop the service (if running)")
		fmt.Println("   • Remove service configuration")
		fmt.Println("   • Delete data, configs, and logs")
		fmt.Println("   • Remove system user (if created)")
		fmt.Println()
		fmt.Print("This will delete ALL data, configs, and the system user. Continue? [y/N] ")

		var response string
		fmt.Scanln(&response)
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}

		// Per AI.md PART 24: Check escalation before service uninstall
		if err := system.HandleEscalation("Service uninstallation"); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Uninstalling Vidveil system service...")
		if err := svc.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to uninstall: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Service uninstalled")

	case "--disable":
		// Per AI.md PART 8: Disable service from starting at boot
		fmt.Println("Disabling Vidveil service from starting at boot...")
		if err := svc.Disable(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to disable: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Service disabled (will not start at boot)")

	case "--help":
		// Per AI.md PART 8: Service command help
		fmt.Println(`Service Management Commands:

  vidveil --service start         Start the service
  vidveil --service stop          Stop the service
  vidveil --service restart       Restart the service
  vidveil --service reload        Reload configuration
  vidveil --service status        Show service status
  vidveil --service --install     Install as system service
  vidveil --service --uninstall   Uninstall system service
  vidveil --service --disable     Disable service from starting at boot

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

// handleUpdateCommand implements AI.md PART 23 --update command
func handleUpdateCommand(cmd, arg string) {
	maint := maintenance.NewMaintenanceManager("", "", version.GetVersion())

	switch cmd {
	case "check":
		// Check for updates without installing (no privileges required)
		fmt.Println("Checking for updates...")
		fmt.Printf("Current version: %s\n", version.GetVersion())

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
		fmt.Printf("Current version: %s\n", version.GetVersion())

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

	case "--help", "help", "-h":
		// Per AI.md PART 8: --update --help prints help and exits 0
		fmt.Println(`Update Commands:
  vidveil --update              Check and perform in-place update with restart
  vidveil --update yes          Same as --update (default)
  vidveil --update check        Check for updates without installing
  vidveil --update branch <name>  Set update branch (stable, beta, daily)

Update Branches:
  stable (default)  Release builds (v*, *.*.*)
  beta              Pre-release builds (*-beta)
  daily             Daily builds (YYYYMMDDHHMM)`)
		os.Exit(0)

	default:
		fmt.Printf("❌ Unknown update command: %s\n", cmd)
		fmt.Println(`
Usage: vidveil --update [check|yes|branch <name>|--help]

Run 'vidveil --update --help' for detailed help.`)
		os.Exit(1)
	}
}

func handleMaintenanceCommand(cmd, arg, password string) {
	maint := maintenance.NewMaintenanceManager("", "", version.GetVersion())

	switch cmd {
	case "backup":
		// Per AI.md PART 22: Support --password for encrypted backups
		if password != "" {
			fmt.Println("Creating encrypted backup...")
			if err := maint.BackupWithOptions(maintenance.BackupOptions{
				Filename:    arg,
				Password:    password,
				IncludeData: true,
				MaxBackups:  1,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Backup failed: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("Creating backup...")
			if err := maint.Backup(arg); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Backup failed: %v\n", err)
				os.Exit(1)
			}
		}

	case "restore":
		if arg == "" {
			fmt.Println("Restoring from most recent backup...")
		} else {
			fmt.Printf("Restoring from %s...\n", arg)
		}
		// Per AI.md PART 22: Support --password for encrypted backups
		if err := maint.RestoreWithPassword(arg, password); err != nil {
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
		// Admin recovery per AI.md PART 22
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

	case "--help", "help", "-h":
		// Per AI.md PART 8: --maintenance --help prints help and exits 0
		fmt.Println(`Maintenance Commands:
  vidveil --maintenance backup [file] [--password <pwd>]   Create backup
  vidveil --maintenance restore [file] [--password <pwd>]  Restore from backup
  vidveil --maintenance update                              Check and apply updates
  vidveil --maintenance mode <on|off>                       Enable/disable maintenance mode
  vidveil --maintenance setup                               Reset admin credentials (recovery)

Options:
  --password <password>    Encryption password for backup/restore (per AI.md PART 22)

Examples:
  vidveil --maintenance backup                              # Backup to default location
  vidveil --maintenance backup --password "secret"          # Encrypted backup
  vidveil --maintenance backup /tmp/backup.tar              # Backup to specific file
  vidveil --maintenance restore                             # Restore from most recent
  vidveil --maintenance restore backup.tar.gz.enc --password "secret"  # Restore encrypted
  vidveil --maintenance mode on                             # Enable maintenance mode`)
		os.Exit(0)

	default:
		fmt.Printf("❌ Unknown maintenance command: %s\n", cmd)
		fmt.Println(`
Usage: vidveil --maintenance [backup|restore|update|mode|setup|--help]

Run 'vidveil --maintenance --help' for detailed help.`)
		os.Exit(1)
	}
}

func getDisplayAddress(serverConfig *config.AppConfig) string {
	// Per AI.md PART 13: Never show 0.0.0.0, 127.0.0.1, localhost, etc.
	return net.JoinHostPort(config.GetDisplayHost(serverConfig), serverConfig.Server.Port)
}
