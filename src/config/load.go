// SPDX-License-Identifier: MIT
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	pathsecurity "github.com/apimgr/vidveil/src/paths"
)

func ParseRuntime(args []string) (ServerConfig, error) {
	globalArgs, command := splitRuntimeCommand(args)
	fs := flag.NewFlagSet("vidveil", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configDir := fs.String("config", "", "Configuration directory")
	dataDir := fs.String("data", "", "Data directory")
	cacheDir := fs.String("cache", "", "Cache directory")
	logDir := fs.String("log", "", "Log directory")
	backupDir := fs.String("backup", "", "Backup directory")
	pidFile := fs.String("pid", "", "PID file path")
	address := fs.String("address", "", "Listen address")
	port := fs.String("port", "", "Listen port")
	baseURL := fs.String("baseurl", "", "URL path prefix")
	mode := fs.String("mode", "", "Application mode")
	color := fs.String("color", "", "Color output: auto, yes, no")
	language := fs.String("lang", "", "Output language")
	debug := fs.Bool("debug", false, "Enable debug logging")
	i2p := fs.Bool("i2p", false, "Enable the optional I2P eepsite")
	daemon := fs.Bool("daemon", false, "Run as background daemon")
	showVersion := fs.Bool("version", false, "Show version and exit")
	showHelp := fs.Bool("help", false, "Show help and exit")
	fs.BoolVar(showVersion, "v", false, "Show version and exit (alias)")
	fs.BoolVar(showHelp, "h", false, "Show help and exit (alias)")
	if err := fs.Parse(globalArgs); err != nil && !errors.Is(err, flag.ErrHelp) {
		return ServerConfig{}, FlagError{Err: err}
	}
	parsedFlags := make(map[string]bool)
	fs.Visit(func(parsed *flag.Flag) { parsedFlags[parsed.Name] = true })

	if *configDir == "" {
		*configDir = os.Getenv("CONFIG_DIR")
	}
	if *dataDir == "" {
		*dataDir = os.Getenv("DATA_DIR")
	}
	if *cacheDir == "" {
		*cacheDir = os.Getenv("CACHE_DIR")
	}
	if *logDir == "" {
		*logDir = os.Getenv("LOG_DIR")
	}
	if *backupDir == "" {
		*backupDir = os.Getenv("BACKUP_DIR")
	}
	if *pidFile == "" {
		*pidFile = os.Getenv("PID_FILE")
	}
	if *address == "" {
		*address = os.Getenv("LISTEN")
	}
	if *port == "" {
		*port = os.Getenv("PORT")
	}
	if *baseURL == "" {
		*baseURL = os.Getenv("BASEURL")
	}
	if *language == "" {
		*language = os.Getenv("LC_ALL")
	}
	if *language == "" {
		*language = os.Getenv("LANG")
	}
	if *mode == "" {
		*mode = os.Getenv("MODE")
	}
	if !*debug && !parsedFlags["debug"] && truthyRuntimeEnv(os.Getenv("DEBUG")) {
		*debug = true
	}
	for _, directory := range []struct {
		name  string
		value *string
	}{
		{"config", configDir}, {"data", dataDir}, {"cache", cacheDir}, {"log", logDir}, {"backup", backupDir},
	} {
		if *directory.value == "" {
			continue
		}
		safe, err := pathsecurity.SafeDirPath(*directory.value)
		if err != nil {
			return ServerConfig{}, fmt.Errorf("invalid --%s directory: %w", directory.name, err)
		}
		*directory.value = safe
	}
	setRuntimePathEnv("CONFIG_DIR", *configDir)
	setRuntimePathEnv("DATA_DIR", *dataDir)
	setRuntimePathEnv("CACHE_DIR", *cacheDir)
	setRuntimePathEnv("LOG_DIR", *logDir)
	setRuntimePathEnv("BACKUP_DIR", *backupDir)
	setRuntimePathEnv("BASEURL", *baseURL)
	setRuntimePathEnv("LANG", *language)

	runtime := RuntimeConfig{
		ConfigDir:  *configDir,
		DataDir:    *dataDir,
		CacheDir:   *cacheDir,
		LogDir:     *logDir,
		BackupDir:  *backupDir,
		PIDFile:    *pidFile,
		Address:    *address,
		Port:       *port,
		BaseURL:    *baseURL,
		Mode:       *mode,
		Color:      *color,
		Language:   *language,
		Debug:      *debug,
		DebugSet:   parsedFlags["debug"],
		I2P:        *i2p,
		I2PSet:     parsedFlags["i2p"],
		Daemon:     *daemon,
		PIDFileSet: parsedFlags["pid"],
		Arguments:  append([]string(nil), args...),
		Command:    append([]string(nil), command...),
		Help:       *showHelp,
		Version:    *showVersion,
	}
	// Configuration files and directories are loaded only after startup path
	// resolution and the privileged staged-update check in app.run.
	cfg := *DefaultAppConfig()
	cfg.Runtime = runtime
	return cfg, nil
}

func Load(args []string) (ServerConfig, error) {
	cfg, err := ParseRuntime(args)
	if err != nil {
		return ServerConfig{}, err
	}
	if cfg.Runtime.Help || cfg.Runtime.Version || runtimeCommandSkipsConfig(cfg.Runtime.Command) {
		return cfg, nil
	}
	appConfig, configPath, err := LoadAppConfig(cfg.Runtime.ConfigDir, cfg.Runtime.DataDir)
	if err != nil {
		return ServerConfig{}, err
	}
	appConfig.Runtime = cfg.Runtime
	appConfig.Runtime.ConfigPath = configPath
	if appConfig.Runtime.Address == "" {
		appConfig.Runtime.Address = appConfig.Server.Address
	}
	if appConfig.Runtime.Port == "" {
		appConfig.Runtime.Port = appConfig.Server.Port
	}
	if appConfig.Runtime.Mode == "" {
		appConfig.Runtime.Mode = appConfig.Server.Mode
	}
	_, _, appConfig.Runtime.PeekDaemonize = PeekServerStartupConfig(configPath)
	return *appConfig, nil
}

// FlagError distinguishes command-line usage errors from configuration and
// runtime failures so the server can return the required usage exit code.
type FlagError struct {
	Err error
}

func (err FlagError) Error() string { return fmt.Sprintf("%v", err.Err) }

func (err FlagError) Unwrap() error { return err.Err }

func truthyRuntimeEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitRuntimeCommand(args []string) ([]string, []string) {
	valueFlags := map[string]struct{}{
		"--config": {}, "-config": {},
		"--data": {}, "-data": {},
		"--cache": {}, "-cache": {},
		"--log": {}, "-log": {},
		"--backup": {}, "-backup": {},
		"--pid": {}, "-pid": {},
		"--address": {}, "-address": {},
		"--port": {}, "-port": {},
		"--baseurl": {}, "-baseurl": {},
		"--mode": {}, "-mode": {},
		"--color": {}, "-color": {},
		"--lang": {}, "-lang": {},
	}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		flagName, _, hasInlineValue := strings.Cut(arg, "=")
		if !hasInlineValue {
			if _, consumesValue := valueFlags[flagName]; consumesValue {
				index++
				continue
			}
		}
		switch arg {
		case "--status", "--shell", "--service", "--update", "--maintenance":
			return args[:index], args[index:]
		}
	}
	return args, nil
}

func runtimeCommandSkipsConfig(command []string) bool {
	if len(command) == 0 {
		return false
	}
	if command[0] == "--maintenance" && len(command) == 1 {
		return false
	}
	return true
}

func setRuntimePathEnv(name, value string) {
	if value == "" || strings.TrimSpace(value) == "" {
		return
	}
	_ = os.Setenv(name, value)
}
