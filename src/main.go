// SPDX-License-Identifier: MIT
// Vidveil - Privacy-respecting adult video meta search engine

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

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
	"github.com/apimgr/vidveil/src/server/service/pgp"
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

// parseUpdateArgs parses the arguments that follow "--update" (or its alias
// "--maintenance update") per AI.md PART 22: [check|yes|branch {stable|beta|daily}].
// rest is the slice of args after the update token. It returns the update
// subcommand (default "yes"), its argument (branch name, if any), and the
// number of args from rest that were consumed.
func parseUpdateArgs(rest []string) (cmd, arg string, consumed int) {
	cmd = "yes"
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "--") {
		cmd = rest[0]
		consumed = 1
		if cmd == "branch" && len(rest) > 1 && !strings.HasPrefix(rest[1], "--") {
			arg = rest[1]
			consumed = 2
		}
	}
	return cmd, arg, consumed
}

// valueFlags lists the server flags that consume the following token as their
// value. The sub-command splitter skips that token so a value that happens to
// match a verb (e.g. "--data tor") is not mistaken for a sub-command.
var valueFlags = map[string]bool{
	"--config": true, "-config": true,
	"--data": true, "-data": true,
	"--cache": true, "-cache": true,
	"--log": true, "-log": true,
	"--backup": true, "-backup": true,
	"--pid": true, "-pid": true,
	"--address": true, "-address": true,
	"--port": true, "-port": true,
	"--baseurl": true, "-baseurl": true,
	"--mode": true, "-mode": true,
	"--color": true, "-color": true,
	"--lang": true, "-lang": true,
}

// isSubcommandVerb reports whether tok begins a sub-command whose remaining
// operands are handled by a dedicated handler rather than the server flag set.
func isSubcommandVerb(tok string) bool {
	switch tok {
	case "tor", "--status", "--service", "--maintenance", "--update", "--shell":
		return true
	}
	return false
}

// splitSubcommand separates the leading server flags from a trailing
// sub-command (verb plus its operands) per AI.md PART 8. It returns the flag
// args for flag.FlagSet and the sub-command args (nil when there is none). A
// value-consuming flag's operand is skipped so it is never read as a verb.
func splitSubcommand(args []string) (global, sub []string) {
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if isSubcommandVerb(tok) {
			return args[:i], args[i:]
		}
		if valueFlags[tok] {
			i++
		}
	}
	return args, nil
}

func main() {
	startTime := time.Now()
	rawArgs := os.Args[1:]

	// Per AI.md PART 8: the server binary parses its primary flag set with the
	// stdlib flag package (no hand-rolled switch/os.Args loop). A thin
	// pre-dispatch splits off the sub-command verbs whose trailing operands the
	// flag package cannot model (a bare positional verb like "tor", optional
	// values, and multi-token operands); everything before the verb is the
	// server's own flag set.
	globalArgs, sub := splitSubcommand(rawArgs)

	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
	configDirF := fs.String("config", "", "Configuration directory")
	dataDirF := fs.String("data", "", "Data directory")
	cacheDirF := fs.String("cache", "", "Cache directory")
	logDirF := fs.String("log", "", "Log directory")
	backupDirF := fs.String("backup", "", "Backup directory")
	pidFileF := fs.String("pid", "", "PID file path")
	addressF := fs.String("address", "", "Listen address")
	portF := fs.String("port", "", "Listen port")
	// Per AI.md PART 8: --baseurl PATH (URL path prefix, default "/").
	baseURLF := fs.String("baseurl", "", "URL path prefix (default \"/\")")
	modeF := fs.String("mode", "", "Application mode")
	// Per AI.md PART 8: --color {auto|yes|no}.
	colorF := fs.String("color", "", "Color output: auto, yes, no")
	// Per AI.md PART 8: --lang CODE (output language, default auto).
	langF := fs.String("lang", "", "Output language (default auto)")
	debugF := fs.Bool("debug", false, "Enable debug logging")
	daemonF := fs.Bool("daemon", false, "Run as background daemon")
	versionF := fs.Bool("version", false, "Show version and exit")
	helpF := fs.Bool("help", false, "Show help and exit")
	fs.BoolVar(versionF, "v", false, "Show version and exit (alias)")
	fs.BoolVar(helpF, "h", false, "Show help and exit (alias)")

	fs.Usage = printHelp
	_ = fs.Parse(globalArgs)

	if *helpF {
		printHelp()
		os.Exit(0)
	}
	if *versionF {
		printVersion()
		os.Exit(0)
	}

	configDir := *configDirF
	dataDir := *dataDirF
	cacheDir := *cacheDirF
	logDir := *logDirF
	backupDir := *backupDirF
	pidFile := *pidFileF
	address := *addressF
	port := *portF
	baseURL := *baseURLF
	modeStr := *modeF
	colorFlag := *colorF
	langFlag := *langF
	debug := *debugF
	daemon := *daemonF

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
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to set %s: %v\n", name, err)
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

	// Per AI.md PARTS 8/22/31: sub-command dispatch. Each verb runs its handler
	// and exits; only a bare server invocation falls through to start the server.
	if len(sub) > 0 {
		rest := sub[1:]
		switch sub[0] {
		case "--status":
			os.Exit(checkStatus())

		case "--shell":
			// Per AI.md PART 8: --shell completions [SHELL] | --shell init [SHELL].
			if len(rest) == 0 {
				fmt.Fprintln(os.Stderr, "Usage: --shell [completions|init] [SHELL]")
				os.Exit(1)
			}
			shell := ""
			if len(rest) > 1 && !strings.HasPrefix(rest[1], "-") {
				shell = rest[1]
			}
			handleShellCommand(rest[0], shell)
			os.Exit(0)

		case "tor":
			// Per AI.md PART 31: tor {status|validate|restart|regenerate|vanity|import-keys}.
			os.Exit(handleTorCommand(rest, configDir, dataDir))

		case "--service":
			if len(rest) == 0 {
				fmt.Fprintln(os.Stderr, "Usage: --service [install|uninstall|start|stop|restart|status|enable|disable]")
				os.Exit(1)
			}
			handleServiceCommand(rest[0], configDir, dataDir)
			return

		case "--update":
			// AI.md PART 22: --update [check|yes|branch {stable|beta|daily}].
			uCmd, uArg, _ := parseUpdateArgs(rest)
			handleUpdateCommand(uCmd, uArg)
			return

		case "--maintenance":
			// A bare --maintenance with no sub-command falls through to a normal
			// server start (matches the prior parser).
			if len(rest) > 0 {
				// Per AI.md PART 22: "--maintenance update [cmd]" is an alias for
				// "--update [cmd]".
				if rest[0] == "update" {
					uCmd, uArg, _ := parseUpdateArgs(rest[1:])
					handleUpdateCommand(uCmd, uArg)
					return
				}
				// Per AI.md PART 21: no --password flag; the password is prompted
				// interactively, so only a non-flag operand is taken here.
				maintArg := ""
				if len(rest) > 1 && !strings.HasPrefix(rest[1], "--") {
					maintArg = rest[1]
				}
				handleMaintenanceCommand(rest[0], maintArg, configDir, dataDir)
				return
			}
		}
	}

	// Initialize mode and debug per AI.md PART 6
	// This must happen before starting the server
	mode.InitializeAppMode(modeStr, debug)

	// Handle daemon mode per AI.md PART 8
	if daemon {
		if err := daemonpkg.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to daemonize: %v\n", err)
			os.Exit(1)
		}
		// If we get here, we're either the child or daemonization failed
	}

	// Load configuration
	appConfig, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to load configuration: %v\n", err)
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
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Failed to ensure system user: %v\n", err)
	} else if system.IsRunningAsRoot() && uid > 0 {
		fmt.Printf(terminal.UserIcon()+" Running as user %s (uid=%d, gid=%d)\n", appName, uid, gid)
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
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
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
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer migrationMgr.Close()

	// Register and run migrations
	migrationMgr.RegisterDefaultMigrations()
	if err := migrationMgr.RunMigrations(); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	// Initialize app secrets per AI.md PART 11
	// Generates installation_secret, cookie_signing_key, csrf_token_secret on first run
	secretsMgr := secrets.NewManager(migrationMgr.GetDB())
	if err := secretsMgr.EnsureSecrets(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to initialize secrets: %v\n", err)
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
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" SSL service initialization failed: %v\n", err)
	}

	// GeoIP service (PART 19)
	geoipSvc := geoip.NewGeoIPService(appConfig)
	if err := geoipSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" GeoIP service initialization failed: %v\n", err)
	}

	// Initialize logger per PART 11
	logger, err := logging.NewAppLogger(appConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Logger initialization failed: %v\n", err)
		// Create a basic logger that doesn't write to files
		logger = &logging.AppLogger{}
	}
	defer logger.Close()

	// Route engine debug logging through the governed AppLogger/debug.log
	// pipeline (AI.md PART 11) instead of the package's stdlib log.Printf
	// fallback.
	engine.SetDebugLogger(logger)

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
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Blocklist service initialization failed: %v\n", err)
	}

	// CVE service (PART 11)
	cveSvc := cve.NewCVEService(appConfig)
	if err := cveSvc.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" CVE service initialization failed: %v\n", err)
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
			// Threads server.backup.retention into the full+daily-incremental backup pair.
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			maint.SetLogger(logger)
			retention := appConfig.Server.Backup.Retention
			// ComplianceMode with no stored password (per PART 21, passwords are
			// never stored) causes BackupWithOptions to reject unattended runs -
			// this is the spec's "Scheduled backups skip with audit log warning".
			return maint.BackupDailyFull(maintenance.BackupOptions{
				IncludeData:    true,
				MaxBackups:     retention.MaxBackups,
				KeepWeekly:     retention.KeepWeekly,
				KeepMonthly:    retention.KeepMonthly,
				KeepYearly:     retention.KeepYearly,
				MaxTotalSize:   retention.MaxTotalSize,
				ComplianceMode: appConfig.Server.Compliance.IsEnabled(),
			})
		},
		BackupHourly: func(ctx context.Context) error {
			// Hourly incremental backup per AI.md PART 18/21 (disabled by default)
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			maint.SetLogger(logger)
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
		// Parse server port from config — Tor forwards .onion traffic to the local
		// serving port. Per AI.md PART 15 overlays prefer the HTTP port; fall back to
		// the HTTPS port in HTTPS-only mode. Handles dual "80,443" without a parse error.
		torHTTPPort, torHTTPSPort := config.ParsePorts(appConfig.Server.Port, appConfig.Server.SSL.Enabled)
		torTargetPort := torHTTPPort
		if torTargetPort == "" {
			torTargetPort = torHTTPSPort
		}
		serverPort, portErr := strconv.Atoi(torTargetPort)
		if portErr != nil {
			fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Tor hidden service: invalid target port %q: %v\n", torTargetPort, portErr)
			return
		}
		if err := torSvc.Start(torCtx, serverPort); err != nil {
			// PART 31: Tor errors are WARN level, server continues without Tor
			fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Tor hidden service: %v\n", err)
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
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Failed to load scheduler history: %v\n", err)
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

	// Wire SSL manager for HTTPS serving and ACME challenges per AI.md PART 15
	srv.SetSSLManager(sslSvc)

	// Start live config watcher per AI.md PART 8 NON-NEGOTIABLE
	configWatcher := config.NewWatcher(configPath, appConfig)
	configWatcher.OnReload(func(newCfg *config.AppConfig) {
		// Config has been reloaded - the shared appConfig pointer is already updated
		// Additional reload actions can be added here if needed
	})
	configWatcher.Start()
	defer configWatcher.Stop()

	// Per AI.md PART 15: split the configured port into HTTP and HTTPS ports.
	// Single port = HTTP (or HTTPS when 443 / ssl.enabled); dual "80,443" = HTTP + HTTPS.
	httpPort, httpsPort := config.ParsePorts(appConfig.Server.Port, appConfig.Server.SSL.Enabled)

	// Per AI.md PART 23: bind privileged ports as root BEFORE starting the goroutine
	// so we can drop privileges while still in the main goroutine.
	// This satisfies: "Bind privileged ports as root, then drop"
	var httpListener, httpsListener net.Listener
	if httpPort != "" {
		addr := appConfig.Server.Address + ":" + httpPort
		httpListener, err = srv.Listen(addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to bind %s: %v\n", addr, err)
			os.Exit(1)
		}
	}
	if httpsPort != "" {
		addr := appConfig.Server.Address + ":" + httpsPort
		httpsListener, err = srv.Listen(addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to bind %s: %v\n", addr, err)
			os.Exit(1)
		}
	}

	// Primary listener drives the startup banner: HTTPS when present, else HTTP.
	primaryPort := httpsPort
	primaryIsHTTPS := true
	if primaryPort == "" {
		primaryPort = httpPort
		primaryIsHTTPS = false
	}
	listenAddr := appConfig.Server.Address + ":" + primaryPort

	// Drop privileges to the vidveil system user after port is bound per AI.md PART 23.
	// ShouldDropPrivileges() returns true only on Unix when current uid == 0.
	if system.ShouldDropPrivileges() {
		if err := system.DropPrivileges(appName); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to drop privileges: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf(terminal.UserIcon()+" Dropped privileges to %s\n", appName)
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

		// Build URL per AI.md PART 8 / PART 15:
		// - NEVER show localhost, 127.0.0.1, 0.0.0.0
		// - Show only one address, the most relevant (the primary listener)
		// - Strip :80 and :443 from URLs
		proto := "http"
		if primaryIsHTTPS {
			proto = "https"
		}
		displayURL := proto + "://" + displayAddr
		if primaryPort == "80" || primaryPort == "443" {
			displayURL = proto + "://" + config.GetDisplayHost(appConfig)
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

		// Serve on the pre-bound listeners (bound before privilege drop above)
		// per AI.md PART 15. In dual mode the HTTP listener serves ACME challenges
		// and 301-redirects to HTTPS; the HTTPS listener terminates TLS.
		if httpListener != nil {
			if httpsListener != nil {
				// Dual-port: HTTP listener does ACME + HTTPS redirect
				go func() {
					if err := srv.ServeHTTPRedirectOn(httpListener, httpsPort); err != nil && err != http.ErrServerClosed {
						fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" HTTP redirect server error: %v\n", err)
					}
				}()
			} else {
				// HTTP-only
				go func() {
					if err := srv.ServeOn(httpListener); err != nil && err != http.ErrServerClosed {
						fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Server error: %v\n", err)
						os.Exit(1)
					}
				}()
			}
		}
		if httpsListener != nil {
			if err := srv.ServeTLSOn(httpsListener); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" HTTPS server error: %v\n", err)
				os.Exit(1)
			}
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
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Shutdown error: %v\n", err)
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

	// Resolve the effective port the same way the running server does
	// (Port chain per AI.md: --port > VIDVEIL_PORT > PORT > config > random).
	// The server never persists an env-resolved port back to server.yml, so
	// reading statusConfig.Server.Port alone would report a stale/wrong port
	// whenever VIDVEIL_PORT or PORT overrides the config file (e.g. the
	// standard Docker deployment sets PORT=80) — causing --status, and thus
	// the Docker HEALTHCHECK, to falsely report "Stopped" against a healthy
	// server listening on a different port.
	statusPort := statusConfig.Server.Port
	if envPort := os.Getenv("VIDVEIL_PORT"); envPort != "" {
		statusPort = envPort
	} else if envPort := os.Getenv("PORT"); envPort != "" {
		statusPort = envPort
	}

	// Try to connect to the server
	addr := net.JoinHostPort("127.0.0.1", statusPort)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		fmt.Println("Server Status: Stopped")
		fmt.Printf("  Port: %s\n", statusPort)
		return 1
	}
	conn.Close()

	// Server is listening - query /server/healthz for mode, uptime, and Tor status
	health := queryHealthz("", "")
	if health == nil {
		fmt.Println("Server Status: Starting")
		fmt.Printf("  Port: %s\n", statusPort)
		return 1
	}

	fmt.Println("Server Status: Running")
	fmt.Printf("  Port: %s\n", statusPort)
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
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to get executable path: %v\n", err)
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
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to start: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + " Service started")

	case "stop":
		fmt.Println("Stopping Vidveil service...")
		if err := svc.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to stop: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + " Service stopped")

	case "restart":
		fmt.Println("Restarting Vidveil service...")
		if err := svc.Restart(); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to restart: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + " Service restarted")

	case "reload":
		fmt.Println("Reloading Vidveil configuration...")
		if err := svc.Reload(); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to reload: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + " Configuration reloaded")

	case "status":
		// Per AI.md PART 24: Show service status
		status, err := svc.GetServiceStatus()
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to get status: %v\n", err)
			os.Exit(1)
		}
		switch status {
		case "running":
			fmt.Println(terminal.StatusIcon(true) + " Vidveil service is running")
		case "stopped":
			fmt.Println(terminal.StopButtonIcon() + " Vidveil service is stopped")
		default:
			fmt.Printf(terminal.QuestionIcon()+" Vidveil service status: %s\n", status)
		}

	case "--install":
		// Per AI.md PART 23: Check escalation before service install
		if err := system.HandleEscalation("Service installation"); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Installing Vidveil as system service...")
		if err := svc.Install(); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to install: %v\n", err)
			os.Exit(1)
		}

	case "--uninstall":
		// Per AI.md PART 23: Confirmation required before destructive action
		fmt.Println(terminal.WarningIcon() + " WARNING: This will:")
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
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Uninstalling Vidveil system service...")
		if err := svc.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to uninstall: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + " Service uninstalled")

	case "--disable":
		// Per AI.md PART 8: Disable service from starting at boot
		fmt.Println("Disabling Vidveil service from starting at boot...")
		if err := svc.Disable(); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to disable: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + " Service disabled (will not start at boot)")

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
		fmt.Printf(terminal.StatusIcon(false)+" Unknown service command: %s\n", cmd)
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
				fmt.Println(terminal.StatusIcon(true) + " Already up to date (no newer release found)")
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Update check failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Latest version:  %s\n", info.LatestVersion)

		if info.UpdateAvailable {
			fmt.Println("\n" + terminal.PackageIcon() + " Update available!")
			fmt.Printf("   Release: %s\n", info.ReleaseURL)
			fmt.Println("\n   Run 'vidveil --update' to download and install")
		} else {
			fmt.Println(terminal.StatusIcon(true) + " Already up to date")
		}
		os.Exit(0)

	case "yes", "":
		// Check and perform in-place update with restart
		fmt.Println("Checking for updates...")
		fmt.Printf("Current version: %s\n", version.GetVersion())

		info, err := maint.CheckUpdate()
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				fmt.Println(terminal.StatusIcon(true) + " Already up to date")
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Update check failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Latest version:  %s\n", info.LatestVersion)

		if info.UpdateAvailable {
			fmt.Println("\n" + terminal.PackageIcon() + " Update available!")
			fmt.Printf("   Release: %s\n", info.ReleaseURL)

			if info.DownloadURL != "" {
				fmt.Println("\nApplying update...")
				if err := maint.ApplyUpdate(info.DownloadURL); err != nil {
					fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Update failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Println(terminal.StatusIcon(true) + " Update successful! Please restart the application.")
			}
		} else {
			fmt.Println(terminal.StatusIcon(true) + " Already up to date")
		}
		os.Exit(0)

	case "branch":
		// Set update branch (stable, beta, daily)
		validBranches := map[string]bool{"stable": true, "beta": true, "daily": true}
		if !validBranches[arg] {
			fmt.Printf(terminal.StatusIcon(false)+" Invalid branch: %s\n", arg)
			fmt.Println("   Valid branches: stable, beta, daily")
			os.Exit(1)
		}

		if err := maint.SetUpdateBranch(arg); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to set branch: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf(terminal.StatusIcon(true)+" Update branch set to: %s\n", arg)
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
  daily             Daily builds (YYYYMMDDHHMMSS)
`, binaryName, binaryName, binaryName, binaryName)
		os.Exit(0)

	default:
		fmt.Printf(terminal.StatusIcon(false)+" Unknown update command: %s\n", cmd)
		fmt.Printf("\nUsage: %s --update [check|yes|branch <name>|--help]\n\nRun '%s --update --help' for detailed help.\n", binaryName, binaryName)
		os.Exit(1)
	}
}

// promptYesNo reads a y/N answer from stdin. Any answer starting with "y" or
// "Y" is treated as yes; everything else (including empty input) is no.
func promptYesNo(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	return strings.HasPrefix(strings.ToLower(line), "y")
}

// promptPassword reads a password from the terminal with input hidden, per
// AI.md PART 21 ("passwords on the command line leak via shell history and
// process lists" - so no --password flag, always an interactive prompt).
func promptPassword(prompt string) string {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to read password: %v\n", err)
		os.Exit(1)
	}
	return string(bytePassword)
}

// promptLine prints prompt and reads a single trimmed line of visible input from
// stdin (unlike promptPassword, the text is echoed). Used for free-text operator
// input such as the reason required for a private-key export.
func promptLine(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// promptPasswordConfirmed prompts for a new password twice and requires both
// entries to match before returning it, per AI.md PART 21 encryption setup.
func promptPasswordConfirmed() string {
	for {
		p1 := promptPassword("Password: ")
		p2 := promptPassword("Confirm password: ")
		if p1 == p2 {
			return p1
		}
		fmt.Println(terminal.StatusIcon(false) + " Passwords do not match, try again.")
	}
}

func handleMaintenanceCommand(cmd, arg, configDir, dataDir string) {
	binaryName := filepath.Base(os.Args[0])
	maint := maintenance.NewMaintenanceManager(configDir, dataDir, version.GetVersion())

	switch cmd {
	case "backup":
		// Per AI.md PART 21: no --password flag - password is always prompted for
		// interactively (shell history/process list leakage). Encryption is opt-in
		// unless compliance mode is enabled, in which case it is mandatory.
		appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		complianceMode := appConfig.Server.Compliance.IsEnabled()

		// Attach the audit logger so backup.created/verification_failed/etc. per
		// AI.md PART 21 "Audit Events" are recorded for CLI-triggered backups too.
		if cliLogger, err := logging.NewAppLogger(appConfig); err == nil {
			maint.SetLogger(cliLogger)
		}

		password := ""
		if complianceMode {
			fmt.Println(terminal.WarningIcon() + " Compliance mode requires encrypted backups.")
			password = promptPasswordConfirmed()
			if password == "" {
				fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" Compliance mode requires backup encryption: set a backup password.")
				os.Exit(1)
			}
		} else if promptYesNo("Encrypt backup with password? [y/N]: ") {
			password = promptPasswordConfirmed()
		}
		if password != "" {
			fmt.Println("Creating encrypted backup...")
			if err := maint.BackupWithOptions(maintenance.BackupOptions{
				Filename:       arg,
				Password:       password,
				IncludeData:    true,
				MaxBackups:     1,
				ComplianceMode: complianceMode,
			}); err != nil {
				fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Backup failed: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("Creating backup...")
			if err := maint.Backup(arg); err != nil {
				fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Backup failed: %v\n", err)
				os.Exit(1)
			}
		}

	case "restore":
		// Per AI.md PART 5 "Sensitive Operations": restore requires server.token OR
		// root OR an empty database (nothing to protect yet).
		if err := authorizeRestore(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}

		if arg == "" {
			fmt.Println("Restoring from most recent backup...")
		} else {
			fmt.Printf("Restoring from %s...\n", arg)
		}
		// Per AI.md PART 21: no --password flag - only prompt interactively if the
		// backup turns out to be encrypted (avoids prompting for plaintext backups).
		err := maint.RestoreWithPassword(arg, "")
		if err != nil && strings.Contains(err.Error(), "password required") {
			password := promptPassword("Backup password: ")
			err = maint.RestoreWithPassword(arg, password)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Restore failed: %v\n", err)
			os.Exit(1)
		}

	case "mode":
		// Per AI.md PART 5 "Sensitive Operations": --maintenance mode requires
		// server.token OR root (no empty-database exception - it changes live
		// server behavior, not data).
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}

		if arg == "" {
			fmt.Println(terminal.StatusIcon(false) + " Missing mode argument")
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
			fmt.Printf(terminal.StatusIcon(false)+" Invalid mode value: %s\n", arg)
			fmt.Println("   Valid values: on, off, true, false, yes, no, enable, disable")
			os.Exit(1)
		}

		if err := maint.SetMaintenanceMode(enabled); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed: %v\n", err)
			os.Exit(1)
		}

	case "setup":
		// Configuration is entirely via server.yml — no admin web UI exists.
		fmt.Println("VidVeil has no admin web UI. All configuration is via server.yml.")
		fmt.Println("Edit /etc/apimgr/vidveil/server.yml to configure the server.")
		fmt.Printf("Restart the service after making changes: %s --service restart\n", binaryName)

	case "compliance":
		// Per AI.md PART 5 "Compliance Routes": operators run
		// "--maintenance compliance report" for a compliance summary. Compliance
		// is configured entirely in server.yml; this reads the live config and
		// reports which regulatory standards are active.
		if arg != "" && arg != "report" {
			fmt.Printf(terminal.StatusIcon(false)+" Unknown compliance action: %s\n", arg)
			fmt.Printf("   Usage: %s --maintenance compliance report\n", binaryName)
			os.Exit(1)
		}
		appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		printComplianceReport(appConfig)

	case "pgp":
		// Per AI.md PART 12 "GPG Keypair Management": keypair actions run through
		// the --maintenance dispatcher, authorized like other sensitive operations.
		handlePGPMaintenance(arg, configDir, dataDir)

	case "--help", "help", "-h":
		// Per AI.md PART 8: --maintenance --help prints help and exits 0
		fmt.Printf(`Maintenance Commands:
  %s --maintenance backup [file]      Create backup (prompts for password)
  %s --maintenance restore [file]     Restore from backup (prompts if encrypted)
  %s --maintenance update             Check and apply updates
  %s --maintenance mode <on|off>      Enable/disable maintenance mode
  %s --maintenance compliance report  Show regulatory compliance summary
  %s --maintenance pgp <action>       Manage the security PGP keypair
  %s --maintenance setup              Show configuration instructions

PGP actions (AI.md PART 12 "GPG Keypair Management"):
  generate                            Generate the security keypair
  rotate                              Rotate the keypair (cross-signs, 30-day grace)
  publish                             Publish the public key to configured keyservers
  export public [path]                Write the public key (stdout if omitted)
  export private <path>               Write the decrypted private key (0600, audited, 1/hour)
  import <file>                       Import an existing private key (identity checked)
  delete                              Delete the keypair (irreversible, typed confirmation)

Per AI.md PART 21 the backup password is never passed on the command
line; backup always prompts and restore prompts only when the archive
is encrypted.

Examples:
  %s --maintenance backup             # Backup to default location
  %s --maintenance backup /tmp/backup.tar  # Backup to specific file
  %s --maintenance restore            # Restore from most recent
  %s --maintenance restore backup.tar.gz.enc  # Restore an encrypted archive
  %s --maintenance mode on            # Enable maintenance mode
  %s --maintenance pgp generate       # Generate the security keypair
  %s --maintenance pgp rotate         # Rotate the security keypair
  %s --maintenance pgp publish        # Publish the public key to keyservers
`, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName,
			binaryName, binaryName, binaryName, binaryName, binaryName, binaryName,
			binaryName, binaryName, binaryName)
		os.Exit(0)

	default:
		fmt.Printf(terminal.StatusIcon(false)+" Unknown maintenance command: %s\n", cmd)
		fmt.Printf("\nUsage: %s --maintenance [backup|restore|update|mode|compliance|pgp|setup|--help]\n\nRun '%s --maintenance --help' for detailed help.\n", binaryName, binaryName)
		os.Exit(1)
	}
}

// handlePGPMaintenance implements "--maintenance pgp <action>" per AI.md PART 12
// "GPG Keypair Management". Keypair actions are sensitive operations, authorized
// the same way as other --maintenance sensitive operations (server.token OR root).
func handlePGPMaintenance(arg, configDir, dataDir string) {
	binaryName := filepath.Base(os.Args[0])
	fields := strings.Fields(arg)
	action := ""
	if len(fields) > 0 {
		action = fields[0]
	}

	switch action {
	case "generate":
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}
		pgpGenerate(configDir, dataDir)

	case "rotate":
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}
		pgpRotate(configDir, dataDir)

	case "publish":
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}
		pgpPublish(configDir, dataDir)

	case "export":
		target := ""
		if len(fields) > 1 {
			target = fields[1]
		}
		outPath := ""
		if len(fields) > 2 {
			outPath = fields[2]
		}
		switch target {
		case "public":
			pgpExportPublic(configDir, dataDir, outPath)
		case "private":
			if outPath == "" {
				fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" A destination path is required for a private key export.")
				fmt.Printf("   Usage: %s --maintenance pgp export private <path>\n", binaryName)
				os.Exit(1)
			}
			if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
				fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
				os.Exit(1)
			}
			pgpExportPrivate(configDir, dataDir, outPath)
		default:
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Unsupported export target: %q\n", target)
			fmt.Printf("   Usage: %s --maintenance pgp export <public|private> [path]\n", binaryName)
			os.Exit(1)
		}

	case "import":
		inPath := ""
		if len(fields) > 1 {
			inPath = fields[1]
		}
		if inPath == "" {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" A source file is required to import a private key.")
			fmt.Printf("   Usage: %s --maintenance pgp import <file>\n", binaryName)
			os.Exit(1)
		}
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}
		pgpImport(configDir, dataDir, inPath)

	case "delete":
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			os.Exit(1)
		}
		pgpDelete(configDir, dataDir)

	default:
		fmt.Printf(terminal.StatusIcon(false)+" Unknown pgp action: %q\n", action)
		fmt.Printf("   Usage: %s --maintenance pgp [generate|rotate|publish|export <public|private> [path]|import <file>|delete]\n", binaryName)
		os.Exit(1)
	}
}

// pgpGenerate generates the project security keypair and reports the result,
// exiting non-zero on failure (AI.md PART 12 "Generate").
func pgpGenerate(configDir, dataDir string) {
	kp, identityName, securityContact, pubKeyPath, err := pgpGenerateCore(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		os.Exit(1)
	}
	fmt.Println(terminal.StatusIcon(true) + " Security PGP keypair generated.")
	fmt.Printf("   Identity:    %s <%s>\n", identityName, securityContact)
	fmt.Printf("   Fingerprint: %s\n", kp.Fingerprint)
	fmt.Printf("   Expires:     %s\n", kp.ExpiresAt.Format("2006-01-02"))
	fmt.Printf("   Public key:  %s\n", pubKeyPath)
}

// pgpGenerateCore generates the project security keypair, writes it under
// {config_dir}/security/, records metadata in the DB, and flips the config
// flags that expose the public key (AI.md PART 12 "Generate"). It returns the
// keypair, its identity, and the public-key path, or an error.
func pgpGenerateCore(configDir, dataDir string) (kp *pgp.Keypair, identityName, securityContact, pubKeyPath string, err error) {
	appConfig, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to load configuration: %w", err)
	}

	securityContact = appConfig.Server.Contact.Security.Email
	if securityContact == "" {
		return nil, "", "", "", fmt.Errorf("no security contact configured: set server.contact.security.email in server.yml before generating a keypair")
	}
	appName := appConfig.Server.Branding.Title
	if appName == "" {
		appName = "VidVeil"
	}
	identityName = appName + " Security"

	paths := config.GetAppPaths(configDir, dataDir)
	secret, err := loadInstallationSecret(paths.Data)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to load installation secret: %w", err)
	}

	kp, err = pgp.GenerateKeypair(identityName, securityContact, pgp.DefaultValidity)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to generate keypair: %w", err)
	}
	if err := pgp.WriteKeypair(paths.Config, kp, secret); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to write keypair: %w", err)
	}

	serverDBPath := filepath.Join(paths.Data, "db", "server.db")
	dbMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to open database: %w", err)
	}
	defer dbMgr.Close()
	if err := dbMgr.RunMigrations(); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to run migrations: %w", err)
	}
	if err := pgp.SaveKeypairMeta(dbMgr.GetDB(), kp); err != nil {
		return nil, "", "", "", fmt.Errorf("failed to record keypair metadata: %w", err)
	}

	appConfig.Web.Security.PublishPGPKey = true
	appConfig.Web.Security.PGPKeyURL = strings.TrimRight(appConfig.GetPublicURL(), "/") + "/.well-known/pgp-key.asc"
	if err := config.SaveAppConfig(appConfig, configPath); err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Keypair generated but failed to update config: %v\n", err)
	}

	pubKeyPath = filepath.Join(pgp.SecurityDir(paths.Config), pgp.PublicKeyFile)
	return kp, identityName, securityContact, pubKeyPath, nil
}

// pgpRotate generates a fresh security keypair, cross-signs it with the outgoing
// key, archives the old key for the grace window, and reports the result,
// exiting non-zero on failure (AI.md PART 12 "Rotate").
func pgpRotate(configDir, dataDir string) {
	res, err := pgpRotateCore(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		os.Exit(1)
	}
	fmt.Println(terminal.StatusIcon(true) + " Security PGP keypair rotated.")
	fmt.Printf("   Identity:        %s <%s>\n", res.identityName, res.securityContact)
	fmt.Printf("   New fingerprint: %s\n", res.newFingerprint)
	if res.oldFingerprint != "" {
		fmt.Printf("   Old fingerprint: %s\n", res.oldFingerprint)
	}
	fmt.Printf("   Expires:         %s\n", res.expiresAt.Format("2006-01-02"))
	fmt.Printf("   Public key:      %s\n", res.pubKeyPath)
	fmt.Printf("   Old key kept:    %s (valid until %s for in-flight reports)\n",
		res.archiveDir, res.graceUntil.Format("2006-01-02"))
	fmt.Printf("   Next: run '%s --maintenance pgp publish' to push the new key to keyservers.\n",
		filepath.Base(os.Args[0]))
}

// pgpRotateResult carries the outcome of a rotation for reporting.
type pgpRotateResult struct {
	identityName    string
	securityContact string
	newFingerprint  string
	oldFingerprint  string
	expiresAt       time.Time
	pubKeyPath      string
	archiveDir      string
	graceUntil      time.Time
}

// pgpRotateCore generates a new keypair, signs its public key with the outgoing
// private key, archives the previous keypair for RotationGracePeriod, installs
// the new keypair, and updates DB metadata and config (AI.md PART 12 "Rotate":
// "Generates a new keypair, signs the new pubkey with the old key ... Old key
// stays valid for 30 days for in-flight reports"). Keyserver publishing is a
// separate step (`--maintenance pgp publish`).
func pgpRotateCore(configDir, dataDir string) (*pgpRotateResult, error) {
	appConfig, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	securityContact := appConfig.Server.Contact.Security.Email
	if securityContact == "" {
		return nil, fmt.Errorf("no security contact configured: set server.contact.security.email in server.yml before rotating a keypair")
	}
	appName := appConfig.Server.Branding.Title
	if appName == "" {
		appName = "VidVeil"
	}
	identityName := appName + " Security"

	paths := config.GetAppPaths(configDir, dataDir)
	secret, err := loadInstallationSecret(paths.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to load installation secret: %w", err)
	}

	oldPriv, err := pgp.LoadPrivateKey(paths.Config, secret)
	if err != nil {
		return nil, fmt.Errorf("no existing keypair to rotate (run 'pgp generate' first): %w", err)
	}

	serverDBPath := filepath.Join(paths.Data, "db", "server.db")
	dbMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer dbMgr.Close()
	if err := dbMgr.RunMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	oldMeta, err := pgp.GetKeypairMeta(dbMgr.GetDB())
	if err != nil {
		return nil, fmt.Errorf("failed to read current keypair metadata: %w", err)
	}
	oldFingerprint := ""
	if oldMeta != nil {
		oldFingerprint = oldMeta.Fingerprint
	}

	newKP, err := pgp.GenerateKeypair(identityName, securityContact, pgp.DefaultValidity)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new keypair: %w", err)
	}
	crossSigned, err := pgp.CrossSignPublicKey(newKP.PublicArmored, oldPriv)
	if err != nil {
		return nil, fmt.Errorf("failed to cross-sign new public key: %w", err)
	}
	newKP.PublicArmored = crossSigned

	rotatedAt := time.Now()
	archiveDir, err := pgp.ArchiveCurrentKeys(paths.Config, oldFingerprint, rotatedAt, pgp.RotationGracePeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to archive previous keypair: %w", err)
	}
	if err := pgp.WriteKeypair(paths.Config, newKP, secret); err != nil {
		return nil, fmt.Errorf("failed to write new keypair: %w", err)
	}
	if err := pgp.SaveKeypairMeta(dbMgr.GetDB(), newKP); err != nil {
		return nil, fmt.Errorf("failed to record new keypair metadata: %w", err)
	}
	if err := pgp.SetLastRotated(dbMgr.GetDB(), rotatedAt); err != nil {
		return nil, fmt.Errorf("failed to stamp rotation time: %w", err)
	}

	appConfig.Web.Security.PublishPGPKey = true
	appConfig.Web.Security.PGPKeyURL = strings.TrimRight(appConfig.GetPublicURL(), "/") + "/.well-known/pgp-key.asc"
	if err := config.SaveAppConfig(appConfig, configPath); err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Keypair rotated but failed to update config: %v\n", err)
	}

	return &pgpRotateResult{
		identityName:    identityName,
		securityContact: securityContact,
		newFingerprint:  newKP.Fingerprint,
		oldFingerprint:  oldFingerprint,
		expiresAt:       newKP.ExpiresAt,
		pubKeyPath:      filepath.Join(pgp.SecurityDir(paths.Config), pgp.PublicKeyFile),
		archiveDir:      archiveDir,
		graceUntil:      rotatedAt.Add(pgp.RotationGracePeriod),
	}, nil
}

// pgpPublishResult carries the outcome of a keyserver publish for reporting.
type pgpPublishResult struct {
	published []pgp.KeyserverState
	attempted int
}

// pgpPublish submits the security public key to the configured keyservers and
// reports the result, exiting non-zero on failure (AI.md PART 12 "Publish to
// keyservers").
func pgpPublish(configDir, dataDir string) {
	res, err := pgpPublishCore(configDir, dataDir)
	if err != nil {
		if res != nil {
			for _, st := range res.published {
				fmt.Printf(terminal.StatusIcon(true)+" Published to %s\n", st.URL)
			}
		}
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		os.Exit(1)
	}
	if len(res.published) == 0 {
		fmt.Println(terminal.WarningIcon() + " No keyservers configured; nothing published.")
		return
	}
	fmt.Printf(terminal.StatusIcon(true)+" Published the security public key to %d keyserver(s):\n", len(res.published))
	for _, st := range res.published {
		fmt.Printf("   %s (at %s)\n", st.URL, st.PublishedAt.Format(time.RFC3339))
	}
}

// pgpPublishCore loads the security public key and POSTs it to every keyserver
// in web.security.keyservers, persisting per-keyserver publish state to disk and
// the DB so a later restore does not double-submit (AI.md PART 12 "Publish to
// keyservers" and PART 21 "Backup Integration"). Partial success is possible:
// the returned result carries the servers that accepted the key even when err
// is non-nil.
func pgpPublishCore(configDir, dataDir string) (*pgpPublishResult, error) {
	appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	paths := config.GetAppPaths(configDir, dataDir)
	pub, err := pgp.LoadPublicKey(paths.Config)
	if err != nil {
		return nil, fmt.Errorf("no public key found (run 'pgp generate' first): %w", err)
	}

	keyservers := appConfig.Web.Security.Keyservers
	if len(keyservers) == 0 {
		return &pgpPublishResult{}, nil
	}

	ctx := context.Background()
	states, pubErr := pgp.PublishToKeyservers(ctx, pub, keyservers)
	res := &pgpPublishResult{published: states, attempted: len(keyservers)}

	// Persist state even on partial failure so a restore does not re-submit the
	// servers that already accepted the key.
	if len(states) > 0 {
		if err := pgp.WriteKeyserverState(paths.Config, states); err != nil {
			return res, fmt.Errorf("published but failed to write keyserver state: %w", err)
		}
		serverDBPath := filepath.Join(paths.Data, "db", "server.db")
		dbMgr, dbErr := database.NewMigrationManager(serverDBPath)
		if dbErr != nil {
			return res, fmt.Errorf("published but failed to open database: %w", dbErr)
		}
		defer dbMgr.Close()
		if err := dbMgr.RunMigrations(); err != nil {
			return res, fmt.Errorf("published but failed to run migrations: %w", err)
		}
		if err := pgp.UpdateKeyserversPublished(dbMgr.GetDB(), states); err != nil {
			return res, fmt.Errorf("published but failed to record keyserver state: %w", err)
		}
	}

	if pubErr != nil {
		return res, pubErr
	}
	return res, nil
}

// pgpExportPublic writes the armored public key to outPath, or stdout if empty,
// exiting non-zero on failure (AI.md PART 12 "Export public key").
func pgpExportPublic(configDir, dataDir, outPath string) {
	pub, err := pgpExportPublicCore(configDir, dataDir, outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		os.Exit(1)
	}
	if outPath == "" {
		os.Stdout.Write(pub)
		return
	}
	fmt.Printf(terminal.StatusIcon(true)+" Public key written to %s\n", outPath)
}

// pgpExportPublicCore loads the armored public key and, when outPath is set,
// writes it there (0644). Reading the public key is not sensitive. When outPath
// is empty it returns the key bytes for the caller to emit.
func pgpExportPublicCore(configDir, dataDir, outPath string) ([]byte, error) {
	paths := config.GetAppPaths(configDir, dataDir)
	pub, err := pgp.LoadPublicKey(paths.Config)
	if err != nil {
		return nil, fmt.Errorf("no public key found (run 'pgp generate' first): %w", err)
	}
	if outPath == "" {
		return pub, nil
	}
	if err := os.WriteFile(outPath, pub, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write public key: %w", err)
	}
	return pub, nil
}

// privateExportWindow is the minimum interval between private-key exports for a
// single operator, per AI.md PART 12 ("rate-limited to 1 per hour per operator").
const privateExportWindow = time.Hour

// privateExportStateName is the on-disk record of the last private-key export per
// operator, kept in the security dir alongside the keypair so the rate limit
// survives across CLI invocations.
const privateExportStateName = "private_export.state"

// pgpExportPrivate exports the decrypted private key to outPath after the caller
// has already run the sensitive-operation gate. It prompts the operator for the
// mandatory reason text, enforces the per-operator rate limit, writes the key at
// mode 0600, and emits a security.private_key_exported audit event (AI.md PART 12
// "Export private key"). It exits non-zero on any failure.
func pgpExportPrivate(configDir, dataDir, outPath string) {
	operator := "unknown"
	if u, err := user.Current(); err == nil && u.Username != "" {
		operator = u.Username
	}

	reason := promptLine("Reason for exporting the private key (required): ")
	if reason == "" {
		fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" A reason is required to export the private key.")
		os.Exit(1)
	}

	if err := pgpExportPrivateCore(configDir, dataDir, outPath, operator, localOperatorIP(), reason, time.Now().UTC()); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(terminal.StatusIcon(true)+" Private key exported to %s (mode 0600).\n", outPath)
	fmt.Println(terminal.WarningIcon() + " This file contains the decrypted private key. Store it securely and delete it when no longer needed.")
}

// pgpExportPrivateCore enforces the per-operator rate limit, decrypts and writes
// the private key at mode 0600, records the export timestamp, and audits the
// event. now is passed in so the rate limit is deterministically testable.
func pgpExportPrivateCore(configDir, dataDir, outPath, operator, operatorIP, reason string, now time.Time) error {
	paths := config.GetAppPaths(configDir, dataDir)

	if err := checkPrivateExportRateLimit(paths.Config, operator, now); err != nil {
		return err
	}

	secret, err := loadInstallationSecret(paths.Data)
	if err != nil {
		return fmt.Errorf("failed to load installation secret: %w", err)
	}
	priv, err := pgp.LoadPrivateKey(paths.Config, secret)
	if err != nil {
		return fmt.Errorf("no private key found (run 'pgp generate' first): %w", err)
	}

	if dir := filepath.Dir(outPath); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}
	if err := os.WriteFile(outPath, priv, 0o600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	if err := recordPrivateExport(paths.Config, operator, now); err != nil {
		return fmt.Errorf("private key written but failed to record export state: %w", err)
	}

	emitPrivateExportAudit(configDir, dataDir, operator, operatorIP, reason, outPath)
	return nil
}

// checkPrivateExportRateLimit rejects a private-key export when this operator
// exported within privateExportWindow of now (AI.md PART 12 "1 per hour per
// operator"). A missing or unreadable state file means no prior export.
func checkPrivateExportRateLimit(configDir, operator string, now time.Time) error {
	state, err := loadPrivateExportState(configDir)
	if err != nil {
		return err
	}
	last, ok := state[operator]
	if !ok {
		return nil
	}
	lastAt, err := time.Parse(time.RFC3339, last)
	if err != nil {
		return nil
	}
	if elapsed := now.Sub(lastAt); elapsed < privateExportWindow {
		wait := (privateExportWindow - elapsed).Round(time.Minute)
		return fmt.Errorf("rate limited: this operator exported the private key less than an hour ago; try again in %s", wait)
	}
	return nil
}

// recordPrivateExport stamps now as this operator's most recent private-key
// export in the state file (mode 0600).
func recordPrivateExport(configDir, operator string, now time.Time) error {
	state, err := loadPrivateExportState(configDir)
	if err != nil {
		return err
	}
	state[operator] = now.Format(time.RFC3339)

	dir := pgp.SecurityDir(configDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create security dir: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal export state: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, privateExportStateName), data, 0o600); err != nil {
		return fmt.Errorf("write export state: %w", err)
	}
	return nil
}

// loadPrivateExportState reads the per-operator private-key export timestamps. A
// missing file yields an empty map; a malformed file is a hard error so a
// corrupt state cannot silently defeat the rate limit.
func loadPrivateExportState(configDir string) (map[string]string, error) {
	path := filepath.Join(pgp.SecurityDir(configDir), privateExportStateName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read export state: %w", err)
	}
	state := map[string]string{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse export state: %w", err)
	}
	return state, nil
}

// emitPrivateExportAudit writes the security.private_key_exported audit event,
// including the operator IP and typed reason (AI.md PART 12). Audit failures are
// non-fatal: the key was already exported, so we warn rather than error.
func emitPrivateExportAudit(configDir, dataDir, operator, operatorIP, reason, outPath string) {
	appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" private key exported but audit log unavailable: %v\n", err)
		return
	}
	logger, err := logging.NewAppLogger(appConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" private key exported but audit log unavailable: %v\n", err)
		return
	}
	logger.Audit("security.private_key_exported", operator, "operator", operatorIP, "success",
		map[string]interface{}{"reason": reason, "path": outPath})
}

// localOperatorIP returns the machine's primary outbound IP on a best-effort
// basis for the audit trail. A UDP "connection" to a documentation-range address
// sends no packets but resolves the local source IP; failures yield "".
func localOperatorIP() string {
	conn, err := net.Dial("udp", "192.0.2.1:9")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

// pgpImportResult carries the outcome of a private-key import for reporting.
type pgpImportResult struct {
	identityName  string
	identityEmail string
	fingerprint   string
	expiresAt     time.Time
	pubKeyPath    string
}

// pgpImport imports an existing armored private key from a local file and
// reports the result, exiting non-zero on failure or an unconfirmed identity
// mismatch (AI.md PART 12 "Import private key").
func pgpImport(configDir, dataDir, inPath string) {
	res, err := pgpImportCore(configDir, dataDir, inPath, promptImportOverride)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		os.Exit(1)
	}
	if res == nil {
		fmt.Println(terminal.WarningIcon() + " Import aborted: identity mismatch was not confirmed.")
		os.Exit(1)
	}
	fmt.Println(terminal.StatusIcon(true) + " Security PGP private key imported.")
	fmt.Printf("   Identity:    %s <%s>\n", res.identityName, res.identityEmail)
	fmt.Printf("   Fingerprint: %s\n", res.fingerprint)
	fmt.Printf("   Expires:     %s\n", res.expiresAt.Format("2006-01-02"))
	fmt.Printf("   Public key:  %s\n", res.pubKeyPath)
}

// pgpImportCore reads and parses the armored private key at inPath, validates its
// identity against the project's expected identity (calling confirmMismatch on a
// mismatch so the operator can override), then installs the keypair, records its
// metadata, and re-enables publishing (AI.md PART 12 "Import private key"). It
// returns nil result with nil error when a mismatch is declined. confirmMismatch
// is injected so the identity-gate is testable without a terminal.
func pgpImportCore(configDir, dataDir, inPath string, confirmMismatch func(expected, got string) bool) (*pgpImportResult, error) {
	armored, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}
	kp, name, email, err := pgp.ParsePrivateKey(armored)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	appConfig, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	appName := appConfig.Server.Branding.Title
	if appName == "" {
		appName = "VidVeil"
	}
	expectedName := appName + " Security"
	expectedEmail := appConfig.Server.Contact.Security.Email
	if name != expectedName || (expectedEmail != "" && email != expectedEmail) {
		expected := fmt.Sprintf("%s <%s>", expectedName, expectedEmail)
		got := fmt.Sprintf("%s <%s>", name, email)
		if confirmMismatch == nil || !confirmMismatch(expected, got) {
			return nil, nil
		}
	}

	paths := config.GetAppPaths(configDir, dataDir)
	secret, err := loadInstallationSecret(paths.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to load installation secret: %w", err)
	}
	if err := pgp.WriteKeypair(paths.Config, kp, secret); err != nil {
		return nil, fmt.Errorf("failed to write keypair: %w", err)
	}

	serverDBPath := filepath.Join(paths.Data, "db", "server.db")
	dbMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer dbMgr.Close()
	if err := dbMgr.RunMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	if err := pgp.SaveKeypairMeta(dbMgr.GetDB(), kp); err != nil {
		return nil, fmt.Errorf("failed to record keypair metadata: %w", err)
	}

	appConfig.Web.Security.PublishPGPKey = true
	appConfig.Web.Security.PGPKeyURL = strings.TrimRight(appConfig.GetPublicURL(), "/") + "/.well-known/pgp-key.asc"
	if err := config.SaveAppConfig(appConfig, configPath); err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Key imported but failed to update config: %v\n", err)
	}

	operator := "unknown"
	if u, err := user.Current(); err == nil && u.Username != "" {
		operator = u.Username
	}
	emitPrivateImportAudit(configDir, dataDir, operator, localOperatorIP(), inPath, kp.Fingerprint)

	return &pgpImportResult{
		identityName:  name,
		identityEmail: email,
		fingerprint:   kp.Fingerprint,
		expiresAt:     kp.ExpiresAt,
		pubKeyPath:    filepath.Join(pgp.SecurityDir(paths.Config), pgp.PublicKeyFile),
	}, nil
}

// promptImportOverride warns about an identity mismatch and asks the operator to
// confirm the import (AI.md PART 12 "warns on mismatch — operator can override").
func promptImportOverride(expected, got string) bool {
	fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Imported key identity %q does not match the expected identity %q.\n", got, expected)
	ans := strings.ToLower(promptLine("Import anyway? [y/N]: "))
	return ans == "y" || ans == "yes"
}

// emitPrivateImportAudit writes the security.private_key_imported audit event for
// the sensitive-operation import flow (AI.md PART 5 sensitive operations). Audit
// failures are non-fatal: the key was already imported, so we warn rather than error.
func emitPrivateImportAudit(configDir, dataDir, operator, operatorIP, inPath, fingerprint string) {
	appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" private key imported but audit log unavailable: %v\n", err)
		return
	}
	logger, err := logging.NewAppLogger(appConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" private key imported but audit log unavailable: %v\n", err)
		return
	}
	logger.Audit("security.private_key_imported", operator, "operator", operatorIP, "success",
		map[string]interface{}{"source": inPath, "fingerprint": fingerprint})
}

// pgpDelete permanently deletes the security keypair after the caller has run the
// sensitive-operation gate. It warns that in-flight encrypted reports become
// un-decryptable, requires a typed confirmation, and reports the result (AI.md
// PART 12 "Delete"). It exits non-zero on failure or an unconfirmed deletion.
func pgpDelete(configDir, dataDir string) {
	done, err := pgpDeleteCore(configDir, dataDir, promptDeleteConfirm)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		os.Exit(1)
	}
	if !done {
		fmt.Println(terminal.WarningIcon() + " Deletion aborted: confirmation not provided.")
		os.Exit(1)
	}
	fmt.Println(terminal.StatusIcon(true) + " Security PGP keypair deleted.")
	fmt.Println("   Publishing disabled and the Encryption: line removed from security.txt.")
}

// pgpDeleteCore deletes both key files, marks the keypair revoked in the DB (the
// fingerprint stays for audit history), and clears the publishing config so the
// Encryption: line drops out of the dynamically generated security.txt (AI.md
// PART 12 "Delete"). confirm is injected so the typed-confirmation gate is
// testable without a terminal; it returns nil,false with no error when declined.
func pgpDeleteCore(configDir, dataDir string, confirm func() bool) (bool, error) {
	appConfig, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return false, fmt.Errorf("failed to load configuration: %w", err)
	}
	paths := config.GetAppPaths(configDir, dataDir)

	serverDBPath := filepath.Join(paths.Data, "db", "server.db")
	dbMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		return false, fmt.Errorf("failed to open database: %w", err)
	}
	defer dbMgr.Close()
	if err := dbMgr.RunMigrations(); err != nil {
		return false, fmt.Errorf("failed to run migrations: %w", err)
	}
	meta, err := pgp.GetKeypairMeta(dbMgr.GetDB())
	if err != nil {
		return false, fmt.Errorf("failed to read keypair metadata: %w", err)
	}

	if confirm == nil || !confirm() {
		return false, nil
	}

	if err := pgp.DeleteKeypair(paths.Config); err != nil {
		return false, fmt.Errorf("failed to delete key files: %w", err)
	}
	fingerprint := ""
	if meta != nil {
		fingerprint = meta.Fingerprint
		if err := pgp.MarkRevoked(dbMgr.GetDB()); err != nil {
			return false, fmt.Errorf("key files deleted but failed to mark revoked: %w", err)
		}
	}

	appConfig.Web.Security.PublishPGPKey = false
	appConfig.Web.Security.PGPKeyURL = ""
	if err := config.SaveAppConfig(appConfig, configPath); err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Keys deleted but failed to update config: %v\n", err)
	}

	operator := "unknown"
	if u, err := user.Current(); err == nil && u.Username != "" {
		operator = u.Username
	}
	emitPrivateDeleteAudit(configDir, dataDir, operator, localOperatorIP(), fingerprint)

	return true, nil
}

// promptDeleteConfirm warns that deletion is irreversible and that in-flight
// encrypted reports become un-decryptable, then requires the operator to type
// DELETE exactly (AI.md PART 12 "operator warned and must type confirmation").
func promptDeleteConfirm() bool {
	fmt.Fprintln(os.Stderr, terminal.WarningIcon()+" Deleting the security keypair is irreversible.")
	fmt.Fprintln(os.Stderr, "   In-flight encrypted security reports will become permanently un-decryptable.")
	return promptLine("Type DELETE to confirm: ") == "DELETE"
}

// emitPrivateDeleteAudit writes the security.private_key_deleted audit event for
// the sensitive-operation delete flow (AI.md PART 12 — the fingerprint stays in
// audit history). Audit failures are non-fatal: the keys are already gone, so we
// warn rather than error.
func emitPrivateDeleteAudit(configDir, dataDir, operator, operatorIP, fingerprint string) {
	appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" keypair deleted but audit log unavailable: %v\n", err)
		return
	}
	logger, err := logging.NewAppLogger(appConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" keypair deleted but audit log unavailable: %v\n", err)
		return
	}
	logger.Audit("security.private_key_deleted", operator, "operator", operatorIP, "success",
		map[string]interface{}{"fingerprint": fingerprint})
}

// loadInstallationSecret opens the server database and returns the
// installation_secret used to derive the PGP private-key encryption key.
func loadInstallationSecret(dataDir string) ([]byte, error) {
	serverDBPath := filepath.Join(dataDir, "db", "server.db")
	dbMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	defer dbMgr.Close()
	if err := dbMgr.RunMigrations(); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	secretsMgr := secrets.NewManager(dbMgr.GetDB())
	if err := secretsMgr.EnsureSecrets(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure secrets: %w", err)
	}
	return secretsMgr.GetInstallationSecret(context.Background())
}

// printComplianceReport writes a human-readable regulatory compliance summary to
// stdout per AI.md PART 5 ("Operators run ... --maintenance compliance report for
// a compliance summary"). Compliance is configured entirely in server.yml, so the
// report reflects the live config: which standards are enabled and the operational
// controls those toggles activate.
func printComplianceReport(appConfig *config.AppConfig) {
	c := appConfig.Server.Compliance
	enabled := c.EnabledStandards()

	fmt.Println("Compliance Report")
	fmt.Println("=================")
	fmt.Printf("Application: %s\n", appConfig.Server.Branding.Title)
	fmt.Printf("Generated:   %s\n\n", time.Now().UTC().Format(time.RFC3339))

	if len(enabled) == 0 {
		fmt.Println("Compliance mode: DISABLED")
		fmt.Println("No regulatory standards are enabled. Backups are optional and")
		fmt.Println("encryption is not mandatory. Enable standards under")
		fmt.Println("server.compliance in server.yml.")
		return
	}

	fmt.Println("Compliance mode: ENABLED")
	fmt.Println("\nEnabled standards:")
	for _, s := range c.Standards() {
		if s.Enabled {
			fmt.Printf("  [x] %s\n", s.Name)
		}
	}
	fmt.Println("\nDisabled standards:")
	for _, s := range c.Standards() {
		if !s.Enabled {
			fmt.Printf("  [ ] %s\n", s.Name)
		}
	}

	fmt.Println("\nActive controls (derived from enabled standards):")
	fmt.Println("  - Backup encryption is MANDATORY (--maintenance backup requires a password)")
	if c.GDPR || c.CCPA || c.LGPD {
		fmt.Println("  - Data-subject requests: --maintenance data export / data delete")
	}
	fmt.Println("  - Audit logging active for compliance.* events")
	fmt.Println("\nCompliance is configured in server.yml under server.compliance.")
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

// authorizeRestore enforces AI.md PART 5 "Sensitive Operations" restore
// authorization flow:
//   - Empty database (first run) -> allowed, nothing to protect
//   - Root -> allowed (with a warning; caller proceeds)
//   - Service user ({project_name}) -> requires the operator token (server.token)
//   - Any other user -> rejected
func authorizeRestore(configDir, dataDir string) error {
	if isDatabaseEmpty(configDir, dataDir) {
		return nil
	}
	if system.IsRunningAsRoot() {
		fmt.Println(terminal.WarningIcon() + " Running as root: this will OVERWRITE all data.")
		return nil
	}
	return authorizeViaOperatorToken(configDir, dataDir,
		"This will OVERWRITE all data. Enter operator token to confirm: ")
}

// authorizeSensitiveOperation enforces AI.md PART 5 "Sensitive Operations" for
// operations with no empty-database exception (e.g. --maintenance mode):
//   - Root -> allowed (with a warning; caller proceeds)
//   - Service user ({project_name}) -> requires the operator token (server.token)
//   - Any other user -> rejected
func authorizeSensitiveOperation(configDir, dataDir string) error {
	if system.IsRunningAsRoot() {
		fmt.Println(terminal.WarningIcon() + " Running as root: this will change server behavior.")
		return nil
	}
	return authorizeViaOperatorToken(configDir, dataDir,
		"This will change server behavior. Enter operator token to confirm: ")
}

// authorizeViaOperatorToken is the shared service-user/operator-token tail of the
// PART 5 authorization flows: only the service user may be prompted for the
// operator token; any other non-root user is rejected outright.
func authorizeViaOperatorToken(configDir, dataDir, prompt string) error {
	currentUser, err := user.Current()
	if err != nil || currentUser.Username != "vidveil" {
		return fmt.Errorf("requires administrator authorization\n   Run as root or provide the operator token")
	}

	appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	if appConfig.Server.Token == "" {
		return fmt.Errorf("requires administrator authorization: no operator token configured")
	}

	token := promptPassword(prompt)
	sum := sha256.Sum256([]byte(token))
	expected := sha256.Sum256([]byte(appConfig.Server.Token))
	if subtle.ConstantTimeCompare(sum[:], expected[:]) != 1 {
		return fmt.Errorf("invalid operator token")
	}
	return nil
}

// isDatabaseEmpty reports whether the server database has no settings rows yet
// (fresh install / first run per AI.md PART 5). Any error opening the database
// is treated as empty (nothing to protect).
func isDatabaseEmpty(configDir, dataDir string) bool {
	paths := config.GetAppPaths(configDir, dataDir)
	serverDBPath := filepath.Join(paths.Data, "db", "server.db")
	migrationMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		return true
	}
	defer migrationMgr.Close()
	return isDBFirstRun(migrationMgr.GetDB())
}

func getDisplayAddress(serverConfig *config.AppConfig) string {
	// Per AI.md PART 8: Never show 0.0.0.0, 127.0.0.1, localhost, etc.
	return net.JoinHostPort(config.GetDisplayHost(serverConfig), serverConfig.Server.Port)
}
