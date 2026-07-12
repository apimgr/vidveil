// SPDX-License-Identifier: MIT
// Vidveil - Privacy-respecting adult video meta search engine

package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/banner"
	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/server"
	daemonpkg "github.com/apimgr/vidveil/src/server/daemon"
	"github.com/apimgr/vidveil/src/server/service/blocklist"
	"github.com/apimgr/vidveil/src/server/service/cve"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/email"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/geoip"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/maintenance"
	svcmetrics "github.com/apimgr/vidveil/src/server/service/metrics"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
	"github.com/apimgr/vidveil/src/server/service/secrets"
	"github.com/apimgr/vidveil/src/server/service/ssl"
	"github.com/apimgr/vidveil/src/server/service/system"
	"github.com/apimgr/vidveil/src/server/service/tor"
	signalpkg "github.com/apimgr/vidveil/src/server/signal"
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
	// Initialise Prometheus application metrics (PART 20)
	svcmetrics.InitMetricsAppInfo(Version, CommitID, BuildDate, runtime.Version())
}

func main() {
	startTime := time.Now()
	args := os.Args[1:]

	// Parse arguments manually per AI.md spec
	var (
		configDir string
		dataDir   string
		cacheDir  string
		logDir    string
		backupDir string
		pidFile   string
		address   string
		port      string
		// Per AI.md PART 8: --baseurl PATH (URL path prefix, default "/")
		baseURL string
		modeStr string
		debug   bool
		daemon  bool
		// Per AI.md PART 8: --color flag (auto, yes, no)
		colorFlag string
		// Per AI.md PART 8: --lang CODE (output language, default "auto")
		langFlag   string
		serviceCmd string
		maintCmd   string
		maintArg   string
		// Per AI.md PART 21: encryption password for backup/restore
		maintPassword string
		updateCmd     string
		updateArg     string
		// Per AI.md PART 31: tor subcommand args (status, validate, restart, ...)
		torArgs []string
		torCmd  bool
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

		case "--baseurl":
			// Per AI.md PART 8: URL path prefix, default "/"
			if i+1 < len(args) {
				i++
				baseURL = args[i]
			}

		case "--lang":
			// Per AI.md PART 8: language for output, default "auto" (from LANG env)
			if i+1 < len(args) {
				i++
				langFlag = args[i]
			}

		case "--mode":
			if i+1 < len(args) {
				i++
				modeStr = args[i]
			}

		case "--debug":
			debug = true

		case "--color":
			// Per AI.md PART 8: --color {auto|yes|no}
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
			// AI.md PART 22: --update [check|yes|branch {stable|beta|daily}]
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

		case "tor":
			// Per AI.md PART 31: tor {status|validate|restart|regenerate|vanity|import-keys}
			// All remaining args belong to the tor command
			torCmd = true
			torArgs = args[i+1:]
			i = len(args)
			continue

		case "--maintenance":
			if i+1 < len(args) {
				i++
				maintCmd = args[i]
				// Parse remaining args for maintenance command
				for i+1 < len(args) {
					nextArg := args[i+1]
					if nextArg == "--password" && i+2 < len(args) {
						// Per AI.md PART 21: --password for backup/restore encryption
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
			} else if strings.HasPrefix(arg, "--baseurl=") {
				baseURL = strings.TrimPrefix(arg, "--baseurl=")
			} else if strings.HasPrefix(arg, "--mode=") {
				modeStr = strings.TrimPrefix(arg, "--mode=")
			} else if strings.HasPrefix(arg, "--color=") {
				colorFlag = strings.TrimPrefix(arg, "--color=")
			} else if strings.HasPrefix(arg, "--lang=") {
				langFlag = strings.TrimPrefix(arg, "--lang=")
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
	// Port chain per AI.md: --port > VIDVEIL_PORT > PORT > config > random
	if port == "" && os.Getenv("VIDVEIL_PORT") != "" {
		port = os.Getenv("VIDVEIL_PORT")
	}
	if port == "" && os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	if address == "" && os.Getenv("LISTEN") != "" {
		address = os.Getenv("LISTEN")
	} else if address == "" && os.Getenv("ADDRESS") != "" {
		address = os.Getenv("ADDRESS")
	}

	// Per AI.md PART 8: --baseurl PATH (URL path prefix, default "/").
	// Env var fallback: BASEURL.
	if baseURL == "" && os.Getenv("BASEURL") != "" {
		baseURL = os.Getenv("BASEURL")
	}

	// Per AI.md PART 8: --lang CODE (output language, default "auto").
	// Env var fallback: LANG (POSIX standard, e.g. "en_US.UTF-8").
	if langFlag == "" && os.Getenv("LANG") != "" {
		langFlag = os.Getenv("LANG")
	}

	// MODE env var is runtime - always checked per AI.md
	// Priority: CLI flag > env var > config file
	if modeStr == "" && os.Getenv("MODE") != "" {
		modeStr = os.Getenv("MODE")
	}

	setPathEnv := func(name, value string) {
		if value == "" {
			return
		}

		if err := os.Setenv(name, value); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to set %s: %v\n", name, err)
			os.Exit(1)
		}
	}

	setPathEnv("CONFIG_DIR", configDir)
	setPathEnv("DATA_DIR", dataDir)
	setPathEnv("CACHE_DIR", cacheDir)
	setPathEnv("LOG_DIR", logDir)
	setPathEnv("BACKUP_DIR", backupDir)
	// Propagate --baseurl / --lang via env so child code paths (config
	// loader, server router, i18n) can read them without an extra
	// plumbing parameter.
	setPathEnv("BASEURL", baseURL)
	setPathEnv("LANG", langFlag)

	// Per AI.md PART 31: tor CLI commands
	if torCmd {
		os.Exit(handleTorCommand(torArgs, configDir, dataDir))
	}

	if serviceCmd != "" {
		handleServiceCommand(serviceCmd, configDir, dataDir)
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
		handleMaintenanceCommand(maintCmd, maintArg, maintPassword, configDir, dataDir)
		return
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

	// Ensure system user/group and set directory ownership per AI.md PART 23
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
	dbDir := config.GetDatabaseDir(paths.Data)
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
	// Per AI.md PART 12: CLI --baseurl overrides server.baseurl config value.
	if baseURL != "" {
		appConfig.Server.BaseURL = baseURL
	} else if appConfig.Server.BaseURL == "" {
		appConfig.Server.BaseURL = "/"
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
	// VidVeil is stateless/privacy-first: only server.db (admin/config/audit).
	// No regular users, organizations, or custom domains (PARTS 34-36 NOT implemented).
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

	// Initialize app secrets per AI.md PART 11
	// Generates installation_secret, cookie_signing_key, csrf_token_secret on first run
	secretsMgr := secrets.NewManager(migrationMgr.GetDB())
	if err := secretsMgr.EnsureSecrets(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize secrets: %v\n", err)
		os.Exit(1)
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
	sslSvc := ssl.NewSSLManager(appConfig, paths.Config)
	if err := sslSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  SSL service initialization failed: %v\n", err)
	}

	// GeoIP service (PART 19)
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

	// Tor hidden service (PART 31) - auto-enabled if tor binary is found
	// Per PART 31: Also supports outbound network routing for engine queries
	// Pass paths.Data so NewTorService can append "tor" internally → {data_dir}/tor/
	torSvc := tor.NewTorService(paths.Data, logger)
	// Pass Tor config for outbound network settings
	torSvc.SetConfig(&appConfig.Server.Tor)
	// Pass config dir for torrc generation
	torSvc.SetConfigDir(filepath.Join(paths.Config, "tor"))

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

	// Initialize scheduler with database persistence per AI.md PART 18
	// Task state (run_count, fail_count, last_run) survives restarts
	sched := scheduler.NewSchedulerWithDB(migrationMgr.GetDB())

	// Set catch-up window per AI.md PART 18
	// Missed tasks within this window will run on startup
	if appConfig.Server.Schedule.CatchUpWindow != "" {
		if catchUpDuration, err := time.ParseDuration(appConfig.Server.Schedule.CatchUpWindow); err == nil {
			sched.SetCatchUpWindow(catchUpDuration)
		}
	}

	// Register all built-in tasks per AI.md PART 18
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
			// GeoIP database update per PART 19
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
		TokenCleanup: func(ctx context.Context) error {
			return nil
		},
		LogRotation: func(ctx context.Context) error {
			// Log rotation per AI.md PART 18: trigger log file reopen/rotation
			logger.Reopen()
			return nil
		},
		BackupDaily: func(ctx context.Context) error {
			// Daily backup per AI.md PART 18/21 (enabled by default, daily at 02:00)
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			return maint.Backup("")
		},
		BackupHourly: func(ctx context.Context) error {
			// Hourly incremental backup per AI.md PART 18/21 (disabled by default)
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			return maint.BackupIncremental("")
		},
		HealthcheckSelf: func(ctx context.Context) error {
			// Self health check per PART 13
			return nil
		},
		TorHealth: func(ctx context.Context) error {
			// Tor health check per PART 31 - only if hidden service enabled
			// Per PART 31: Tor supports hidden service and optional outbound network routing
			if torSvc == nil {
				return nil
			}
			// Check if Tor service is running
			if !torSvc.IsRunning() {
				return fmt.Errorf("tor service is not running")
			}
			return nil
		},
		UpdateCheck: func(ctx context.Context) error {
			// Update check per AI.md PART 18/22 — daily at 06:00
			// Notify-only unless update.auto_install is true; honors update.defer_days
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			info, err := maint.CheckUpdate()
			if err != nil {
				return fmt.Errorf("update check: %w", err)
			}
			if !info.UpdateAvailable {
				return nil
			}
			// Apply defer_days gate: skip releases younger than defer_days
			deferDays := appConfig.Server.Update.DeferDays
			if deferDays > 0 && !info.PublishedAt.IsZero() {
				cutoff := info.PublishedAt.AddDate(0, 0, deferDays)
				if time.Now().Before(cutoff) {
					return nil
				}
			}
			// Notify via structured log — event consumed by the email/webhook notification path
			logger.Info("update available", map[string]interface{}{
				"current": info.CurrentVersion,
				"latest":  info.LatestVersion,
				"url":     info.ReleaseURL,
			})
			// Auto-install only when explicitly configured
			if appConfig.Server.Update.AutoInstall {
				return maint.ApplyUpdate(info.DownloadURL)
			}
			return nil
		},
	})

	// Set Tor provider for engine manager per PART 31
	// This enables Tor outbound network for anonymized engine queries when UseNetwork is true
	engineMgr.SetTorProvider(torSvc)

	// Start Tor hidden service per PART 31 (in background to not block HTTP server)
	// Auto-enabled if tor binary is installed - no enable flag needed
	// Per PART 31: ADD_ONION maps .onion:virtualPort → 127.0.0.1:serverPort (existing HTTP listener)
	go func() {
		torCtx := context.Background()
		// Parse server port from config — Tor will forward .onion traffic to this existing HTTP port
		serverPort, _ := strconv.Atoi(appConfig.Server.Port)
		if err := torSvc.Start(torCtx, serverPort); err != nil {
			// PART 31: Tor errors are WARN level, server continues without Tor
			fmt.Fprintf(os.Stderr, "⚠️  Tor hidden service: %v\n", err)
		} else {
			// Wire resolved onion address back to config so PART 12 Tor request
			// detection (urlvars.isTorRequest) can match the Host header.
			if addr := torSvc.GetOnionAddress(); addr != "" {
				appConfig.Server.Tor.OnionAddress = addr
			}
			if torSvc.UseNetworkEnabled() && torSvc.OutboundEnabled() {
				fmt.Println("[INFO] Tor outbound network enabled - engine queries are anonymized")
			}
		}
	}()
	defer torSvc.Stop()

	// Load scheduler history from database per AI.md PART 18
	if err := sched.LoadHistoryFromDB(100); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Failed to load scheduler history: %v\n", err)
	}

	// Start scheduler
	sched.Start(context.Background())
	defer sched.Stop()

	// Create server with migration manager, scheduler, and logger per AI.md PART 11
	srv := server.NewServer(appConfig, configDir, dataDir, engineMgr, migrationMgr, sched, logger)

	// Set Tor service for handlers per AI.md PART 31
	srv.SetTorService(torSvc)

	// Set GeoIP service for content restriction checks and country blocking per AI.md PART 19
	srv.SetGeoIPService(geoipSvc)

	// Set blocklist service for IP/domain blocklist middleware per AI.md PART 11
	srv.SetBlocklistService(blocklistSvc)

	// Start live config watcher per AI.md PART 8 NON-NEGOTIABLE
	configWatcher := config.NewWatcher(configPath, appConfig)
	configWatcher.OnReload(func(newCfg *config.AppConfig) {
		// Config has been reloaded - the shared appConfig pointer is already updated
		// Additional reload actions can be added here if needed
	})
	configWatcher.Start()
	defer configWatcher.Stop()

	// Per AI.md PART 23: bind privileged port as root BEFORE starting the goroutine
	// so we can drop privileges while still in the main goroutine.
	// This satisfies: "Bind privileged ports as root, then drop"
	listenAddr := appConfig.Server.Address + ":" + appConfig.Server.Port
	listener, err := srv.Listen(listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to bind %s: %v\n", listenAddr, err)
		os.Exit(1)
	}

	// Drop privileges to the vidveil system user after port is bound per AI.md PART 23.
	// ShouldDropPrivileges() returns true only on Unix when current uid == 0.
	if system.ShouldDropPrivileges() {
		if err := system.DropPrivileges(appName); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to drop privileges: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("👤 Dropped privileges to %s\n", appName)
	}

	// Start server goroutine — serves on the pre-bound listener
	go func() {
		// Per AI.md PART 8: Display Rules
		// - Never show: 0.0.0.0, 127.0.0.1, localhost
		// - Show only: One address, the most relevant
		displayAddr := getDisplayAddress(appConfig)

		// Console output per AI.md PART 7
		// First run = settings table is empty (no config rows exist yet)
		isFirstRun := isDBFirstRun(migrationMgr.GetDB())

		// Check SMTP status per AI.md PART 17
		// enabled is determined by SMTP connectivity, not a manual toggle
		smtpInfo := ""
		smtpHost := appConfig.Server.Notifications.Email.SMTP.Host
		smtpPort := appConfig.Server.Notifications.Email.SMTP.Port
		if smtpHost != "" && smtpPort > 0 {
			// Per PART 17: Test configured SMTP on every startup
			if err := email.TestSMTPConfig(smtpHost, smtpPort); err == nil {
				smtpInfo = fmt.Sprintf("%s:%d", smtpHost, smtpPort)
				appConfig.Server.Notifications.Email.Enabled = true
			}
		} else {
			// Per PART 17: Auto-detect on first run if no host configured
			detectedHost, detectedPort := email.AutodetectSMTP(nil, nil)
			if detectedHost != "" && detectedPort > 0 {
				smtpInfo = fmt.Sprintf("%s:%d (auto)", detectedHost, detectedPort)
				appConfig.Server.Notifications.Email.Enabled = true
			}
		}

		// Build URL per AI.md PART 8:
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

		// Print responsive startup banner per AI.md PART 7
		banner.PrintStartupBanner(banner.BannerConfig{
			AppName:   "VidVeil",
			Version:   version.GetVersion(),
			AppMode:   appConfig.Server.Mode,
			Debug:     mode.IsDebugEnabled(),
			URLs:      []string{displayURL},
			ShowSetup: isFirstRun,
		})

		if isFirstRun {
			fmt.Println("[INFO] First run detected. Edit /etc/apimgr/vidveil/server.yml to configure.")
		}

		// Log INFO lines per AI.md PART 11
		fmt.Printf("[INFO] Server started successfully\n")
		fmt.Printf("[INFO] Listening on %s\n", listenAddr)
		if smtpInfo != "" {
			fmt.Printf("[INFO] SMTP configured: %s\n", smtpInfo)
		}
		fmt.Println()

		// Serve on the pre-bound listener (bound before privilege drop above)
		if err := srv.ServeOn(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "❌ Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// DB health monitor — auto-enters/exits maintenance mode per AI.md PART 5/6.
	// Maintenance mode triggers ONLY for DB connection failure or file-write failure.
	// Self-heals continuously (retry every 30s) — no human intervention required.
	maintMgr := maintenance.NewMaintenanceManager(configDir, dataDir, version.GetVersion())
	go func() {
		const healInterval = 30 * time.Second
		inMaintenance := false
		for {
			// Test DB connectivity
			dbErr := migrationMgr.GetDB().Ping()
			// Test file-write ability (write to a probe file in the data dir)
			probeFile := filepath.Join(dataDir, ".write_probe")
			writeErr := os.WriteFile(probeFile, []byte("probe"), 0o600)
			if writeErr == nil {
				os.Remove(probeFile)
			}
			unhealthy := dbErr != nil || writeErr != nil

			if unhealthy && !inMaintenance {
				// Auto-enter maintenance mode
				if enterErr := maintMgr.SetMaintenanceMode(true); enterErr == nil {
					inMaintenance = true
					if dbErr != nil {
						fmt.Fprintf(os.Stderr, "[WARN] DB unavailable (%v) — entering maintenance mode; retrying every %s\n", dbErr, healInterval)
					} else {
						fmt.Fprintf(os.Stderr, "[WARN] File-write failure (%v) — entering maintenance mode; retrying every %s\n", writeErr, healInterval)
					}
				}
			} else if !unhealthy && inMaintenance {
				// Self-heal: condition cleared, exit maintenance mode
				if exitErr := maintMgr.SetMaintenanceMode(false); exitErr == nil {
					inMaintenance = false
					fmt.Println("[INFO] Health restored — exiting maintenance mode")
				}
			}

			time.Sleep(healInterval)
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
		fmt.Fprintf(os.Stderr, "[STATUS] Uptime: %v\n", time.Since(startTime))
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
	// Per AI.md PART 8: Exact --help output format with " - " separator
	binaryName := filepath.Base(os.Args[0])
	fmt.Printf(`%s %s - Privacy-respecting adult video meta search engine

Usage:
  %s [flags]

Information:
-h, --help                             - Show help (--help for any command shows its help)
-v, --version                          - Show version
--status                               - Show server status and health

Shell Integration:
--shell completions [SHELL]            - Print shell completions
--shell init [SHELL]                   - Print shell init command
--shell help                           - Show shell help

Server Configuration:
--mode {production|development}        - Application mode (default: production)
--config DIR                           - Config directory
--data DIR                             - Data directory
--cache DIR                            - Cache directory
--log DIR                              - Log directory
--backup DIR                           - Backup directory
--pid FILE                             - PID file path
--address ADDR                         - Listen address (default: 0.0.0.0)
--port PORT                            - Listen port (default: random 64xxx, 80 in container)
--baseurl PATH                         - URL path prefix (default: /)
--daemon                               - Run as daemon (detach from terminal)
--debug                                - Enable debug mode
--color {auto|yes|no}                  - Color output (default: auto)
--lang CODE                            - Language for output (default: auto)

Service Management:
--service CMD                          - Service management (run --service help for details)
--maintenance CMD                      - Maintenance operations (run --maintenance help for details)
--update [CMD]                         - Check/perform updates (run --update help for details)

Tor Hidden Service:
tor CMD                                - Tor management (run tor help for details)

Run '%s <command> help' for detailed help on any command.
`, binaryName, version.GetVersion(), binaryName, binaryName)
}

func printVersion() {
	// Use main.go build variables per AI.md PART 13: --version Output
	// Per AI.md PART 8: Use actual binary name, not hardcoded
	binaryName := filepath.Base(os.Args[0])
	fmt.Printf("%s %s\n", binaryName, Version)
	fmt.Printf("Commit: %s\n", CommitID)
	fmt.Printf("Built: %s\n", BuildDate)
	fmt.Printf("Go: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if OfficialSite != "" {
		fmt.Printf("Site: %s\n", OfficialSite)
	}
}

func checkStatus() int {
	// Per AI.md PART 31 CLI: exact --status output format
	// Server Status / Port / Mode / Uptime + Tor Hidden Service section
	appPaths := config.GetAppPaths("", "")

	// Try to load config to check if initialized
	statusConfig, _, err := config.LoadAppConfig("", "")
	if err != nil {
		fmt.Println("Server Status: Not initialized")
		fmt.Printf("  Config dir: %s\n", appPaths.Config)
		return 1
	}

	// Try to connect to the server
	addr := net.JoinHostPort("127.0.0.1", statusConfig.Server.Port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		fmt.Println("Server Status: Stopped")
		fmt.Printf("  Port: %s\n", statusConfig.Server.Port)
		return 1
	}
	conn.Close()

	// Server is listening - query /server/healthz for mode, uptime, and Tor status
	health := queryHealthz("", "")
	if health == nil {
		fmt.Println("Server Status: Starting")
		fmt.Printf("  Port: %s\n", statusConfig.Server.Port)
		return 1
	}

	fmt.Println("Server Status: Running")
	fmt.Printf("  Port: %s\n", statusConfig.Server.Port)
	fmt.Printf("  Mode: %s\n", health.Mode)
	fmt.Printf("  Uptime: %s\n", health.Uptime)
	fmt.Println()

	// Per AI.md PART 31: Tor status field is Connected/disabled + onion address
	t := health.Features.Tor
	switch {
	case t.Running:
		fmt.Println("Tor Hidden Service: Connected")
		fmt.Printf("  Address: %s\n", t.Hostname)
	case t.Status == "starting":
		fmt.Println("Tor Hidden Service: Starting")
	default:
		fmt.Println("Tor Hidden Service: Disabled")
	}
	return 0
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
		fmt.Printf(`Shell Integration Commands:
  %s --shell completions [SHELL]   Print shell completions script
  %s --shell init [SHELL]          Print shell init command for eval

Supported Shells:
  bash, zsh, fish, powershell, pwsh, sh, dash, ksh

Examples:
  # Add to ~/.bashrc or ~/.zshrc
  eval "$(%s --shell init)"

  # Or source completions directly
  source <(%s --shell completions bash)
`, binaryName, binaryName, binaryName, binaryName)
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
    local opts="--help --version --shell --config --data --cache --log --backup --pid --address --port --baseurl --mode --status --daemon --debug --color --lang --service --maintenance --update tor"
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
    '--baseurl[URL path prefix]:path:' \
    '--mode[Application mode]:mode:(production development)' \
    '--status[Show status]' \
    '--daemon[Run as daemon]' \
    '--debug[Enable debug mode]' \
    '--color[Color output]:color:(auto yes no)' \
    '--lang[Output language]:code:' \
    '--service[Service command]:command:(start stop restart reload status --install --uninstall --disable)' \
    '--maintenance[Maintenance command]:command:(backup restore update mode setup)' \
    '--update[Update command]:command:(check yes branch)' \
    '1:command:(tor)' \
    '2:tor command:(status validate restart regenerate vanity import-keys help)'
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
complete -c %s -l baseurl -d 'URL path prefix'
complete -c %s -l mode -d 'Application mode' -xa 'production development'
complete -c %s -l status -d 'Show status'
complete -c %s -l daemon -d 'Run as daemon'
complete -c %s -l debug -d 'Enable debug mode'
complete -c %s -l color -d 'Color output' -xa 'auto yes no'
complete -c %s -l lang -d 'Output language'
complete -c %s -l service -d 'Service command' -xa 'start stop restart reload status --install --uninstall --disable'
complete -c %s -l maintenance -d 'Maintenance command' -xa 'backup restore update mode setup'
complete -c %s -l update -d 'Update command' -xa 'check yes branch'
complete -c %s -n '__fish_use_subcommand' -a tor -d 'Tor hidden service management'
complete -c %s -n '__fish_seen_subcommand_from tor' -a 'status validate restart regenerate vanity import-keys help'
`, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName)
}

func printPowerShellCompletions(binaryName string) {
	fmt.Printf(`Register-ArgumentCompleter -Native -CommandName %s -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $completions = @(
        '--help', '--version', '--shell', '--config', '--data', '--cache',
        '--log', '--backup', '--pid', '--address', '--port', '--baseurl', '--mode',
        '--status', '--daemon', '--debug', '--color', '--lang', '--service', '--maintenance', '--update', 'tor'
    )
    $completions | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`, binaryName)
}

func handleServiceCommand(cmd, configDir, dataDir string) {
	// Per AI.md PART 23 and PART 24: Use system.NewServiceManager which creates system user
	// Get binary path
	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	appPaths := config.GetAppPaths(configDir, dataDir)

	// Derive the service name from the binary name so a renamed binary installs
	// a matching service (AI.md PART 23/24 + binary-rules: os.Args[0] determines name).
	appName := filepath.Base(os.Args[0])
	if appName == "" {
		appName = "vidveil"
	}
	if ext := filepath.Ext(appName); ext != "" {
		appName = strings.TrimSuffix(appName, ext)
	}
	if strings.Contains(appName, "-") && !strings.HasPrefix(appName, "vidveil-") {
		appName = "vidveil"
	}

	// Use system.NewServiceManager which handles user creation per AI.md PART 23
	svc := system.NewServiceManager(appName, binaryPath, appPaths.Config, appPaths.Data)

	// Capture raw binary name for user-facing help text (not the service name)
	binaryName := filepath.Base(os.Args[0])

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
		// Per AI.md PART 24: Show service status
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
		// Per AI.md PART 23: Check escalation before service install
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
		// Per AI.md PART 23: Confirmation required before destructive action
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

		// Per AI.md PART 23: Check escalation before service uninstall
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
		fmt.Printf(`Service Management Commands:

  %s --service start         Start the service
  %s --service stop          Stop the service
  %s --service restart       Restart the service
  %s --service reload        Reload configuration
  %s --service status        Show service status
  %s --service --install     Install as system service
  %s --service --uninstall   Uninstall system service
  %s --service --disable     Disable service from starting at boot

Supported service managers:
  - systemd (Linux)
  - runit (Linux)
  - launchd (macOS)
  - Windows Service Manager
  - BSD rc.d
`, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName)

	default:
		fmt.Printf("❌ Unknown service command: %s\n", cmd)
		fmt.Printf("   Run '%s --service --help' for available commands\n", binaryName)
		os.Exit(1)
	}
}

// handleUpdateCommand implements AI.md PART 22 --update command
func handleUpdateCommand(cmd, arg string) {
	binaryName := filepath.Base(os.Args[0])
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
		fmt.Printf(`Update Commands:
  %s --update              Check and perform in-place update with restart
  %s --update yes          Same as --update (default)
  %s --update check        Check for updates without installing
  %s --update branch <name>  Set update branch (stable, beta, daily)

Update Branches:
  stable (default)  Release builds (v*, *.*.*)
  beta              Pre-release builds (*-beta)
  daily             Daily builds (YYYYMMDDHHMM)
`, binaryName, binaryName, binaryName, binaryName)
		os.Exit(0)

	default:
		fmt.Printf("❌ Unknown update command: %s\n", cmd)
		fmt.Printf("\nUsage: %s --update [check|yes|branch <name>|--help]\n\nRun '%s --update --help' for detailed help.\n", binaryName, binaryName)
		os.Exit(1)
	}
}

func handleMaintenanceCommand(cmd, arg, password, configDir, dataDir string) {
	binaryName := filepath.Base(os.Args[0])
	maint := maintenance.NewMaintenanceManager(configDir, dataDir, version.GetVersion())

	switch cmd {
	case "backup":
		// Per AI.md PART 21: Support --password for encrypted backups
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
		// Per AI.md PART 21: Support --password for encrypted backups
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
		// Configuration is entirely via server.yml — no admin web UI exists.
		fmt.Println("VidVeil has no admin web UI. All configuration is via server.yml.")
		fmt.Println("Edit /etc/apimgr/vidveil/server.yml to configure the server.")
		fmt.Printf("Restart the service after making changes: %s --service restart\n", binaryName)

	case "--help", "help", "-h":
		// Per AI.md PART 8: --maintenance --help prints help and exits 0
		fmt.Printf(`Maintenance Commands:
  %s --maintenance backup [file] [--password <pwd>]   Create backup
  %s --maintenance restore [file] [--password <pwd>]  Restore from backup
  %s --maintenance update                              Check and apply updates
  %s --maintenance mode <on|off>                       Enable/disable maintenance mode
  %s --maintenance setup                               Show configuration instructions

Options:
  --password <password>    Encryption password for backup/restore (per AI.md PART 21)

Examples:
  %s --maintenance backup                              # Backup to default location
  %s --maintenance backup --password "secret"          # Encrypted backup
  %s --maintenance backup /tmp/backup.tar              # Backup to specific file
  %s --maintenance restore                             # Restore from most recent
  %s --maintenance restore backup.tar.gz.enc --password "secret"  # Restore encrypted
  %s --maintenance mode on                             # Enable maintenance mode
`, binaryName, binaryName, binaryName, binaryName, binaryName,
			binaryName, binaryName, binaryName, binaryName, binaryName, binaryName)
		os.Exit(0)

	default:
		fmt.Printf("❌ Unknown maintenance command: %s\n", cmd)
		fmt.Printf("\nUsage: %s --maintenance [backup|restore|update|mode|setup|--help]\n\nRun '%s --maintenance --help' for detailed help.\n", binaryName, binaryName)
		os.Exit(1)
	}
}

// isDBFirstRun returns true if the settings table has no rows, indicating first run.
// A missing or inaccessible table also counts as first run.
func isDBFirstRun(db *sql.DB) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count)
	if err != nil {
		return true
	}
	return count == 0
}

func getDisplayAddress(serverConfig *config.AppConfig) string {
	// Per AI.md PART 8: Never show 0.0.0.0, 127.0.0.1, localhost, etc.
	return net.JoinHostPort(config.GetDisplayHost(serverConfig), serverConfig.Server.Port)
}
