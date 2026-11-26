// SPDX-License-Identifier: MIT
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func seedFirstRunEnvironment(cfg *AppConfig) {
	if cfg == nil {
		return
	}
	if value := os.Getenv("APPLICATION_NAME"); value != "" {
		cfg.Server.Branding.Title = value
	}
	if value := os.Getenv("APPLICATION_TAGLINE"); value != "" {
		cfg.Server.Branding.Tagline = value
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		if driver := os.Getenv("DATABASE_DRIVER"); driver != "" {
			cfg.Server.Database.Driver = driver
		}
	} else {
		cfg.Server.Database.URL = databaseURL
		if driver := databaseDriverForURL(databaseURL); driver != "" {
			if requested := os.Getenv("DATABASE_DRIVER"); requested != "" && normalizeDatabaseDriver(requested) != driver {
				fmt.Fprintln(os.Stderr, "Warning: DATABASE_URL scheme selects the database driver; DATABASE_DRIVER is ignored")
			}
			cfg.Server.Database.Driver = driver
		}
	}

	if cacheURL := os.Getenv("CACHE_URL"); cacheURL != "" {
		cfg.Server.Cache.URL = cacheURL
		if cacheType := cacheTypeForURL(cacheURL); cacheType != "" {
			cfg.Server.Cache.Type = cacheType
		}
	}
}

func databaseDriverForURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "libsql", "libsqls", "turso":
		return "libsql"
	case "sqlite", "sqlite2", "sqlite3", "file":
		return "sqlite"
	default:
		return ""
	}
}

func normalizeDatabaseDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "libsql", "turso":
		return "libsql"
	case "sqlite", "sqlite2", "sqlite3", "file", "":
		return "sqlite"
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

func cacheTypeForURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "valkey", "valkeys":
		return "valkey"
	case "redis", "rediss":
		return "redis"
	default:
		return ""
	}
}

func applyRuntimeEnvironmentOverrides(cfg *AppConfig) {
	if cfg == nil {
		return
	}
	value, ok := os.LookupEnv("I2P_ENABLED")
	if !ok {
		return
	}
	enabled, err := ParseBool(value, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid boolean for I2P_ENABLED: %q (ignored)\n", value)
		return
	}
	cfg.Server.I2P.Enabled = enabled
}
