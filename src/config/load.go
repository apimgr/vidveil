// SPDX-License-Identifier: MIT
package config

import (
	"errors"
	"flag"
	"io"
	"os"
	"strings"
)

func Load(args []string) (ServerConfig, error) {
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
		return ServerConfig{}, err
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
	loadConfig := !runtime.Help && !runtime.Version && !runtimeCommandSkipsConfig(command)
	if !loadConfig {
		cfg := *DefaultAppConfig()
		cfg.Runtime = runtime
		return cfg, nil
	}
	cfg, configPath, err := LoadAppConfig(*configDir, *dataDir)
	if err != nil {
		return ServerConfig{}, err
	}
	cfg.Runtime = runtime
	cfg.Runtime.ConfigPath = configPath
	if cfg.Runtime.Address == "" {
		cfg.Runtime.Address = cfg.Server.Address
	}
	if cfg.Runtime.Port == "" {
		cfg.Runtime.Port = cfg.Server.Port
	}
	if cfg.Runtime.Mode == "" {
		cfg.Runtime.Mode = cfg.Server.Mode
	}
	_, _, cfg.Runtime.PeekDaemonize = PeekServerStartupConfig(configPath)
	return *cfg, nil
}

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
		case "tor", "--status", "--shell", "--service", "--update", "--maintenance":
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
