// SPDX-License-Identifier: MIT
// Vidveil - Privacy-respecting adult video meta search engine

package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/mode"
	pathpkg "github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server"
	serverbanner "github.com/apimgr/vidveil/src/server/banner"
	daemonpkg "github.com/apimgr/vidveil/src/server/daemon"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/blocklist"
	"github.com/apimgr/vidveil/src/server/service/cve"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/email"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/geoip"
	"github.com/apimgr/vidveil/src/server/service/health"
	"github.com/apimgr/vidveil/src/server/service/i2p"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/maintenance"
	svcmetrics "github.com/apimgr/vidveil/src/server/service/metric"
	"github.com/apimgr/vidveil/src/server/service/pgp"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
	"github.com/apimgr/vidveil/src/server/service/ssl"
	"github.com/apimgr/vidveil/src/server/service/system"
	"github.com/apimgr/vidveil/src/server/service/tor"
	signalpkg "github.com/apimgr/vidveil/src/server/signal"
)

func pingDatabaseForRecovery(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is unavailable")
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		lastErr = db.PingContext(ctx)
		cancel()
		if lastErr == nil {
			return nil
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
	return fmt.Errorf("database unavailable after 3 recovery probes: %w", lastErr)
}

func buildEpoch() int64 {
	if loadedServerConfig == nil {
		return 0
	}
	epoch, err := strconv.ParseInt(loadedServerConfig.BuildEpoch, 10, 64)
	if err != nil {
		return 0
	}
	return epoch
}

func probeDataDirectoryWrite(dataDir string) error {
	probe, err := os.CreateTemp(dataDir, ".vidveil-write-probe-*")
	if err != nil {
		return fmt.Errorf("create data-directory write probe: %w", err)
	}
	probePath := probe.Name()
	if err := probe.Chmod(0600); err != nil {
		_ = probe.Close()
		_ = os.Remove(probePath)
		return fmt.Errorf("secure data-directory write probe: %w", err)
	}
	if _, err := probe.Write([]byte("probe")); err != nil {
		_ = probe.Close()
		_ = os.Remove(probePath)
		return fmt.Errorf("write data-directory probe: %w", err)
	}
	if err := probe.Sync(); err != nil {
		_ = probe.Close()
		_ = os.Remove(probePath)
		return fmt.Errorf("sync data-directory probe: %w", err)
	}
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return fmt.Errorf("close data-directory probe: %w", err)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("remove data-directory write probe: %w", err)
	}
	return nil
}

func maintenanceDiagnostic(databaseErr, fileWriteErr error) string {
	return errors.Join(databaseErr, fileWriteErr).Error()
}

func applySchedulerConfig(sched *scheduler.Scheduler, appConfig *config.AppConfig, logger *logging.AppLogger) {
	if zone := appConfig.Server.Schedule.Timezone; zone != "" {
		if err := sched.SetTimezone(zone); err != nil {
			logger.Warn("invalid server.scheduler.timezone; using America/New_York", map[string]interface{}{"key": "server.scheduler.timezone", "value": zone, "default": "America/New_York", "error": err.Error()})
			if fallbackErr := sched.SetTimezone("America/New_York"); fallbackErr != nil {
				logger.Warn("America/New_York scheduler timezone unavailable; using UTC", map[string]interface{}{"key": "server.scheduler.timezone", "default": "UTC", "error": fallbackErr.Error()})
				_ = sched.SetTimezone("UTC")
			}
		}
	}
	if window := appConfig.Server.Schedule.CatchUpWindow; window != "" {
		if duration, err := time.ParseDuration(window); err == nil {
			sched.SetCatchUpWindow(duration)
		} else {
			logger.Warn("invalid server.scheduler.catch_up_window; retaining current value", map[string]interface{}{"key": "server.scheduler.catch_up_window", "value": window, "error": err.Error()})
		}
	}
	for id, taskConfig := range appConfig.Server.Schedule.Tasks {
		schedule := taskConfig.Schedule
		if schedule == "" {
			continue
		}
		if err := sched.SetSchedule(id, schedule); err != nil {
			if strings.Contains(err.Error(), "task not found:") {
				continue
			}
			logger.Warn("invalid configured scheduler task schedule", map[string]interface{}{"task_id": id, "schedule": schedule, "error": err.Error()})
			continue
		}
		if err := sched.SetRetryPolicy(id, taskConfig.RetryOnFail, taskConfig.RetryDelay); err != nil {
			if !strings.Contains(err.Error(), "task not found:") {
				logger.Warn("invalid configured scheduler retry policy", map[string]interface{}{"task_id": id, "error": err.Error()})
			}
			continue
		}
		enabled := taskConfig.Enabled
		switch id {
		case "database_check":
			enabled = enabled && appConfig.Server.Notifications.DatabaseError.Enabled
		case "disk_check":
			enabled = enabled && appConfig.Server.Notifications.DiskLow.Enabled
		}
		var err error
		if enabled {
			err = sched.EnableTask(id)
		} else {
			err = sched.DisableTask(id)
		}
		if err != nil && !strings.Contains(err.Error(), "task not found:") {
			logger.Warn("failed to apply configured scheduler task state", map[string]interface{}{"task_id": id, "enabled": enabled, "error": err.Error()})
		}
	}
}

func formatServerStartupTime(started time.Time) string {
	zone := os.Getenv("TZ")
	if zone == "" {
		zone = "America/New_York"
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.Local
	}
	return started.In(location).Format("January 02, 2006 at 15:04:05 MST")
}

func openAuditMirrorDatabase(appConfig *config.AppConfig, configDir, dataDir string) (*database.SchemaManager, error) {
	appPaths := config.GetAppPaths(configDir, dataDir)
	sqliteDir := appConfig.Server.Database.SQLite.Dir
	if sqliteDir == "" {
		sqliteDir = config.GetDatabaseDir(appPaths.Data)
	}
	serverDB := appConfig.Server.Database.SQLite.ServerDB
	if serverDB == "" {
		serverDB = "server.db"
	}
	return database.NewSchemaManagerWithConfig(database.DatabaseConfig{
		Driver:      database.Driver(appConfig.Server.Database.Driver),
		URL:         appConfig.Server.Database.URL,
		Token:       appConfig.Server.Database.Token,
		Path:        filepath.Join(sqliteDir, serverDB),
		JournalMode: appConfig.Server.Database.SQLite.JournalMode,
		BusyTimeout: appConfig.Server.Database.SQLite.BusyTimeout,
	})
}

func attachAuditMirror(logger *logging.AppLogger, manager *database.SchemaManager) {
	logger.SetAuditMirror(func(entry logging.AuditEntry) error {
		return manager.InsertAuditMirror(context.Background(), entry)
	})
	logger.ReportAuditChainIssue()
}

func newCLIAuditLogger(appConfig *config.AppConfig, configDir, dataDir string) (*logging.AppLogger, func(), error) {
	logger, err := logging.NewAppLogger(appConfig)
	if err != nil {
		return nil, nil, err
	}
	manager, err := openAuditMirrorDatabase(appConfig, configDir, dataDir)
	if err != nil {
		logger.Close()
		return nil, nil, err
	}
	if err := manager.EnsureSchema(); err != nil {
		_ = manager.Close()
		logger.Close()
		return nil, nil, err
	}
	attachAuditMirror(logger, manager)
	return logger, func() {
		logger.Close()
		_ = manager.Close()
	}, nil
}

func waitForOverlayAddresses(torService *tor.TorService, i2pService *i2p.Service, torDone, i2pDone <-chan struct{}) (string, string) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	torFinished := func() bool {
		select {
		case <-torDone:
			return true
		default:
			return false
		}
	}
	i2pFinished := func() bool {
		select {
		case <-i2pDone:
			return true
		default:
			return false
		}
	}
	for {
		onionAddress, i2pAddress := torService.GetOnionAddress(), i2pService.Address()
		if torFinished() && !torService.IsEnabled() {
			onionAddress = ""
		}
		if i2pFinished() && !i2pService.Enabled() {
			i2pAddress = ""
		}
		if (onionAddress != "" || torFinished()) && (i2pAddress != "" || i2pFinished()) {
			return onionAddress, i2pAddress
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			return torService.GetOnionAddress(), i2pService.Address()
		}
	}
}

// parseUpdateArgs parses the arguments that follow "--update" (or its alias
// "--maintenance update") per AI.md PART 22: [check|yes|branch {stable|beta|devel}].
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

// applyPortOverride sets cfg.Server.Port from an explicit port value
// (--port flag or PORT env var) and persists the change to
// server.yml immediately. Per AI.md PART 5 Port Rules: "Once a port is
// selected (randomly or specified), it is saved to server.yml and persists
// across restarts" - so an explicit override must be written to disk right
// away, not only the randomly-generated first-run port. Returns true if the
// config file was rewritten.
func applyPortOverride(cfg *config.AppConfig, port, configPath string) (bool, error) {
	priorPort := cfg.Server.Port
	if port != "" {
		cfg.Server.Port = port
	}
	if cfg.Server.Port == priorPort {
		return false, nil
	}
	if err := config.SaveAppConfig(cfg, configPath); err != nil {
		return false, err
	}
	return true, nil
}

var loadedServerConfig *config.AppConfig

type ExitError struct {
	Code int
}

func (err ExitError) Error() string {
	return fmt.Sprintf("application command exited with status %d", err.Code)
}

type exitRequest struct {
	code int
}

func requestExit(code int) {
	panic(exitRequest{code: code})
}

func Run(ctx context.Context, serverConfig config.ServerConfig) (runErr error) {
	if serverConfig.Runtime.TestMode {
		serverConfig.Server.Tor.UseNetwork = false
		serverConfig.Server.Tor.Expose = false
		serverConfig.Server.I2P.Enabled = false
		serverConfig.Server.GeoIP.Enabled = false
		serverConfig.Server.SSL.Enabled = false
		serverConfig.Server.Notifications.Email.Enabled = false
	}
	loadedServerConfig = &serverConfig
	defer func() {
		loadedServerConfig = nil
		if recovered := recover(); recovered != nil {
			request, ok := recovered.(exitRequest)
			if !ok {
				panic(recovered)
			}
			if request.code != 0 {
				runErr = ExitError{Code: request.code}
			}
		}
	}()
	if serverConfig.Runtime.Help {
		printHelp()
		return nil
	}
	if serverConfig.Runtime.Version {
		printVersion()
		return nil
	}
	if system.IsWindowsService() {
		return system.RunAsWindowsService(run)
	}
	return run(ctx)
}

func run(ctx context.Context) error {
	signalpkg.ResetShutdown()
	startTime := time.Now()
	if loadedServerConfig == nil {
		return errors.New("app.Run requires a configuration returned by config.Load")
	}
	runtimeConfig := loadedServerConfig.Runtime
	sub := runtimeConfig.Command
	configDir := runtimeConfig.ConfigDir
	dataDir := runtimeConfig.DataDir
	cacheDir := runtimeConfig.CacheDir
	logDir := runtimeConfig.LogDir
	backupDir := runtimeConfig.BackupDir
	pidFile := runtimeConfig.PIDFile
	address := runtimeConfig.Address
	port := runtimeConfig.Port
	baseURL := runtimeConfig.BaseURL
	modeStr := runtimeConfig.Mode
	colorFlag := runtimeConfig.Color
	langFlag := runtimeConfig.Language
	serviceLanguage := runtimeConfig.Language
	debug := runtimeConfig.Debug
	daemon := runtimeConfig.Daemon

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
	// Port chain per AI.md: --port > PORT > config > random
	if port == "" && os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	if address == "" && os.Getenv("LISTEN") != "" {
		address = os.Getenv("LISTEN")
	}
	peekConfigDir := configDir
	if peekConfigDir == "" {
		peekConfigDir = pathpkg.GetDefaultConfigDir(pathpkg.IsPrivilegedMode())
	}
	peekAddress, peekPort := "", ""
	peekDaemonize := runtimeConfig.PeekDaemonize
	if loadedServerConfig.Runtime.ConfigPath == "" {
		peekAddress, peekPort, peekDaemonize = config.PeekServerStartupConfig(filepath.Join(peekConfigDir, "server.yml"))
	}
	if address == "" {
		address = peekAddress
	}
	if address == "" {
		address = "[::]"
	}
	if port == "" {
		port = peekPort
	}

	// Per AI.md PART 8: --baseurl PATH (URL path prefix, default "/").
	// Env var fallback: BASEURL.
	if baseURL == "" && os.Getenv("BASEURL") != "" {
		baseURL = os.Getenv("BASEURL")
	}

	// Per AI.md PART 8: --lang CODE (output language, default "auto").
	// Env var fallback: LANG (POSIX standard, e.g. "en_US.UTF-8").
	if langFlag == "" && os.Getenv("LC_ALL") != "" {
		langFlag = os.Getenv("LC_ALL")
	} else if langFlag == "" && os.Getenv("LANG") != "" {
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
			requestExit(1)
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
			requestExit(checkStatus())

		case "--shell":
			// Per AI.md PART 8: --shell completions [SHELL] | --shell init [SHELL].
			if len(rest) == 0 {
				fmt.Fprintln(os.Stderr, "Usage: --shell [completions|init] [SHELL]")
				requestExit(1)
			}
			shell := ""
			if len(rest) > 1 && !strings.HasPrefix(rest[1], "-") {
				shell = rest[1]
			}
			handleShellCommand(rest[0], shell)
			requestExit(0)

		case "tor":
			// Per AI.md PART 31: tor {status|validate|restart|regenerate|vanity|import-keys}.
			requestExit(handleTorCommand(rest, configDir, dataDir))

		case "--service":
			if len(rest) == 0 {
				fmt.Fprintln(os.Stderr, serviceText(cliOutputLocale(serviceLanguage, configDir, dataDir), "cli.service.dispatch_usage"))
				requestExit(1)
			}
			handleServiceCommand(rest[0], configDir, dataDir, serviceLanguage)
			return nil

		case "--update":
			// AI.md PART 22: --update [check|yes|branch {stable|beta|devel}].
			uCmd, uArg, _ := parseUpdateArgs(rest)
			handleUpdateCommand(uCmd, uArg)
			return nil

		case "--maintenance":
			// A bare --maintenance with no sub-command falls through to a normal
			// server start (matches the prior parser).
			if len(rest) > 0 {
				// Per AI.md PART 22: "--maintenance update [cmd]" is an alias for
				// "--update [cmd]".
				if rest[0] == "update" {
					uCmd, uArg, _ := parseUpdateArgs(rest[1:])
					handleUpdateCommand(uCmd, uArg)
					return nil
				}
				// Per AI.md PART 21: no --password flag; the password is prompted
				// interactively, so only a non-flag operand is taken here.
				maintArg := ""
				if (rest[0] == "data" || rest[0] == "security" || (rest[0] == "backup" && len(rest) > 1 && rest[1] == "test")) && len(rest) > 1 {
					maintArg = strings.Join(rest[1:], " ")
				} else if len(rest) > 1 && !strings.HasPrefix(rest[1], "--") {
					maintArg = rest[1]
				}
				handleMaintenanceCommand(rest[0], maintArg, configDir, dataDir)
				return nil
			}
		}
	}

	// Initialize mode and debug per AI.md PART 6
	// This must happen before starting the server
	mode.InitializeAppModeWithDebugOverride(modeStr, debug, runtimeConfig.DebugSet)

	// Manual daemonization happens before privileged sockets are bound. Service
	// commands return above, and container runtimes must supervise the process.
	testMode := loadedServerConfig != nil && loadedServerConfig.Runtime.TestMode
	if !testMode && !system.IsRunningInContainer() && system.ShouldDaemonize(false, daemon, peekDaemonize) {
		if err := daemonpkg.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to daemonize: %v\n", err)
			requestExit(1)
		}
		// If we get here, we're either the child or daemonization failed
	}

	// Load configuration
	appConfig := loadedServerConfig
	configPath := ""
	if appConfig == nil {
		var err error
		appConfig, configPath, err = config.LoadAppConfig(configDir, dataDir)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
	} else {
		configPath = appConfig.Runtime.ConfigPath
	}
	if runtimeConfig.I2PSet {
		appConfig.Server.I2P.Enabled = runtimeConfig.I2P
	}

	// Get paths early so we can override log directory
	paths := config.GetAppPaths(configDir, dataDir)

	// Ensure system user/group and set directory ownership per AI.md PART 23
	// "Binary handles EVERYTHING else: directories, permissions, user/group, Tor, etc."
	appName := system.ServiceUserName()

	// Create user and chown directories (only if running as root)
	// Include db subdirectory for SQLite database and the Tor data/site
	// directories per AI.md PART 23 step 8b — Tor dirs must exist and be
	// owned by the service user before privilege drop, not created later
	// by the Tor goroutine (which raced the drop and caused permission
	// errors on the hidden-service key file).
	dbDir := config.GetDatabaseDir(paths.Data)
	torDataDir := filepath.Join(paths.Data, "tor")
	torSiteDir := filepath.Join(torDataDir, "site")
	dirsToOwn := []string{
		paths.Config,
		paths.Data,
		dbDir,
		paths.Cache,
		paths.Log,
		paths.Backup,
		pathpkg.GetMaintenanceLockDir(paths.Data),
		torDataDir,
		torSiteDir,
	}
	if runtime.GOOS == "linux" && paths.PIDFile == pathpkg.GetDefaultPIDFile(true) {
		dirsToOwn = append(dirsToOwn, filepath.Dir(paths.PIDFile))
	}
	if appConfig.Server.I2P.Enabled {
		dirsToOwn = append(dirsToOwn,
			filepath.Join(paths.Config, "i2p"),
			filepath.Join(paths.Data, "i2p"),
			filepath.Join(paths.Data, "i2p", "site"),
		)
	}
	if !testMode {
		uid, gid, err := system.EnsureSystemUser(appName, dirsToOwn)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to ensure system user: %v\n", err)
			requestExit(1)
		} else if system.IsRunningAsRoot() && uid > 0 {
			fmt.Printf(terminal.UserIcon()+" Running as user %s (uid=%d, gid=%d)\n", appName, uid, gid)
		}
	}

	// Override log directory if specified
	if logDir != "" {
		paths.Log = logDir
	}

	// Write PID file per AI.md PART 8. The default path comes from paths.PIDFile
	// (already resolved from --pid > PID_FILE > OS default by GetAppPaths), so a
	// bare `vidveil server` still gets the documented PID file rather than none.
	// A --pid/PID_FILE value is an explicit operator request and always wins;
	// otherwise the server.pidfile config key decides (default true).
	// Uses signal package which handles stale PID detection per AI.md PART 8
	// - Checks if PID file exists and process is running
	// - Verifies process is actually our binary (not PID reuse)
	// - Removes stale PID files automatically
	// Containers never get a PID file per AI.md PART 8: the runtime supervises the
	// process and PIDs are namespace-local, so a shared-volume PID file is misleading
	pidFileExplicit := runtimeConfig.PIDFileSet || os.Getenv("PID_FILE") != ""
	if pidFile == "" {
		pidFile = paths.PIDFile
	}
	if !testMode && (pidFileExplicit || appConfig.Server.PIDFile) && pidFile != "" && runtime.GOOS != "windows" && !system.IsRunningInContainer() {
		if err := signalpkg.WritePIDFile(pidFile, appName); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			requestExit(1)
		}
		defer func() { _ = signalpkg.RemovePIDFile(pidFile) }()
	}

	// Override with command line flags
	if address != "" {
		appConfig.Server.Address = address
	}
	if _, err := applyPortOverride(appConfig, port, configPath); err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Failed to persist port to config: %v\n", err)
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
	// The project definition excludes regular user accounts, organizations, and custom domains.
	serverDBPath := filepath.Join(paths.Data, "db", "server.db")
	migrationMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to initialize database: %v\n", err)
		requestExit(1)
	}
	defer func() { _ = migrationMgr.Close() }()

	// Register and run migrations
	migrationMgr.RegisterDefaultMigrations()
	if err := migrationMgr.RunMigrations(); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to run migrations: %v\n", err)
		requestExit(1)
	}
	if err := config.ImportLegacyAppSecrets(migrationMgr.GetDB(), configPath, appConfig); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to import legacy application secrets: %v\n", err)
		requestExit(1)
	}

	// Initialize app secrets per AI.md PART 11
	// Generates installation_secret, cookie_signing_key, csrf_token_secret on first run
	secretsMgr := secret.NewManager(migrationMgr.GetDB())
	if err := secretsMgr.EnsureSecrets(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to initialize secrets: %v\n", err)
		requestExit(1)
	}
	legacySecret, err := secretsMgr.GetInstallationSecret(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to load installation secret for security-report migration: %v\n", err)
		requestExit(1)
	}
	if err := secreport.MigrateLegacySchema(context.Background(), migrationMgr.GetDB(), paths.Config, appConfig.Server.Security.EncryptionKey, legacySecret); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to migrate legacy security reports: %v\n", err)
		requestExit(1)
	}
	if err := pgp.RecoverStagedRotation(paths.Config, migrationMgr.GetDB()); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to recover PGP key rotation: %v\n", err)
		requestExit(1)
	}
	keyRecoveryCtx, cancelKeyRecovery := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelKeyRecovery()
	if err := secret.ImportLegacyPreviousEncryptionKey(keyRecoveryCtx, migrationMgr.GetDB(), appConfig.Server.Security.EncryptionKey); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to migrate legacy previous encryption key: %v\n", err)
		requestExit(1)
	}
	pendingKey, pending, err := secret.LoadPendingEncryptionKey(keyRecoveryCtx, migrationMgr.GetDB(), appConfig.Server.Security.EncryptionKey, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to recover staged encryption key: %v\n", err)
		requestExit(1)
	}
	if pending && pendingKey != appConfig.Server.Security.EncryptionKey {
		appConfig.Server.Security.EncryptionKey = pendingKey
		if err := config.SaveAppConfig(appConfig, configPath); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to install staged encryption key: %v\n", err)
			requestExit(1)
		}
	}
	previousKey, hasPreviousKey, err := secret.LoadPreviousEncryptionKey(keyRecoveryCtx, migrationMgr.GetDB(), appConfig.Server.Security.EncryptionKey, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to load previous encryption key: %v\n", err)
		requestExit(1)
	}
	if !hasPreviousKey {
		previousKey = ""
	}
	if err := secret.EnsureCurrentEncryptionKeyMarker(keyRecoveryCtx, migrationMgr.GetDB(), appConfig.Server.Security.EncryptionKey); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to verify current encryption key: %v\n", err)
		requestExit(1)
	}
	if err := completeEncryptionKeyRotation(keyRecoveryCtx, migrationMgr.GetDB(), appConfig.Server.Security.EncryptionKey, previousKey); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to finish encryption-key rotation: %v\n", err)
		requestExit(1)
	}
	if pending {
		if err := emitSecretRotationAudit(paths.Config, paths.Data, "security.encryption_key_rotated"); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to audit recovered encryption-key rotation: %v\n", err)
			requestExit(1)
		}
		cleanupTx, err := migrationMgr.GetDB().BeginTx(keyRecoveryCtx, nil)
		if err == nil {
			err = secret.DeletePendingEncryptionKey(keyRecoveryCtx, cleanupTx)
		}
		if err == nil {
			err = cleanupTx.Commit()
		} else if cleanupTx != nil {
			_ = cleanupTx.Rollback()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to clear encryption-key recovery marker: %v\n", err)
			requestExit(1)
		}
	}
	tokenHashCtx, cancelTokenHashMigration := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelTokenHashMigration()
	if err := apitoken.NormalizeLegacyHashes(tokenHashCtx, migrationMgr.GetDB(), appConfig.Server.Security.EncryptionKey); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to migrate stored API-token digests: %v\n", err)
		requestExit(1)
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
	sslSvc.SetDatabase(migrationMgr.GetDB())
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
	logger.SetAuditMirror(func(entry logging.AuditEntry) error {
		return migrationMgr.InsertAuditMirror(context.Background(), entry)
	})
	logger.ReportAuditChainIssue()
	defer logger.Close()
	for _, key := range appConfig.GeneratedSecretKeys {
		logger.Audit("config.secret_generated", "system", "system", "", "success", map[string]interface{}{"key_name": key})
	}
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
	// Pass log dir so Tor writes its notice log to {log_dir}/tor.log
	torSvc.SetLogDir(paths.Log)
	i2pSvc := i2p.NewService(appConfig.Server.I2P, paths.Config, paths.Data, paths.Log, logger)
	i2pCtx, cancelI2P := context.WithCancel(context.Background())
	defer cancelI2P()
	defer func() {
		if err := i2pSvc.Close(); err != nil {
			logger.Warn("I2P shutdown failed", map[string]interface{}{"error": err.Error()})
		}
	}()

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
	healthRegistry := health.NewRegistry()
	geoipSvc.RegisterHealthChecks(healthRegistry)
	blocklistSvc.RegisterHealthChecks(healthRegistry)
	cveSvc.RegisterHealthChecks(healthRegistry)
	healthRegistry.Refresh()

	// Initialize scheduler with database persistence per AI.md PART 18
	// Task state (run_count, fail_count, last_run) survives restarts
	sched := scheduler.NewSchedulerWithDB(migrationMgr.GetDB())
	sched.SetLogger(logger)
	var i2pHealthTask scheduler.TaskFunc
	if appConfig.Server.I2P.Enabled {
		i2pHealthTask = func(ctx context.Context) error {
			if err := i2pSvc.CheckHealth(ctx); err == nil {
				return nil
			}
			if err := i2pSvc.Restart(ctx); err != nil {
				return fmt.Errorf("I2P provider health check and restart: %w", err)
			}
			appConfig.Server.I2P.Address = i2pSvc.Address()
			return nil
		}
	}

	// Register all built-in tasks per AI.md PART 18
	if err := sched.RegisterBuiltinTasks(scheduler.BuiltinTaskFuncs{
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
			err := geoipSvc.Update()
			healthRegistry.Refresh()
			return err
		},
		BlocklistUpdate: func(ctx context.Context) error {
			// IP/domain blocklist update per PART 11
			err := blocklistSvc.Update(ctx)
			healthRegistry.Refresh()
			return err
		},
		CVEUpdate: func(ctx context.Context) error {
			// CVE/security database update per PART 11
			err := cveSvc.Update(ctx)
			healthRegistry.Refresh()
			return err
		},
		DatasetHealthRefresh: func(context.Context) error {
			healthRegistry.Refresh()
			return nil
		},
		TokenCleanup: func(ctx context.Context) error {
			cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			result, err := migrationMgr.GetDB().ExecContext(cleanupCtx,
				`DELETE FROM api_tokens WHERE expires_at IS NOT NULL AND expires_at <= ?`, time.Now().Unix())
			if err != nil {
				return fmt.Errorf("remove expired API tokens: %w", err)
			}
			removed, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("count removed API tokens: %w", err)
			}
			if removed > 0 {
				logger.Info("expired API tokens removed", map[string]interface{}{"count": removed})
			}
			captchaCleanup, err := migrationMgr.GetDB().ExecContext(cleanupCtx,
				`DELETE FROM captcha_challenges WHERE expires_at <= ?`, time.Now())
			if err != nil {
				return fmt.Errorf("remove expired CAPTCHA challenges: %w", err)
			}
			removedChallenges, err := captchaCleanup.RowsAffected()
			if err != nil {
				return fmt.Errorf("count removed CAPTCHA challenges: %w", err)
			}
			if removedChallenges > 0 {
				logger.Info("expired CAPTCHA challenges removed", map[string]interface{}{"count": removedChallenges})
			}
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
			if appConfig.Server.Backup.PasswordUnreadable {
				return errors.New("backup.password_unreadable")
			}
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			maint.SetLogger(logger)
			retention := appConfig.Server.Backup.Retention
			return maint.BackupDailyFull(maintenance.BackupOptions{
				Password:       appConfig.Server.Backup.EncryptionPassword,
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
			if appConfig.Server.Backup.PasswordUnreadable {
				return errors.New("backup.password_unreadable")
			}
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			maint.SetLogger(logger)
			return maint.BackupIncrementalWithPassword("", appConfig.Server.Backup.EncryptionPassword)
		},
		HealthcheckSelf: func(ctx context.Context) error {
			// Self health check per PART 13
			if err := migrationMgr.GetDB().PingContext(ctx); err != nil {
				return fmt.Errorf("database health check failed: %w", err)
			}
			if !sched.IsRunning() {
				return fmt.Errorf("scheduler health check failed: scheduler is stopped")
			}
			return nil
		},
		TorHealth: func(ctx context.Context) error {
			// Tor health check per PART 31 - only if hidden service enabled
			// Per PART 31: Tor supports hidden service and optional outbound network routing
			if torSvc == nil {
				// Refresh the PART 20 Tor gauges even when Tor is absent so a
				// scrape reports a definite 0 instead of a stale value.
				svcmetrics.SetTorState(false, false, false)
				return nil
			}
			running := torSvc.IsRunning()
			// A connected status means Tor finished bootstrap and published the
			// hidden service descriptor, which requires a live circuit.
			circuit := torSvc.GetStatus() == tor.TorServiceStatusConnected
			svcmetrics.SetTorState(torSvc.IsEnabled(), running, circuit)
			// Check if Tor service is running
			if !running {
				return fmt.Errorf("tor service is not running")
			}
			return nil
		},
		I2PHealth: i2pHealthTask,
		UpdateCheck: func(ctx context.Context) error {
			// Update check per AI.md PART 18/22 — daily at 06:00
			// Notify-only unless update.auto_install is true; honors update.defer_days
			maint := maintenance.NewMaintenanceManager(paths.Config, paths.Data, version.GetVersion())
			maint.SetBuildEpoch(buildEpoch())
			info, err := maint.CheckUpdateContext(ctx)
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
				return maint.ApplyUpdateContext(ctx, info.DownloadURL)
			}
			return nil
		},
		KeyserverRetry: func(ctx context.Context) error {
			_, err := pgpPublishCoreWithPublisherContext(ctx, paths.Config, paths.Data, pgp.PublishToKeyserversWithState)
			return err
		},
	}); err != nil {
		logger.Error("failed to register built-in scheduler tasks", map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("register built-in scheduler tasks: %w", err)
	}
	// Set Tor provider for engine manager per PART 31
	// This enables Tor outbound network for anonymized engine queries when UseNetwork is true
	engineMgr.SetTorProvider(torSvc)
	defer func() { _ = torSvc.Stop() }()

	// Load scheduler history from database per AI.md PART 18
	if err := sched.LoadHistoryFromDB(100); err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Failed to load scheduler history: %v\n", err)
	}

	// Create server with migration manager, scheduler, and logger per AI.md PART 11
	srv := server.NewServer(appConfig, configDir, dataDir, engineMgr, migrationMgr, sched, logger)
	applySchedulerConfig(sched, appConfig, logger)
	serveTorProxy := func(listener net.Listener) {
		if listener == nil {
			return
		}
		go func() {
			if err := srv.ServeOn(listener); err != nil && err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
				logger.Warn("Tor proxy listener failed", map[string]interface{}{"error": err.Error()})
			}
		}()
	}

	// Set Tor service for handlers per AI.md PART 31
	srv.SetTorService(torSvc)
	srv.SetI2PService(i2pSvc)
	srv.SetHealthRegistry(healthRegistry)

	// Wire the internal loopback-only Tor control channel used by the
	// `{project_name} tor ...` CLI subcommand per AI.md PART 31
	srv.SetTorControl(torSvc)

	// Set GeoIP service for content restriction checks and country blocking per AI.md PART 19
	srv.SetGeoIPService(geoipSvc)

	// Set blocklist service for IP/domain blocklist middleware per AI.md PART 11
	srv.SetBlocklistService(blocklistSvc)

	// Wire SSL manager for HTTPS serving and ACME challenges per AI.md PART 15
	srv.SetSSLManager(sslSvc)

	// Start the scheduler only after server handlers and failure notifications
	// are wired, so startup catch-up runs cannot miss their configured channels.
	sched.Start(context.Background())
	defer sched.Stop()

	// Register the process-wide active config for the Output Sanitization
	// Pipeline (AI.md PART 11 stage 5) — packages outside the request path
	// use config.IsProductionMode() instead of threading a config pointer
	// through every handler. The watcher below mutates this same pointer
	// in place on hot-reload, so this call is only needed once.
	config.SetCurrent(appConfig)

	// Start live config watcher per AI.md PART 8 NON-NEGOTIABLE
	configWatcher := config.NewWatcher(configPath, appConfig)
	var torConfigMu sync.Mutex
	var torRestartMu sync.Mutex
	lastTorConfig := appConfig.Server.Tor
	lastOperatorToken := appConfig.Server.Token
	torStartupComplete := make(chan struct{})
	torStartAttempted := false
	configWatcher.OnReload(func(newCfg *config.AppConfig) {
		applySchedulerConfig(sched, newCfg, logger)
		torConfigMu.Lock()
		operatorTokenChanged := lastOperatorToken != newCfg.Server.Token
		lastOperatorToken = newCfg.Server.Token
		torChanged := !reflect.DeepEqual(lastTorConfig, newCfg.Server.Tor)
		lastTorConfig = newCfg.Server.Tor
		torConfigMu.Unlock()
		if torChanged {
			go func() {
				<-torStartupComplete
				if !torStartAttempted {
					return
				}
				torRestartMu.Lock()
				defer torRestartMu.Unlock()
				if err := torSvc.Restart(context.Background()); err != nil {
					logger.Warn("Tor restart after configuration reload failed", map[string]interface{}{"error": err.Error()})
					return
				}
				svcmetrics.SetTorState(torSvc.IsEnabled(), torSvc.IsRunning(),
					torSvc.GetStatus() == tor.TorServiceStatusConnected)
				serveTorProxy(torSvc.GetProxyListener())
			}()
		}
		if operatorTokenChanged {
			if err := srv.RotateCSRFSecretAfterOperatorTokenChange(context.Background()); err != nil {
				logger.Warn("CSRF secret rotation after operator token regeneration failed", map[string]interface{}{"error": err.Error()})
			}
		}
		// Config has been reloaded - the shared appConfig pointer is already updated
		// Refresh token-auth state so server.token changes take effect immediately.
		srv.RefreshOperatorTokenHash()
		srv.RefreshEncryptionKey(newCfg.Server.Security.EncryptionKey)
		go func() {
			if err := srv.RefreshBrandingImages(context.Background()); err != nil {
				logger.Warn("branding image refresh after config reload failed", nil)
			}
		}()
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
	if loadedServerConfig != nil && loadedServerConfig.Runtime.Listener != nil {
		httpListener = loadedServerConfig.Runtime.Listener
		httpPort = strconv.Itoa(httpListener.Addr().(*net.TCPAddr).Port)
		httpsPort = ""
	} else if httpPort != "" {
		addr := net.JoinHostPort(strings.Trim(appConfig.Server.Address, "[]"), httpPort)
		httpListener, err = srv.Listen(addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to bind %s: %v\n", addr, err)
			requestExit(1)
		}
	}
	if httpsPort != "" {
		addr := net.JoinHostPort(strings.Trim(appConfig.Server.Address, "[]"), httpsPort)
		httpsListener, err = srv.Listen(addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to bind %s: %v\n", addr, err)
			requestExit(1)
		}
	}

	// Primary listener drives the startup banner: HTTPS when present, else HTTP.
	primaryPort := httpsPort
	primaryIsHTTPS := true
	if primaryPort == "" {
		primaryPort = httpPort
		primaryIsHTTPS = false
	}
	listenAddr := net.JoinHostPort(strings.Trim(appConfig.Server.Address, "[]"), primaryPort)
	// Re-apply ownership immediately before dropping privileges per AI.md PART 23
	// step 8c — catches files created since the initial EnsureSystemUser call
	// above (step 15's database file, downloaded GeoIP databases), which the
	// earlier one-time chown could not have covered since they did not exist yet.
	if !testMode && system.IsRunningAsRoot() {
		if _, _, err := system.EnsureSystemUser(appName, dirsToOwn); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to re-apply ownership before privilege drop: %v\n", err)
			requestExit(1)
		}
	}

	// Drop privileges to the vidveil system user after port is bound per AI.md PART 23.
	// ShouldDropPrivileges() returns true only on Unix when current uid == 0.
	if !testMode && system.ShouldDropPrivileges() {
		if err := system.DropPrivileges(appName, paths.Config); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to drop privileges: %v\n", err)
			requestExit(1)
		}
		fmt.Printf(terminal.UserIcon()+" Dropped privileges to %s\n", appName)
	}

	i2pStartupComplete := make(chan struct{})
	if appConfig.Server.I2P.Enabled {
		go func() {
			defer close(i2pStartupComplete)
			if err := i2pSvc.Start(i2pCtx); err != nil {
				if i2pCtx.Err() == nil {
					logger.Warn("I2P eepsite unavailable", map[string]interface{}{"error": err.Error()})
				}
				return
			}
			appConfig.Server.I2P.Address = i2pSvc.Address()
			logger.Info("I2P eepsite started", map[string]interface{}{"address": i2pSvc.Address(), "provider": i2pSvc.Provider()})
			if listener := i2pSvc.Listener(); listener != nil {
				if err := srv.ServeOn(listener); err != nil && err != http.ErrServerClosed {
					logger.Warn("I2P backend listener stopped", map[string]interface{}{"error": err.Error()})
				}
			}
		}()
	} else {
		close(i2pStartupComplete)
	}

	// Start Tor hidden service per PART 31 (in background to not block HTTP server).
	// Auto-enabled if tor binary is installed - no enable flag needed.
	// Per PART 31: torrc declares HiddenServiceDir/HiddenServicePort 80 pointing at a
	// dedicated loopback PROXY-protocol listener (torSvc.GetProxyListener()), not the
	// public HTTP listener directly — HiddenServiceExportCircuitID haproxy prepends a
	// PROXY protocol v1 header to each backend connection, which that listener parses.
	// Started only after privilege drop above so its directory/key ownership
	// (self-chowned to os.Getuid()/os.Getgid() inside torSvc.Start) always
	// targets the final, stable process uid — starting it earlier raced the
	// drop and intermittently left root-owned files the dropped process
	// could not write to.
	if !testMode {
		go func() {
			defer close(torStartupComplete)
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
			torStartAttempted = true
			if err := torSvc.Start(torCtx, serverPort); err != nil {
				// PART 31: Tor errors are WARN level, server continues without Tor
				fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Tor hidden service: %v\n", err)
				// Publish the PART 20 gauges immediately rather than waiting for the
				// first tor_health run ten minutes later.
				svcmetrics.SetTorState(false, false, false)
			} else {
				svcmetrics.SetTorState(torSvc.IsEnabled(), torSvc.IsRunning(),
					torSvc.GetStatus() == tor.TorServiceStatusConnected)
				// Wire resolved onion address back to config so PART 12 Tor request
				// detection (urlvars.isTorRequest) can match the Host header.
				if addr := torSvc.GetOnionAddress(); addr != "" {
					appConfig.Server.Tor.OnionAddress = addr
				}
				if torSvc.UseNetworkEnabled() && torSvc.OutboundEnabled() {
					fmt.Println("[INFO] Tor outbound network enabled - engine queries are anonymized")
				}
				// Serve the same handler chain on the dedicated loopback PROXY-protocol
				// listener that torrc's HiddenServicePort 80 forwards .onion traffic to.
				serveTorProxy(torSvc.GetProxyListener())
			}
		}()
	} else {
		close(torStartupComplete)
	}

	// Start server goroutine — serves on the pre-bound listener
	serverErrors := make(chan error, 1)
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
			} else {
				// Per PART 17: On failure disable email, log a warning, keep running, retry next startup
				appConfig.Server.Notifications.Email.Enabled = false
				logger.Warn("smtp connection test failed, email features disabled until next startup", map[string]interface{}{
					"host":  smtpHost,
					"port":  smtpPort,
					"error": err.Error(),
				})
			}
		} else {
			// Per PART 17: Auto-detect on first run if no host configured
			detectedHost, detectedPort := email.AutodetectSMTP(nil, nil)
			if detectedHost != "" && detectedPort > 0 {
				smtpInfo = fmt.Sprintf("%s:%d (auto)", detectedHost, detectedPort)
				appConfig.Server.Notifications.Email.SMTP.Host = detectedHost
				appConfig.Server.Notifications.Email.SMTP.Port = detectedPort
				appConfig.Server.Notifications.Email.Enabled = true
				// Per PART 17: Persist the detected host:port to server.yml
				if err := config.SaveAppConfig(appConfig, configPath); err != nil {
					logger.Warn("could not save detected smtp settings to server.yml", map[string]interface{}{
						"host":  detectedHost,
						"port":  detectedPort,
						"error": err.Error(),
					})
				}
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
		var forceColor *bool
		if colorFlag == "yes" || colorFlag == "no" {
			value := colorFlag == "yes"
			forceColor = &value
		}
		onionAddress, i2pAddress := waitForOverlayAddresses(torSvc, i2pSvc, torStartupComplete, i2pStartupComplete)
		setupToken := ""
		if appConfig.GeneratedServerToken && os.Getenv("VIDVEIL_SERVER_TOKEN") == "" && os.Getenv("VIDVEIL_TOKEN") == "" {
			setupToken = appConfig.Server.Token
		}
		serverbanner.PrintServerStartupBanner(serverbanner.ServerBannerInfo{
			AppName:         "VidVeil",
			Version:         version.GetVersion(),
			AppMode:         appConfig.Server.Mode,
			StartupDatetime: formatServerStartupTime(time.Now()),
			URLs:            []string{displayURL},
			OnionAddress:    onionAddress,
			I2PAddress:      i2pAddress,
			SMTPAddress:     smtpInfo,
			SetupToken:      setupToken,
			ForceColor:      forceColor,
			Lang:            langFlag,
		})

		if isFirstRun {
			fmt.Printf("[INFO] First run detected. Edit %s to configure.\n", configPath)
		}

		// Log INFO lines per AI.md PART 11
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
						serverErrors <- fmt.Errorf("HTTP redirect server: %w", err)
					}
				}()
			} else {
				// HTTP-only
				go func() {
					if err := srv.ServeOn(httpListener); err != nil && err != http.ErrServerClosed {
						serverErrors <- fmt.Errorf("HTTP server: %w", err)
					}
				}()
				if loadedServerConfig != nil && loadedServerConfig.Runtime.Ready != nil {
					close(loadedServerConfig.Runtime.Ready)
				}
			}
		}
		if httpsListener != nil {
			if err := srv.ServeTLSOn(httpsListener); err != nil && err != http.ErrServerClosed {
				serverErrors <- fmt.Errorf("HTTPS server: %w", err)
			}
		}
	}()

	// DB health monitor — auto-enters/exits maintenance mode per AI.md PART 5/6.
	// Maintenance mode triggers ONLY for DB connection failure or file-write failure.
	// Self-heals continuously (retry every 30s) — no human intervention required.
	maintMgr := maintenance.NewMaintenanceManager(configDir, dataDir, version.GetVersion())
	if !testMode {
		go func() {
			healInterval, parseErr := time.ParseDuration(appConfig.Server.Maintenance.SelfHealing.RetryInterval)
			if parseErr != nil || healInterval <= 0 {
				healInterval = maintenance.MaintenanceRetryInterval
			}
			inMaintenance := false
			var maintenanceSince time.Time
			var healingAttempts uint64
			if persisted, active := maintenance.ReadMaintenanceStatus(dataDir); active && persisted != nil && (persisted.Reason == "database_connection" || persisted.Reason == "file_write") {
				inMaintenance = true
				maintenanceSince = persisted.Since
			}
			for {
				if ctx.Err() != nil {
					return
				}
				// Probe database access with the PART 13 retry schedule and verify
				// that the data directory can durably create and remove a file.
				dbErr := pingDatabaseForRecovery(migrationMgr.GetDB())
				writeErr := probeDataDirectoryWrite(paths.Data)
				unhealthy := dbErr != nil || writeErr != nil

				if unhealthy && !inMaintenance {
					// Auto-enter maintenance mode
					if enterErr := maintMgr.SetMaintenanceMode(true); enterErr != nil {
						fmt.Fprintf(os.Stderr, "[WARN] Unable to persist maintenance mode: %v\n", enterErr)
					}
					inMaintenance = true
					maintenanceSince = time.Now().UTC()
					healingAttempts = 0
					srv.NotifyMaintenance(true, "database_connection or file_write", 0)
					if dbErr != nil {
						fmt.Fprintf(os.Stderr, "[WARN] DB unavailable (%v) — entering maintenance mode; retrying every %s\n", dbErr, healInterval)
					} else {
						fmt.Fprintf(os.Stderr, "[WARN] File-write failure (%v) — entering maintenance mode; retrying every %s\n", writeErr, healInterval)
					}
				}
				if unhealthy && inMaintenance {
					healingAttempts++
					lastAttempt := time.Now().UTC()
					nextAttempt := lastAttempt.Add(healInterval)
					if total, free, diskErr := maintenance.DiskSpace(paths.Data); diskErr == nil && total > 0 {
						usedPercent := int((total - free) * 100 / total)
						if usedPercent >= appConfig.Server.Maintenance.Cleanup.DiskThreshold {
							retention := appConfig.Server.Backup.Retention
							freed, cleanupErr := maintMgr.CleanupDiskPressure(retention, paths.Log, appConfig.Server.Maintenance.Cleanup.LogRetentionDays)
							if cleanupErr != nil {
								logger.Warn("maintenance disk cleanup failed", map[string]interface{}{"error": cleanupErr.Error(), "used_percent": usedPercent})
							} else {
								logger.Info("maintenance disk cleanup completed", map[string]interface{}{"bytes_freed": freed, "used_percent": usedPercent})
							}
						}
					}
					reason := "file_write"
					message := "The server cannot write to its data directory. It is read-only while recovery is attempted."
					if dbErr != nil {
						reason = "database_connection"
						message = "The server cannot connect to its database. It is read-only while recovery is attempted."
					}
					selfHealing := maintenance.SelfHealingAttemptStatus(healingAttempts, appConfig.Server.Maintenance.SelfHealing.MaxAttempts, lastAttempt, nextAttempt)
					status := maintenance.MaintenanceStatus{
						Reason: reason, Message: message, Diagnostic: maintenanceDiagnostic(dbErr, writeErr), Since: maintenanceSince,
						SelfHealing: selfHealing,
					}
					if statusErr := maintenance.WriteMaintenanceStatus(dataDir, status); statusErr != nil {
						fmt.Fprintf(os.Stderr, "[WARN] Unable to persist maintenance recovery status: %v\n", statusErr)
					}
					srv.SetMaintenanceStatus(&status)
					if selfHealing.Exhausted {
						logger.Audit("server.self_healing_exhausted", "system", "system", "", "failure", map[string]interface{}{"reason": reason, "attempts": healingAttempts})
						srv.NotifySelfHealingExhausted(reason, healingAttempts)
						return
					}
				} else if !unhealthy && inMaintenance {
					// Self-heal: condition cleared, exit maintenance mode
					if exitErr := maintMgr.SetMaintenanceMode(false); exitErr == nil {
						inMaintenance = false
						srv.SetMaintenanceStatus(nil)
						srv.NotifyMaintenance(false, "database and file checks recovered", time.Since(maintenanceSince))
						fmt.Println("[INFO] Health restored — exiting maintenance mode")
					}
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(healInterval):
				}
			}
		}()
	}

	// Configure signal handlers per AI.md PART 8
	// SIGUSR1 (10) → Reopen logs (log rotation)
	// SIGUSR2 (12) → Status dump
	signalpkg.SetLogReopenFunc(func() {
		logger.Reopen()
	})
	signalpkg.SetStatusDumpFunc(func() {
		// Dump status to stderr
		fmt.Fprintf(os.Stderr, "[STATUS] Server running on %s\n", net.JoinHostPort(strings.Trim(appConfig.Server.Address, "[]"), appConfig.Server.Port))
		fmt.Fprintf(os.Stderr, "[STATUS] Mode: %s, Debug: %v\n", appConfig.Server.Mode, mode.IsDebugEnabled())
		fmt.Fprintf(os.Stderr, "[STATUS] Uptime: %v\n", time.Since(startTime))
	})

	// Wait for shutdown signal per AI.md PART 8
	// Handles: SIGTERM(15), SIGINT(2), SIGQUIT(3), SIGRTMIN+3(37)
	// Ignores: SIGHUP(1) - config auto-reloads via file watcher
	signalContext, cancelSignalWait := context.WithCancel(ctx)
	defer cancelSignalWait()
	shutdownSignal := make(chan os.Signal, 1)
	go func() {
		shutdownSignal <- signalpkg.WaitForShutdown(signalContext)
	}()
	var sig os.Signal
	select {
	case sig = <-shutdownSignal:
	case serverErr := <-serverErrors:
		cancelI2P()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelShutdown()
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			return errors.Join(serverErr, fmt.Errorf("server shutdown: %w", shutdownErr))
		}
		return serverErr
	}
	fmt.Printf("\n%s Received %v, shutting down gracefully...\n", terminal.StopIcon(), sig)
	signalpkg.BeginShutdown()
	cancelI2P()

	// Graceful shutdown with timeout (30 seconds per AI.md PART 8)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	fmt.Printf("%s Server stopped\n", terminal.StatusIcon(true))
	return nil
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
--mode {production|development|debug}  - Application mode (default: production)
--config DIR                           - Config directory
--data DIR                             - Data directory
--cache DIR                            - Cache directory
--log DIR                              - Log directory
--backup DIR                           - Backup directory
--pid FILE                             - PID file path
--address ADDR                         - Listen address (default: [::], dual-stack; IPv4-only fallback if IPv6 is unavailable)
--port PORT                            - Listen port (default: random 64xxx, 80 in container)
--baseurl PATH                         - URL path prefix (default: /)
--daemon                               - Run as daemon (detach from terminal)
--debug                                - Enable debug mode
--i2p                                  - Enable the optional I2P eepsite
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
	fmt.Printf("%s %s\n", binaryName, version.GetVersion())
	fmt.Printf("Commit: %s\n", version.CommitID)
	fmt.Printf("Built: %s\n", version.BuildTime)
	fmt.Printf("Go: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if version.OfficialSite != "" {
		fmt.Printf("Site: %s\n", version.OfficialSite)
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
	// (Port chain per AI.md: --port > PORT > config > random).
	// The server never persists an env-resolved port back to server.yml, so
	// reading statusConfig.Server.Port alone would report a stale/wrong port
	// whenever PORT overrides the config file (e.g. the
	// standard Docker deployment sets PORT=80) — causing --status, and thus
	// the Docker HEALTHCHECK, to falsely report "Stopped" against a healthy
	// server listening on a different port.
	statusPort := statusConfig.Server.Port
	if envPort := os.Getenv("PORT"); envPort != "" {
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
	_ = conn.Close()

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
	i2p := health.Features.I2P
	switch {
	case i2p.Running:
		fmt.Printf("\nI2P Eepsite: Running (%s)\n", i2p.Provider)
		fmt.Printf("  Address: %s\n", i2p.Hostname)
	case !i2p.Enabled:
		fmt.Println("\nI2P Eepsite: Disabled")
	case i2p.Status == "error:no_provider" || i2p.Provider == "none":
		fmt.Println("\nI2P Eepsite: No Provider")
	default:
		fmt.Println("\nI2P Eepsite: Error")
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
		if err := printCompletions(shell, binaryName); err != nil {
			fmt.Fprintln(os.Stderr, err)
			requestExit(2)
		}
	case "init":
		printInit(shell, binaryName)
	case "--help", "help", "-h":
		// Per AI.md PART 8: --shell --help prints help and exits 0
		fmt.Printf(`Shell Integration Commands:
  %s --shell completions [SHELL]   Print shell completions script
  %s --shell init [SHELL]          Print shell init command for eval

Supported Shells:
  bash, zsh, fish, powershell, pwsh
  init also supports sh, dash, and ksh (no programmable completions)

Examples:
  # Add to ~/.bashrc or ~/.zshrc
  eval "$(%s --shell init)"

  # Or source completions directly
  source <(%s --shell completions bash)
`, binaryName, binaryName, binaryName, binaryName)
		requestExit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown --shell command: %s\nUsage: --shell [completions|init|--help] [SHELL]\n", subCmd)
		requestExit(1)
	}
}

// printCompletions prints shell completion script to stdout per PART 8
func printCompletions(shell, binaryName string) error {
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
		return fmt.Errorf("shell %s has no programmable completions", shell)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
	return nil
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
		fmt.Printf("# %s: %s has no programmable completion\n", binaryName, shell)
	case "powershell", "pwsh":
		fmt.Printf("Invoke-Expression (& %s --shell completions powershell)\n", binaryName)
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", shell)
		requestExit(1)
	}
}

func printBashCompletions(binaryName string) {
	fmt.Printf(`_%s_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local opts="--help --version --shell --config --data --cache --log --backup --pid --address --port --baseurl --mode --status --daemon --debug --i2p --color --lang --service --maintenance --update tor"
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
    '--i2p[Enable the optional I2P eepsite]' \
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
			'--status', '--daemon', '--debug', '--i2p', '--color', '--lang', '--service', '--maintenance', '--update', 'tor'
    )
    $completions | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`, binaryName)
}

func handleServiceCommand(cmd, configDir, dataDir, language string) {
	locale := cliOutputLocale(language, configDir, dataDir)
	// Per AI.md PART 23 and PART 24: Use system.NewServiceManager which creates system user
	// Get binary path
	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.executable_error", "{error}", err.Error()))
		requestExit(1)
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
		fmt.Println(serviceText(locale, "cli.service.starting"))
		if err := svc.Start(); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.start_error", "{error}", err.Error()))
			requestExit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + serviceText(locale, "cli.service.started"))

	case "stop":
		fmt.Println(serviceText(locale, "cli.service.stopping"))
		if err := svc.Stop(); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.stop_error", "{error}", err.Error()))
			requestExit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + serviceText(locale, "cli.service.stopped"))

	case "restart":
		fmt.Println(serviceText(locale, "cli.service.restarting"))
		if err := svc.Restart(); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.restart_error", "{error}", err.Error()))
			requestExit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + serviceText(locale, "cli.service.restarted"))

	case "reload":
		fmt.Println(serviceText(locale, "cli.service.reloading"))
		if err := svc.Reload(); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.reload_error", "{error}", err.Error()))
			requestExit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + serviceText(locale, "cli.service.reloaded"))

	case "status":
		// Per AI.md PART 24: Show service status
		status, err := svc.GetServiceStatus()
		if err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.status_error", "{error}", err.Error()))
			requestExit(1)
		}
		switch status {
		case "running":
			fmt.Println(terminal.StatusIcon(true) + serviceText(locale, "cli.service.status_running"))
		case "stopped":
			fmt.Println(terminal.StopButtonIcon() + serviceText(locale, "cli.service.status_stopped"))
		default:
			fmt.Println(terminal.QuestionIcon() + serviceText(locale, "cli.service.status_other", "{status}", status))
		}

	case "--install":
		// Per AI.md PART 23: Check escalation before service install
		if err := system.HandleEscalation(serviceText(locale, "cli.service.installing"), locale); err != nil {
			fmt.Fprintf(os.Stderr, "Using the current user's service scope: %v\n", err)
		}
		fmt.Println(serviceText(locale, "cli.service.installing"))
		if err := svc.Install(); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.install_error", "{error}", err.Error()))
			requestExit(1)
		}

	case "--uninstall":
		// Per AI.md PART 23: Confirmation required before destructive action
		fmt.Println(terminal.WarningIcon() + serviceText(locale, "cli.service.uninstall_warning"))
		fmt.Println("   • " + serviceText(locale, "cli.service.uninstall_stop"))
		fmt.Println("   • " + serviceText(locale, "cli.service.uninstall_remove_config"))
		fmt.Println("   • " + serviceText(locale, "cli.service.uninstall_delete_data"))
		fmt.Println("   • " + serviceText(locale, "cli.service.uninstall_remove_user"))
		fmt.Println()
		fmt.Print(serviceText(locale, "cli.service.uninstall_prompt"))

		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to read confirmation: %v\n", err)
			requestExit(1)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println(serviceText(locale, "cli.service.cancelled"))
			requestExit(0)
		}

		// Per AI.md PART 23: Check escalation before service uninstall
		if err := system.HandleEscalation(serviceText(locale, "cli.service.uninstalling"), locale); err != nil {
			fmt.Fprintf(os.Stderr, "Using the current user's service scope: %v\n", err)
		}

		fmt.Println(serviceText(locale, "cli.service.uninstalling"))
		if err := svc.UninstallConfirmed(); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.uninstall_error", "{error}", err.Error()))
			requestExit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + serviceText(locale, "cli.service.uninstalled"))

	case "--disable":
		// Per AI.md PART 8: Disable service from starting at boot
		fmt.Println(serviceText(locale, "cli.service.disabling"))
		if err := svc.Disable(); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.disable_error", "{error}", err.Error()))
			requestExit(1)
		}
		fmt.Println(terminal.StatusIcon(true) + serviceText(locale, "cli.service.disabled"))

	case "--help":
		// Per AI.md PART 8: Service command help
		fmt.Println(serviceText(locale, "cli.service.help_title"))
		for _, item := range []struct{ key, command string }{
			{"cli.service.help_start", "start"}, {"cli.service.help_stop", "stop"},
			{"cli.service.help_restart", "restart"}, {"cli.service.help_reload", "reload"},
			{"cli.service.help_status", "status"}, {"cli.service.help_install", "--install"},
			{"cli.service.help_uninstall", "--uninstall"}, {"cli.service.help_disable", "--disable"},
		} {
			fmt.Println(serviceText(locale, item.key, "{binary}", binaryName, "{command}", item.command))
		}
		fmt.Println()
		fmt.Println(serviceText(locale, "cli.service.help_managers"))
		for _, key := range []string{
			"cli.service.manager_systemd", "cli.service.manager_openrc", "cli.service.manager_sysvinit",
			"cli.service.manager_runit", "cli.service.manager_launchd", "cli.service.manager_windows",
			"cli.service.manager_bsd",
		} {
			fmt.Println("  - " + serviceText(locale, key))
		}

	default:
		fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+serviceText(locale, "cli.service.unknown_command", "{command}", cmd))
		fmt.Fprintln(os.Stderr, serviceText(locale, "cli.service.help_hint", "{binary}", binaryName))
		requestExit(1)
	}
}

func cliOutputLocale(language, configDir, dataDir string) string {
	if language == "" {
		language = serviceConfigLanguage(configDir, dataDir)
	}
	if language == "" {
		language = os.Getenv("LC_ALL")
		if language == "" {
			language = os.Getenv("LANG")
		}
	}
	language = strings.ToLower(strings.TrimSpace(language))
	if index := strings.IndexAny(language, "._@"); index >= 0 {
		language = language[:index]
	}
	if index := strings.Index(language, "-"); index >= 0 {
		language = language[:index]
	}
	if i18n.GlobalTranslator().HasLocale(language) {
		return language
	}
	return i18n.DefaultLocale
}

func serviceConfigLanguage(configDir, dataDir string) string {
	configPath := filepath.Join(config.GetAppPaths(configDir, dataDir).Config, "server.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var cfg struct {
		Lang   string `yaml:"lang"`
		Server struct {
			I18N struct {
				DefaultLanguage string `yaml:"default_language"`
			} `yaml:"i18n"`
		} `yaml:"server"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	if cfg.Lang != "" && cfg.Lang != "auto" {
		return cfg.Lang
	}
	if strings.EqualFold(cfg.Server.I18N.DefaultLanguage, "auto") {
		return ""
	}
	return cfg.Server.I18N.DefaultLanguage
}

func serviceText(locale, key string, replacements ...string) string {
	text := i18n.Translate(locale, key)
	if len(replacements) >= 2 {
		text = strings.NewReplacer(replacements...).Replace(text)
	}
	return text
}

// handleUpdateCommand implements AI.md PART 22 --update command
func handleUpdateCommand(cmd, arg string) {
	binaryName := filepath.Base(os.Args[0])
	maint := maintenance.NewMaintenanceManager("", "", version.GetVersion())
	maint.SetBuildEpoch(buildEpoch())

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
				requestExit(0)
			}
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Update check failed: %v\n", err)
			requestExit(1)
		}

		fmt.Printf("Latest version:  %s\n", info.LatestVersion)

		if info.UpdateAvailable {
			fmt.Println("\n" + terminal.PackageIcon() + " Update available!")
			fmt.Printf("   Release: %s\n", info.ReleaseURL)
			fmt.Println("\n   Run 'vidveil --update' to download and install")
		} else {
			fmt.Println(terminal.StatusIcon(true) + " Already up to date")
		}
		requestExit(0)

	case "yes", "":
		// Check and perform in-place update with restart
		fmt.Println("Checking for updates...")
		fmt.Printf("Current version: %s\n", version.GetVersion())

		info, err := maint.CheckUpdate()
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				fmt.Println(terminal.StatusIcon(true) + " Already up to date")
				requestExit(0)
			}
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Update check failed: %v\n", err)
			requestExit(1)
		}

		fmt.Printf("Latest version:  %s\n", info.LatestVersion)

		if info.UpdateAvailable {
			fmt.Println("\n" + terminal.PackageIcon() + " Update available!")
			fmt.Printf("   Release: %s\n", info.ReleaseURL)

			if info.DownloadURL != "" {
				fmt.Println("\nApplying update...")
				if err := maint.ApplyUpdate(info.DownloadURL); err != nil {
					fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Update failed: %v\n", err)
					requestExit(1)
				}
				fmt.Println(terminal.StatusIcon(true) + " Update successful! Please restart the application.")
			}
		} else {
			fmt.Println(terminal.StatusIcon(true) + " Already up to date")
		}
		requestExit(0)

	case "branch":
		// Set update branch, normalizing accepted aliases to the canonical name.
		branch, valid := config.NormalizeUpdateChannel(arg)
		if arg == "" || !valid {
			fmt.Printf(terminal.StatusIcon(false)+" Invalid branch: %s\n", arg)
			fmt.Println("   Valid branches: stable, beta, devel (aliases: daily, rolling, dev, development)")
			requestExit(1)
		}

		if err := maint.SetUpdateBranch(branch); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to set branch: %v\n", err)
			requestExit(1)
		}
		fmt.Printf(terminal.StatusIcon(true)+" Update branch set to: %s\n", branch)
		requestExit(0)

	case "--help", "help", "-h":
		// Per AI.md PART 8: --update --help prints help and exits 0
		fmt.Printf(`Update Commands:
  %s --update              Check and perform in-place update with restart
  %s --update yes          Same as --update (default)
  %s --update check        Check for updates without installing
  %s --update branch <name>  Set update branch (stable, beta, devel)

Update Branches:
  stable (default)  Release builds (v*, *.*.*)
  beta              Pre-release builds (*-beta)
  devel             Rolling development builds
  daily, rolling, dev, development  Aliases for devel
`, binaryName, binaryName, binaryName, binaryName)
		requestExit(0)

	default:
		fmt.Printf(terminal.StatusIcon(false)+" Unknown update command: %s\n", cmd)
		fmt.Printf("\nUsage: %s --update [check|yes|branch <name>|--help]\n\nRun '%s --update --help' for detailed help.\n", binaryName, binaryName)
		requestExit(1)
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
		requestExit(1)
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

func handleMaintenanceCommand(cmd, arg, configDir, dataDir string) {
	binaryName := filepath.Base(os.Args[0])
	maint := maintenance.NewMaintenanceManager(configDir, dataDir, version.GetVersion())

	switch cmd {
	case "backup":
		if strings.HasPrefix(arg, "test") && (arg == "test" || strings.HasPrefix(arg, "test ")) {
			backupFile := strings.TrimSpace(strings.TrimPrefix(arg, "test"))
			if strings.HasPrefix(backupFile, "--") {
				fmt.Fprintln(os.Stderr, "Usage: vidveil --maintenance backup test [file]")
				requestExit(2)
			}
			result, err := maint.TestBackup(backupFile, "")
			if err != nil && strings.Contains(err.Error(), "password required") {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					fmt.Fprintln(os.Stderr, "Backup test requires an interactive terminal to enter the backup password.")
					requestExit(1)
				}
				result, err = maint.TestBackup(backupFile, promptPassword("Enter backup password: "))
			}
			fmt.Printf("Backup: %s\n", result.Filename)
			fmt.Printf("Archive checksum: %t\n", result.ChecksumVerified)
			fmt.Printf("Manifest and content: %t\n", result.ManifestVerified)
			fmt.Printf("SQLite database: %t\n", result.DatabaseVerified)
			fmt.Printf("PGP keypair: %t\n", result.PGPKeypairVerified)
			if err != nil {
				fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Backup test failed: %v\n", err)
				requestExit(1)
			}
			fmt.Println(terminal.StatusIcon(true) + " Backup test passed.")
			return
		}
		// Per AI.md PART 21, backups use only the configured secret value and
		// never prompt for or accept a password on the command line.
		appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to load configuration: %v\n", err)
			requestExit(1)
		}
		complianceMode := appConfig.Server.Compliance.IsEnabled()

		// Attach the audit logger so backup.created/verification_failed/etc. per
		// AI.md PART 21 "Audit Events" are recorded for CLI-triggered backups too.
		if cliLogger, closeLogger, err := newCLIAuditLogger(appConfig, configDir, dataDir); err == nil {
			maint.SetLogger(cliLogger)
			defer closeLogger()
		} else {
			fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Backup audit mirror unavailable: %v\n", err)
		}

		password := appConfig.Server.Backup.EncryptionPassword
		if appConfig.Server.Backup.PasswordUnreadable {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" backup.password_unreadable: configured backup password cannot be decrypted")
			requestExit(1)
		}
		if complianceMode && password == "" {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" Compliance mode requires backup encryption: set server.backup.encryption_password in secrets.yml.")
			requestExit(1)
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
				requestExit(1)
			}
		} else {
			fmt.Println(terminal.WarningIcon() + " Creating an unencrypted backup because no backup password is configured.")
			fmt.Println("Creating backup...")
			if err := maint.Backup(arg); err != nil {
				fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Backup failed: %v\n", err)
				requestExit(1)
			}
		}

	case "restore":
		// Per AI.md PART 5 "Sensitive Operations": restore requires server.token OR
		// root OR an empty database (nothing to protect yet).
		if err := authorizeRestore(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			requestExit(1)
		}
		appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to load configuration: %v\n", err)
			requestExit(1)
		}
		if err := maint.ConfigureRestore(appConfig.Server.Restore); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Invalid restore limits: %v\n", err)
			requestExit(1)
		}

		if arg == "" {
			fmt.Println("Restoring from most recent backup...")
		} else {
			fmt.Printf("Restoring from %s...\n", arg)
		}
		// Per AI.md PART 21: no --password flag - only prompt interactively if the
		// backup turns out to be encrypted (avoids prompting for plaintext backups).
		paths := config.GetAppPaths(configDir, dataDir)
		binaryPath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Resolve server binary: %v\n", err)
			requestExit(1)
		}
		binaryName := filepath.Base(os.Args[0])
		controller := system.NewServiceManager(binaryName, binaryPath, paths.Config, paths.Data)
		err = restoreWithServiceLifecycle(controller, paths.PIDFile, binaryName, func() error {
			err := maint.RestoreWithPassword(arg, "")
			if err != nil && strings.Contains(err.Error(), "password required") {
				password := promptPassword("Enter backup password: ")
				err = maint.RestoreWithPassword(arg, password)
			}
			return err
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Restore operation failed: %v\n", err)
			requestExit(1)
		}

	case "mode":
		// Per AI.md PART 5 "Sensitive Operations": --maintenance mode requires
		// server.token OR root (no empty-database exception - it changes live
		// server behavior, not data).
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			requestExit(1)
		}

		if arg == "" {
			fmt.Println(terminal.StatusIcon(false) + " Missing mode argument")
			fmt.Println("   Usage: vidveil --maintenance mode <on|off>")
			requestExit(1)
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
			requestExit(1)
		}

		if err := maint.SetMaintenanceMode(enabled); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed: %v\n", err)
			requestExit(1)
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
			requestExit(1)
		}
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" "+err.Error())
			requestExit(1)
		}
		appConfig, _, err := config.LoadAppConfig(configDir, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to load configuration: %v\n", err)
			requestExit(1)
		}
		requestID, err := newDataRequestID()
		if err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to create report ID: %v\n", err)
			requestExit(1)
		}
		if err := emitDataAudit(configDir, dataDir, "compliance.report_generated", requestID, map[string]interface{}{"report_type": "compliance"}); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Failed to audit compliance report: %v\n", err)
			requestExit(1)
		}
		printComplianceReport(appConfig, config.GetAppPaths(configDir, dataDir).Data)

	case "pgp":
		// Per AI.md PART 12 "GPG Keypair Management": keypair actions run through
		// the --maintenance dispatcher, authorized like other sensitive operations.
		handlePGPMaintenance(arg, configDir, dataDir)

	case "secret":
		// Per AI.md PART 11 "Secret Rotation": installation_secret and
		// encryption_key rotate through the same sensitive-operation flow
		// (operator token OR root + typed confirmation).
		if err := handleMaintenanceSecret(arg, configDir, dataDir); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" "+err.Error())
			requestExit(1)
		}

	case "token":
		// Per AI.md "Operator revocation": token revoke/list are operator-only.
		if err := handleMaintenanceToken(arg, configDir, dataDir); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" "+err.Error())
			requestExit(1)
		}

	case "data":
		// Per AI.md "Compliance-Specific Behaviors": GDPR data export/delete.
		if err := handleMaintenanceData(arg, configDir, dataDir); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" "+err.Error())
			requestExit(1)
		}

	case "security":
		if err := handleMaintenanceSecurity(arg, configDir, dataDir); err != nil {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" "+err.Error())
			requestExit(1)
		}

	case "--help", "help", "-h":
		// Per AI.md PART 8: --maintenance --help prints help and exits 0
		fmt.Printf(`Maintenance Commands:
  %s --maintenance backup [file]      Create backup (uses configured password)
      backup test [file]               Verify archive without touching live data
  %s --maintenance restore [file]     Restore from backup (prompts if encrypted)
  %s --maintenance update             Check and apply updates
  %s --maintenance mode <on|off>      Enable/disable maintenance mode
  %s --maintenance compliance report  Show regulatory compliance summary
  %s --maintenance pgp <action>       Manage the security PGP keypair
  %s --maintenance secret rotate <n>  Rotate installation_secret or encryption_key
  %s --maintenance token list         List active API tokens (prefix + resource)
  %s --maintenance security <action>  Manage coordinated-disclosure reports
  %s --maintenance token revoke <pfx> Revoke a specific API token by prefix
  %s --maintenance data export --owner <prefix> [--output FILE]
                                      Export one owner's data (GDPR)
  %s --maintenance data delete --owner <prefix> [--yes]
                                      Delete one owner's data (GDPR erasure)
  %s --maintenance setup              Show configuration instructions

PGP actions (AI.md PART 12 "GPG Keypair Management"):
  generate                            Generate the security keypair
  rotate                              Rotate the keypair (cross-signs, 30-day grace)
  publish                             Publish the public key to configured keyservers
  export public [path]                Write the public key (stdout if omitted)
  export private <path>               Write the decrypted private key (0600, audited, 1/hour)
  import <file>                       Import an existing private key (identity checked)
  delete                              Delete the keypair (irreversible, typed confirmation)

Secret actions (AI.md PART 11 "Secret Rotation"):
  rotate installation_secret          New 32-byte secret; re-encrypts PGP key (7-day grace)
  rotate encryption_key               New AES-256-GCM key; re-encrypts at-rest data (30-day grace)

Token actions (operator-only):
  list                                Show prefix, resource, and expiry for active tokens
  revoke <prefix>                     Revoke the token matching <prefix>

Data actions (AI.md GDPR/CCPA):
  export [--out <file>]               Write a JSON export of all stored data
  delete                              Erase personal data; audit log retained

Per AI.md PART 21 the backup password is never passed on the command line.
Scheduled and manual backups use the configured password; restore and backup
test prompt only when the selected archive is encrypted.

Examples:
  %s --maintenance backup             # Backup to default location
  %s --maintenance backup backup.tar      # Backup to specific file
  %s --maintenance restore            # Restore from most recent
  %s --maintenance restore backup.tar.gz.enc  # Restore an encrypted archive
  %s --maintenance mode on            # Enable maintenance mode
  %s --maintenance pgp generate       # Generate the security keypair
  %s --maintenance pgp rotate         # Rotate the security keypair
  %s --maintenance pgp publish        # Publish the public key to keyservers
`, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName,
			binaryName, binaryName, binaryName, binaryName, binaryName, binaryName,
			binaryName, binaryName, binaryName, binaryName, binaryName, binaryName, binaryName,
			binaryName, binaryName)
		requestExit(0)

	default:
		fmt.Printf(terminal.StatusIcon(false)+" Unknown maintenance command: %s\n", cmd)
		fmt.Printf("\nUsage: %s --maintenance [backup|restore|update|mode|compliance|pgp|secret|token|security|setup|--help]\n\nRun '%s --maintenance --help' for detailed help.\n", binaryName, binaryName)
		requestExit(1)
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
			requestExit(1)
		}
		pgpGenerate(configDir, dataDir)

	case "rotate":
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			requestExit(1)
		}
		pgpRotate(configDir, dataDir)

	case "publish":
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			requestExit(1)
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
				requestExit(1)
			}
			if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
				fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
				requestExit(1)
			}
			pgpExportPrivate(configDir, dataDir, outPath)
		default:
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" Unsupported export target: %q\n", target)
			fmt.Printf("   Usage: %s --maintenance pgp export <public|private> [path]\n", binaryName)
			requestExit(1)
		}

	case "import":
		inPath := ""
		if len(fields) > 1 {
			inPath = fields[1]
		}
		if inPath == "" {
			fmt.Fprintln(os.Stderr, terminal.StatusIcon(false)+" A source file is required to import a private key.")
			fmt.Printf("   Usage: %s --maintenance pgp import <file>\n", binaryName)
			requestExit(1)
		}
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			requestExit(1)
		}
		pgpImport(configDir, dataDir, inPath)

	case "delete":
		if err := authorizeSensitiveOperation(configDir, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
			requestExit(1)
		}
		pgpDelete(configDir, dataDir)

	default:
		fmt.Printf(terminal.StatusIcon(false)+" Unknown pgp action: %q\n", action)
		fmt.Printf("   Usage: %s --maintenance pgp [generate|rotate|publish|export <public|private> [path]|import <file>|delete]\n", binaryName)
		requestExit(1)
	}
}

// pgpGenerate generates the project security keypair and reports the result,
// exiting non-zero on failure (AI.md PART 12 "Generate").
func pgpGenerate(configDir, dataDir string) {
	kp, identityName, securityContact, pubKeyPath, err := pgpGenerateCore(configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		requestExit(1)
	}
	if _, publishErr := pgpPublishCore(configDir, dataDir); publishErr != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Keypair generated; keyserver retry is scheduled: %v\n", publishErr)
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
	defer func() { _ = dbMgr.Close() }()
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
		requestExit(1)
	}
	if publishResult, publishErr := pgpPublishCore(configDir, dataDir); publishErr != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" Rotation succeeded; keyserver publication will retry: %v\n", publishErr)
	} else if len(publishResult.published) == 0 {
		fmt.Println(terminal.WarningIcon() + " No keyservers configured; rotation key was not published.")
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
	defer func() { _ = dbMgr.Close() }()
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

	rotatedAt := time.Now().UTC()
	unlockSubmissions := secreport.LockKeyRotation()
	defer unlockSubmissions()
	if _, err := pgp.CleanupRetiredPrivateKey(paths.Config, rotatedAt); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(pgp.SecurityDir(paths.Config), pgp.RetiredPrivateKeyFile)); err == nil {
		return nil, fmt.Errorf("previous PGP key is still inside its 30-day retention window")
	}
	if err := pgp.WriteStagedKeypair(paths.Config, newKP, secret); err != nil {
		return nil, fmt.Errorf("failed to stage rotated keypair: %w", err)
	}
	tx, err := dbMgr.GetDB().BeginTx(context.Background(), nil)
	if err != nil {
		_ = pgp.RemoveStagedKeypair(paths.Config)
		return nil, fmt.Errorf("begin PGP rotation transaction: %w", err)
	}
	rollback := func(cause error) (*pgpRotateResult, error) {
		_ = tx.Rollback()
		_ = pgp.RemoveStagedKeypair(paths.Config)
		return nil, cause
	}
	rotationCtx, cancelRotation := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelRotation()
	if _, err := tx.ExecContext(rotationCtx, `UPDATE security_reports SET updated_at = updated_at WHERE tracking_id = ''`); err != nil {
		return rollback(fmt.Errorf("lock security report key rotation: %w", err))
	}
	if err := secreport.ReencryptPGPBodies(rotationCtx, tx, oldPriv, newKP.PublicArmored); err != nil {
		return rollback(err)
	}
	if _, err := tx.ExecContext(rotationCtx, `DELETE FROM pgp_keypair`); err != nil {
		return rollback(fmt.Errorf("update PGP rotation marker: %w", err))
	}
	if _, err := tx.ExecContext(rotationCtx, `INSERT INTO pgp_keypair (fingerprint,created_at,expires_at,last_rotated_at,keyservers_published,revoked) VALUES (?,?,?,?,?,0)`, newKP.Fingerprint, newKP.CreatedAt.UTC(), newKP.ExpiresAt.UTC(), rotatedAt, "[]"); err != nil {
		return rollback(fmt.Errorf("write PGP rotation fingerprint marker: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("PGP rotation transaction outcome is uncertain; staged recovery will reconcile it: %w", err)
	}
	if err := pgp.InstallStagedKeypair(paths.Config); err != nil {
		return nil, fmt.Errorf("PGP rotation committed but staged key install is pending startup recovery: %w", err)
	}
	archiveDir := filepath.Join(pgp.SecurityDir(paths.Config), pgp.RetiredPrivateKeyFile)

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
		requestExit(1)
	}
	if len(res.published) == 0 {
		if res.attempted == 0 {
			fmt.Println(terminal.WarningIcon() + " No keyservers configured; nothing published.")
		} else {
			fmt.Println(terminal.WarningIcon() + " No keyserver submissions are due; any pending retries remain scheduled.")
		}
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
	return pgpPublishCoreWithPublisherContext(context.Background(), configDir, dataDir, pgp.PublishToKeyserversWithState)
}

func pgpPublishCoreWithPublisher(configDir, dataDir string, publish func(context.Context, []byte, string, []string, []pgp.KeyserverState) ([]pgp.KeyserverState, []pgp.KeyserverState, error)) (*pgpPublishResult, error) {
	return pgpPublishCoreWithPublisherContext(context.Background(), configDir, dataDir, publish)
}

func pgpPublishCoreWithPublisherContext(ctx context.Context, configDir, dataDir string, publish func(context.Context, []byte, string, []string, []pgp.KeyserverState) ([]pgp.KeyserverState, []pgp.KeyserverState, error)) (*pgpPublishResult, error) {
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

	serverDBPath := filepath.Join(paths.Data, "db", "server.db")
	dbMgr, err := database.NewMigrationManager(serverDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = dbMgr.Close() }()
	if err := dbMgr.RunMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	meta, err := pgp.GetKeypairMeta(dbMgr.GetDB())
	if err != nil {
		return nil, fmt.Errorf("failed to read keypair metadata: %w", err)
	}
	if meta == nil {
		return nil, fmt.Errorf("keypair metadata is unavailable")
	}
	previous, err := pgp.LoadKeyserverState(paths.Config)
	if err != nil {
		return nil, err
	}
	legacyPublished := make(map[string]struct{}, len(meta.KeyserversPublished))
	for _, state := range meta.KeyserversPublished {
		legacyPublished[state.URL] = struct{}{}
	}
	currentState := make([]pgp.KeyserverState, 0, len(previous))
	for _, state := range previous {
		if state.Fingerprint == "" {
			if _, wasPublished := legacyPublished[state.URL]; !wasPublished {
				continue
			}
			state.Fingerprint = meta.Fingerprint
		}
		currentState = append(currentState, state)
	}
	states, published, pubErr := publish(ctx, pub, meta.Fingerprint, keyservers, currentState)
	res := &pgpPublishResult{published: published, attempted: len(keyservers)}

	if err := pgp.WriteKeyserverState(paths.Config, states); err != nil {
		return res, fmt.Errorf("failed to write keyserver state: %w", err)
	}
	if len(published) > 0 {
		var allPublished []pgp.KeyserverState
		for _, state := range states {
			if state.Fingerprint == meta.Fingerprint && !state.PublishedAt.IsZero() {
				allPublished = append(allPublished, state)
			}
		}
		if err := pgp.UpdateKeyserversPublished(dbMgr.GetDB(), allPublished); err != nil {
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
		requestExit(1)
	}
	if outPath == "" {
		_, _ = os.Stdout.Write(pub)
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
		requestExit(1)
	}

	if err := pgpExportPrivateCore(configDir, dataDir, outPath, operator, localOperatorIP(), reason, time.Now().UTC()); err != nil {
		fmt.Fprintf(os.Stderr, terminal.StatusIcon(false)+" %v\n", err)
		requestExit(1)
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
	logger, closeLogger, err := newCLIAuditLogger(appConfig, configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" private key exported but audit log unavailable: %v\n", err)
		return
	}
	defer closeLogger()
	logger.Audit("security.private_key_exported", operator, "operator", operatorIP, "success",
		map[string]interface{}{"reason": reason, "path": outPath})
}

// localOperatorIP identifies local CLI audit events without inventing a remote
// address for an operation that has no network peer.
func localOperatorIP() string {
	return "local"
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
		requestExit(1)
	}
	if res == nil {
		fmt.Println(terminal.WarningIcon() + " Import aborted: identity mismatch was not confirmed.")
		requestExit(1)
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
	defer func() { _ = dbMgr.Close() }()
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
	logger, closeLogger, err := newCLIAuditLogger(appConfig, configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" private key imported but audit log unavailable: %v\n", err)
		return
	}
	defer closeLogger()
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
		requestExit(1)
	}
	if !done {
		fmt.Println(terminal.WarningIcon() + " Deletion aborted: confirmation not provided.")
		requestExit(1)
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
	defer func() { _ = dbMgr.Close() }()
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
	logger, closeLogger, err := newCLIAuditLogger(appConfig, configDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, terminal.WarningIcon()+" keypair deleted but audit log unavailable: %v\n", err)
		return
	}
	defer closeLogger()
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
	defer func() { _ = dbMgr.Close() }()
	if err := dbMgr.RunMigrations(); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	secretsMgr := secret.NewManager(dbMgr.GetDB())
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
func printComplianceReport(appConfig *config.AppConfig, dataDirs ...string) {
	c := appConfig.Server.Compliance
	enabled := c.EnabledStandards()
	dataDir := ""
	if len(dataDirs) > 0 {
		dataDir = dataDirs[0]
	}

	fmt.Println("Compliance Report")
	fmt.Println("=================")
	fmt.Printf("Application: %s\n", appConfig.Server.Branding.Title)
	fmt.Printf("Generated:   %s\n", time.Now().Format("January 02, 2006 at 15:04:05 MST"))

	if dataDir != "" {
		printComplianceDataSummary(dataDir, appConfig)
	}
	fmt.Println("\nStandards:")
	for _, standard := range c.Standards() {
		state := "disabled"
		if standard.Enabled {
			state = "enabled"
		}
		fmt.Printf("  %s: %s\n", strings.ToUpper(standard.Key), state)
	}

	if len(enabled) == 0 {
		fmt.Println("Compliance mode: DISABLED")
		fmt.Println("No regulatory standards are enabled. Backups are optional and")
		fmt.Println("encryption is not mandatory. Enable standards under")
		fmt.Println("server.compliance in server.yml.")
		return
	}

	fmt.Println("Compliance mode: ENABLED")
	fmt.Println("\nActive controls (derived from enabled standards):")
	fmt.Println("  - Backup encryption is MANDATORY (--maintenance backup requires a password)")
	if c.GDPR || c.CCPA || c.LGPD {
		fmt.Println("  - Data-subject requests: --maintenance data export / data delete")
	}
	fmt.Println("  - Audit logging active for compliance.* events")
	fmt.Println("\nCompliance is configured in server.yml under server.compliance.")
}

func printComplianceDataSummary(dataDir string, appConfig *config.AppConfig) {
	paths := config.GetAppPaths("", dataDir)
	counts := map[string]string{"owners": "unavailable", "resources": "unavailable", "active tokens": "unavailable", "open security reports": "unavailable", "compliance events (30 days)": "unavailable"}
	driver := strings.ToLower(strings.TrimSpace(appConfig.Server.Database.Driver))
	remote := driver == "libsql" || driver == "turso"
	dbDir := appConfig.Server.Database.SQLite.Dir
	if dbDir == "" {
		dbDir = config.GetDatabaseDir(paths.Data)
	}
	dbName := appConfig.Server.Database.SQLite.ServerDB
	if dbName == "" {
		dbName = "server.db"
	}
	dbPath := filepath.Join(dbDir, dbName)
	var db *sql.DB
	var closeDB func()
	if remote {
		if manager, err := openAuditMirrorDatabase(appConfig, "", dataDir); err == nil {
			db = manager.GetDB()
			closeDB = func() { _ = manager.Close() }
		}
	} else if _, err := os.Stat(dbPath); err == nil {
		if opened, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro"); err == nil {
			db = opened
			closeDB = func() { _ = opened.Close() }
		}
	}
	if db != nil {
		tablePrefix := ""
		if remote {
			tablePrefix = "srv_"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		queries := map[string]string{
			"owners":                      "SELECT COUNT(*) FROM " + tablePrefix + "owners",
			"resources":                   "SELECT COUNT(*) FROM " + tablePrefix + "favorites",
			"active tokens":               "SELECT COUNT(*) FROM " + tablePrefix + "api_tokens WHERE revoked_at IS NULL AND (expires_at IS NULL OR expires_at > CAST(strftime('%s','now') AS INTEGER))",
			"open security reports":       "SELECT COUNT(*) FROM " + tablePrefix + "security_reports WHERE status NOT IN ('closed','resolved','disclosed')",
			"compliance events (30 days)": "SELECT COUNT(*) FROM " + tablePrefix + "audit_log WHERE action LIKE 'compliance.%' AND timestamp >= CAST(strftime('%s','now','-30 days') AS INTEGER)",
		}
		for label, query := range queries {
			var count int64
			if err := db.QueryRowContext(ctx, query).Scan(&count); err == nil {
				counts[label] = strconv.FormatInt(count, 10)
			}
		}
		cancel()
		closeDB()
	}
	fmt.Println("\nData summary:")
	for _, label := range []string{"owners", "resources", "active tokens", "open security reports", "compliance events (30 days)"} {
		fmt.Printf("  %s: %s\n", label, counts[label])
	}
	retention := appConfig.Server.Privacy.Retention.Period
	if retention == "" {
		retention = "not configured"
	}
	fmt.Printf("Retention period: %s\n", retention)
	backupRetention := appConfig.Server.Backup.Retention
	fmt.Printf("Backup retention: daily=%d, weekly=%d, monthly=%d, yearly=%d, max_total_size=%s\n",
		backupRetention.MaxBackups, backupRetention.KeepWeekly, backupRetention.KeepMonthly,
		backupRetention.KeepYearly, backupRetention.MaxTotalSize)
	backupDir := paths.Backup
	lastBackup := "never"
	if entries, err := os.ReadDir(backupDir); err == nil {
		var newest time.Time
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), "vidveil_backup_") {
				continue
			}
			info, err := entry.Info()
			if err == nil && info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
		if !newest.IsZero() {
			lastBackup = newest.Format("January 02, 2006 at 15:04:05 MST")
		}
	}
	fmt.Printf("Last backup: %s\n", lastBackup)
	auditPath := appConfig.Server.Logs.Audit.Filename
	if auditPath == "" {
		auditPath = "audit.log"
	}
	if !filepath.IsAbs(auditPath) {
		auditPath = filepath.Join(paths.Log, auditPath)
	}
	fmt.Printf("Audit log integrity: %s\n", logging.AuditLogIntegrity(auditPath, filepath.Join(filepath.Dir(auditPath), "audit.anchor")))
}

// isDBFirstRun returns true if the settings table has no rows, indicating first run.
// A missing or inaccessible table also counts as first run.
// Per AI.md PART 10
// all queries MUST have timeouts (5s for reads), so the query runs under an
// explicit context rather than the bare *sql.DB, which would block startup
// indefinitely if the database is unreachable or blocked.
func isDBFirstRun(db *sql.DB) bool {
	var count int
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings").Scan(&count)
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
	// Fail closed: without a config directory the operator token cannot be
	// read, so the check below would have nothing to compare against. Treat it
	// as unauthorized rather than silently falling through to a default path.
	if configDir == "" {
		return fmt.Errorf("requires administrator authorization: no config directory to verify the operator token")
	}

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
	defer func() { _ = migrationMgr.Close() }()
	return isDBFirstRun(migrationMgr.GetDB())
}

func getDisplayAddress(serverConfig *config.AppConfig) string {
	// Per AI.md PART 8: Never show 0.0.0.0, 127.0.0.1, localhost, etc.
	return net.JoinHostPort(config.GetDisplayHost(serverConfig), serverConfig.Server.Port)
}
