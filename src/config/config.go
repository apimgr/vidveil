// SPDX-License-Identifier: MIT
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ProjectOrg  = "apimgr"
	ProjectName = "vidveil"
)

// Version is set at build time via ldflags
var Version = "dev"

// Config holds all application configuration per TEMPLATE.md spec
type Config struct {
	Server ServerConfig `yaml:"server"`
	Web    WebConfig    `yaml:"web"`
	Search SearchConfig `yaml:"search"`
}

// ServerConfig holds server-related settings per TEMPLATE.md
type ServerConfig struct {
	// Port: single (HTTP) or dual (HTTP,HTTPS) e.g., "8090" or "8090,64453"
	Port    string `yaml:"port"`
	FQDN    string `yaml:"fqdn"`
	Address string `yaml:"address"`

	// Application mode: production or development
	// Can be overridden by MODE env var or --mode CLI flag
	Mode string `yaml:"mode"`

	// Application branding
	Title       string `yaml:"title"`
	Description string `yaml:"description"`

	// System user/group
	User  string `yaml:"user"`
	Group string `yaml:"group"`

	// PID file
	PIDFile bool `yaml:"pidfile"`

	// Admin panel configuration
	Admin AdminConfig `yaml:"admin"`

	// Email/SMTP
	Email EmailConfig `yaml:"email"`

	// Notifications
	Notifications NotificationsConfig `yaml:"notifications"`

	// Scheduler
	Schedule ScheduleConfig `yaml:"schedule"`

	// SSL/TLS
	SSL SSLConfig `yaml:"ssl"`

	// Metrics
	Metrics MetricsConfig `yaml:"metrics"`

	// Logging
	Logs LogsConfig `yaml:"logs"`

	// Rate limiting
	RateLimit RateLimitConfig `yaml:"rate_limit"`

	// Request limits
	Limits LimitsConfig `yaml:"limits"`

	// Compression
	Compression CompressionConfig `yaml:"compression"`

	// Trusted proxies
	TrustedProxies TrustedProxiesConfig `yaml:"trusted_proxies"`

	// Security headers
	SecurityHeaders SecurityHeadersConfig `yaml:"security_headers"`

	// Session
	Session SessionConfig `yaml:"session"`

	// Cache
	Cache CacheConfig `yaml:"cache"`

	// Database
	Database DatabaseConfig `yaml:"database"`

	// GeoIP
	GeoIP GeoIPConfig `yaml:"geoip"`

	// Users (PART 31)
	Users UsersConfig `yaml:"users"`
}

// AdminConfig holds admin panel settings
type AdminConfig struct {
	Email       string          `yaml:"email"`
	Username    string          `yaml:"username"`
	Password    string          `yaml:"password"`
	Token       string          `yaml:"token"`
	TwoFactor   TwoFactorConfig `yaml:"two_factor"`
}

// TwoFactorConfig holds 2FA settings per TEMPLATE.md PART 31
type TwoFactorConfig struct {
	// 2FA is enabled for this admin
	Enabled bool `yaml:"enabled"`
	// TOTP secret (stored securely)
	Secret string `yaml:"secret,omitempty"`
	// One-time backup codes
	BackupCodes []string `yaml:"backup_codes,omitempty"`
	// Trust device for N days
	RememberDeviceDays int `yaml:"remember_device_days"`
}

// EmailConfig holds SMTP settings
type EmailConfig struct {
	Enabled        bool     `yaml:"enabled"`
	Autodetect     bool     `yaml:"autodetect"`
	AutodetectHost []string `yaml:"autodetect_hosts"`
	AutodetectPort []int    `yaml:"autodetect_ports"`
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
	From           string   `yaml:"from"`
	TLS            string   `yaml:"tls"`
}

// NotificationsConfig holds notification settings
type NotificationsConfig struct {
	Enabled bool                    `yaml:"enabled"`
	Email   bool                    `yaml:"email"`
	Bell    bool                    `yaml:"bell"`
	Types   NotificationTypesConfig `yaml:"types"`
}

// NotificationTypesConfig holds which events to notify
type NotificationTypesConfig struct {
	Startup    bool `yaml:"startup"`
	Shutdown   bool `yaml:"shutdown"`
	Error      bool `yaml:"error"`
	Security   bool `yaml:"security"`
	Update     bool `yaml:"update"`
	CertExpiry bool `yaml:"cert_expiry"`
}

// ScheduleConfig holds scheduler settings
type ScheduleConfig struct {
	Enabled       bool   `yaml:"enabled"`
	CertRenewal   string `yaml:"cert_renewal"`
	Notifications string `yaml:"notifications"`
	Cleanup       string `yaml:"cleanup"`
}

// SSLConfig holds SSL/TLS settings
type SSLConfig struct {
	Enabled     bool             `yaml:"enabled"`
	CertPath    string           `yaml:"cert_path"`
	LetsEncrypt LetsEncryptConfig `yaml:"letsencrypt"`
}

// LetsEncryptConfig holds Let's Encrypt settings
type LetsEncryptConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Domain          string `yaml:"domain"`
	Email           string `yaml:"email"`
	Challenge       string `yaml:"challenge"`
	DNSProviderType string `yaml:"dns_provider_type"`
	DNSProviderKey  string `yaml:"dns_provider_key"`
}

// MetricsConfig holds Prometheus metrics settings
type MetricsConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Endpoint      string `yaml:"endpoint"`
	IncludeSystem bool   `yaml:"include_system"`
	Token         string `yaml:"token"`
}

// GeoIPConfig holds GeoIP settings per TEMPLATE.md PART 10
type GeoIPConfig struct {
	Enabled       bool              `yaml:"enabled"`
	Dir           string            `yaml:"dir"`
	Update        string            `yaml:"update"`
	DenyCountries []string          `yaml:"deny_countries"`
	Databases     GeoIPDatabasesConfig `yaml:"databases"`
}

// GeoIPDatabasesConfig holds which GeoIP databases to use
type GeoIPDatabasesConfig struct {
	ASN     bool `yaml:"asn"`
	Country bool `yaml:"country"`
	City    bool `yaml:"city"`
}

// UsersConfig holds user management settings per TEMPLATE.md PART 31
type UsersConfig struct {
	// Enable multi-user mode (default: false = admin-only)
	Enabled bool `yaml:"enabled"`
	// Registration settings
	Registration RegistrationConfig `yaml:"registration"`
	// Role configuration
	Roles RolesConfig `yaml:"roles"`
	// API token settings
	Tokens TokensConfig `yaml:"tokens"`
	// Profile settings
	Profile ProfileConfig `yaml:"profile"`
	// Authentication settings
	Auth UserAuthConfig `yaml:"auth"`
	// Per-user rate limits
	Limits UserLimitsConfig `yaml:"limits"`
}

// RegistrationConfig holds user registration settings
type RegistrationConfig struct {
	// Allow public registration
	Enabled bool `yaml:"enabled"`
	// Require email verification
	RequireEmailVerification bool `yaml:"require_email_verification"`
	// Admin must approve new users
	RequireApproval bool `yaml:"require_approval"`
	// Allowed email domains (empty = all)
	AllowedDomains []string `yaml:"allowed_domains"`
	// Blocked email domains
	BlockedDomains []string `yaml:"blocked_domains"`
}

// RolesConfig holds role configuration
type RolesConfig struct {
	// Available roles
	Available []string `yaml:"available"`
	// Default role for new users
	Default string `yaml:"default"`
}

// TokensConfig holds API token settings
type TokensConfig struct {
	// Allow users to generate API tokens
	Enabled bool `yaml:"enabled"`
	// Maximum tokens per user
	MaxPerUser int `yaml:"max_per_user"`
	// Token expiration (0 = never)
	ExpirationDays int `yaml:"expiration_days"`
}

// ProfileConfig holds user profile settings
type ProfileConfig struct {
	// Allow users to upload avatars
	AllowAvatar bool `yaml:"allow_avatar"`
	// Allow users to set display name
	AllowDisplayName bool `yaml:"allow_display_name"`
	// Allow users to set bio
	AllowBio bool `yaml:"allow_bio"`
}

// UserAuthConfig holds user authentication settings
type UserAuthConfig struct {
	// Session duration (e.g., "30d")
	SessionDuration string `yaml:"session_duration"`
	// Require 2FA for all users
	Require2FA bool `yaml:"require_2fa"`
	// Allow 2FA (user choice)
	Allow2FA bool `yaml:"allow_2fa"`
	// Minimum password length
	PasswordMinLength int `yaml:"password_min_length"`
	// Require uppercase
	PasswordRequireUppercase bool `yaml:"password_require_uppercase"`
	// Require number
	PasswordRequireNumber bool `yaml:"password_require_number"`
	// Require special character
	PasswordRequireSpecial bool `yaml:"password_require_special"`
}

// UserLimitsConfig holds per-user rate limit settings
type UserLimitsConfig struct {
	// Rate limit per minute (0 = use global)
	RequestsPerMinute int `yaml:"requests_per_minute"`
	// Rate limit per day (0 = use global)
	RequestsPerDay int `yaml:"requests_per_day"`
}

// LogsConfig holds logging settings per TEMPLATE.md PART 21
type LogsConfig struct {
	Level  string         `yaml:"level"`
	Debug  DebugLogConfig `yaml:"debug"`
	Access AccessLogConfig `yaml:"access"`
	Server ServerLogConfig `yaml:"server"`
	// TEMPLATE.md PART 21: error.log
	Error    ErrorLogConfig    `yaml:"error"`
	Audit    AuditLogConfig    `yaml:"audit"`
	Security SecurityLogConfig `yaml:"security"`
}

// DebugLogConfig holds debug log settings
type DebugLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename"`
	Format   string `yaml:"format"`
	Keep     string `yaml:"keep"`
	Rotate   string `yaml:"rotate"`
}

// AccessLogConfig holds access log settings
type AccessLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename"`
	Format   string `yaml:"format"`
	Keep     string `yaml:"keep"`
	Rotate   string `yaml:"rotate"`
}

// ServerLogConfig holds server log settings
type ServerLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename"`
	Format   string `yaml:"format"`
	Keep     string `yaml:"keep"`
	Rotate   string `yaml:"rotate"`
}

// ErrorLogConfig holds error log settings per TEMPLATE.md PART 21
type ErrorLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename"`
	Format   string `yaml:"format"`
	Keep     string `yaml:"keep"`
	Rotate   string `yaml:"rotate"`
}

// AuditLogConfig holds audit log settings
type AuditLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename"`
	Format   string `yaml:"format"`
	Keep     string `yaml:"keep"`
	Rotate   string `yaml:"rotate"`
}

// SecurityLogConfig holds security log settings
type SecurityLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename"`
	Format   string `yaml:"format"`
	Keep     string `yaml:"keep"`
	Rotate   string `yaml:"rotate"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	Enabled  bool `yaml:"enabled"`
	Requests int  `yaml:"requests"`
	Window   int  `yaml:"window"`
}

// LimitsConfig holds request limit settings
type LimitsConfig struct {
	MaxBodySize  string `yaml:"max_body_size"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
	IdleTimeout  string `yaml:"idle_timeout"`
}

// CompressionConfig holds compression settings
type CompressionConfig struct {
	Enabled bool     `yaml:"enabled"`
	Level   int      `yaml:"level"`
	Types   []string `yaml:"types"`
}

// TrustedProxiesConfig holds trusted proxy settings
type TrustedProxiesConfig struct {
	Additional []string `yaml:"additional"`
}

// SecurityHeadersConfig holds security header settings
type SecurityHeadersConfig struct {
	Enabled             bool   `yaml:"enabled"`
	HSTS                bool   `yaml:"hsts"`
	HSTSMaxAge          int    `yaml:"hsts_max_age"`
	XFrameOptions       string `yaml:"x_frame_options"`
	XContentTypeOptions string `yaml:"x_content_type_options"`
	XXSSProtection      string `yaml:"x_xss_protection"`
	ReferrerPolicy      string `yaml:"referrer_policy"`
	CSP                 string `yaml:"csp"`
}

// SessionConfig holds session settings
type SessionConfig struct {
	CookieName string `yaml:"cookie_name"`
	MaxAge     int    `yaml:"max_age"`
	Secure     string `yaml:"secure"`
	HTTPOnly   bool   `yaml:"http_only"`
	SameSite   string `yaml:"same_site"`
}

// CacheConfig holds cache settings
type CacheConfig struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Prefix   string `yaml:"prefix"`
	TTL      int    `yaml:"ttl"`
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Driver string       `yaml:"driver"`
	SQLite SQLiteConfig `yaml:"sqlite"`
	// For Postgres/MySQL
	Host string `yaml:"host"`
	// For Postgres/MySQL
	Port int `yaml:"port"`
	// Database name for Postgres/MySQL
	Name string `yaml:"name"`
	// For Postgres/MySQL
	User string `yaml:"user"`
	// For Postgres/MySQL
	Password string `yaml:"password"`
	// disable, require, verify-ca, verify-full
	SSLMode string `yaml:"ssl_mode"`
}

// SQLiteConfig holds SQLite settings
type SQLiteConfig struct {
	Dir         string `yaml:"dir"`
	ServerDB    string `yaml:"server_db"`
	JournalMode string `yaml:"journal_mode"`
	BusyTimeout int    `yaml:"busy_timeout"`
}

// WebConfig holds frontend settings per TEMPLATE.md
type WebConfig struct {
	UI            UIConfig            `yaml:"ui"`
	Announcements AnnouncementsConfig `yaml:"announcements"`
	Robots        RobotsConfig        `yaml:"robots"`
	Security      WebSecurityConfig   `yaml:"security"`
	CORS          string              `yaml:"cors"`
	CSRF          CSRFConfig          `yaml:"csrf"`
	Footer        FooterConfig        `yaml:"footer"`
}

// UIConfig holds UI settings
type UIConfig struct {
	Theme   string `yaml:"theme"`
	Logo    string `yaml:"logo"`
	Favicon string `yaml:"favicon"`
}

// AnnouncementsConfig holds announcement settings
type AnnouncementsConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Messages []string `yaml:"messages"`
}

// RobotsConfig holds robots.txt settings
type RobotsConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// WebSecurityConfig holds security.txt settings
type WebSecurityConfig struct {
	Contact string `yaml:"contact"`
	Expires string `yaml:"expires"`
}

// CSRFConfig holds CSRF settings
type CSRFConfig struct {
	Enabled     bool   `yaml:"enabled"`
	TokenLength int    `yaml:"token_length"`
	CookieName  string `yaml:"cookie_name"`
	HeaderName  string `yaml:"header_name"`
	Secure      string `yaml:"secure"`
}

// FooterConfig holds footer settings
type FooterConfig struct {
	TrackingID    string              `yaml:"tracking_id"`
	CookieConsent CookieConsentConfig `yaml:"cookie_consent"`
	CustomHTML    string              `yaml:"custom_html"`
}

// CookieConsentConfig holds cookie consent settings
type CookieConsentConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Message    string `yaml:"message"`
	PolicyText string `yaml:"policy_text"`
	PolicyURL  string `yaml:"policy_url"`
}

// SearchConfig holds search-specific settings (project-specific)
type SearchConfig struct {
	DefaultEngines     []string `yaml:"default_engines"`
	ConcurrentRequests int      `yaml:"concurrent_requests"`
	EngineTimeout      int      `yaml:"engine_timeout"`
	ResultsPerPage     int      `yaml:"results_per_page"`
	MaxPages           int      `yaml:"max_pages"`
	// Minimum video duration (default 300 = 5 min)
	MinDurationSeconds int `yaml:"min_duration_seconds"`
	// Filter out premium/gold content
	FilterPremium bool `yaml:"filter_premium"`
	// Use spoofed TLS fingerprint (Chrome) to bypass Cloudflare
	SpoofTLS        bool                  `yaml:"spoof_tls"`
	Tor             TorConfig             `yaml:"tor"`
	AgeVerification AgeVerificationConfig `yaml:"age_verification"`
}

// TorConfig holds Tor proxy settings
type TorConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Proxy            string `yaml:"proxy"`
	ForceAll         bool   `yaml:"force_all"`
	RotateCircuit    bool   `yaml:"rotate_circuit"`
	ControlPort      int    `yaml:"control_port"`
	ControlPassword  string `yaml:"control_password"`
	Timeout          int    `yaml:"timeout"`
	ClearnetFallback bool   `yaml:"clearnet_fallback"`
}

// AgeVerificationConfig holds age verification settings
type AgeVerificationConfig struct {
	Enabled    bool `yaml:"enabled"`
	CookieDays int  `yaml:"cookie_days"`
}

// Paths holds resolved directory paths
type Paths struct {
	Config string
	Data   string
	Log    string
	Backup string
}

// Default returns a Config with sensible defaults per TEMPLATE.md
func Default() *Config {
	fqdn := getHostname()
	randomPort := findUnusedPort()

	return &Config{
		Server: ServerConfig{
			Port:        strconv.Itoa(randomPort),
			FQDN:        fqdn,
			Address:     "[::]",
			Mode:        "production",
			Title:       "Vidveil",
			Description: "Privacy-respecting adult video search",
			User:        "",
			Group:       "",
			PIDFile:     true,
			Admin: AdminConfig{
				Email:    "admin@" + fqdn,
				Username: "administrator",
				Password: generateToken(16),
				Token:    generateToken(32),
				TwoFactor: TwoFactorConfig{
					Enabled:            false,
					RememberDeviceDays: 30,
				},
			},
			Email: EmailConfig{
				Enabled:        false,
				Autodetect:     true,
				AutodetectHost: []string{"localhost", "172.17.0.1"},
				AutodetectPort: []int{25, 465, 587},
				Port:           587,
				From:           "no-reply@" + fqdn,
				TLS:            "auto",
			},
			Notifications: NotificationsConfig{
				Enabled: true,
				Email:   true,
				Bell:    true,
				Types: NotificationTypesConfig{
					Startup:    true,
					Shutdown:   true,
					Error:      true,
					Security:   true,
					Update:     true,
					CertExpiry: true,
				},
			},
			Schedule: ScheduleConfig{
				Enabled:       true,
				CertRenewal:   "daily",
				Notifications: "hourly",
				Cleanup:       "weekly",
			},
			SSL: SSLConfig{
				Enabled:  false,
				CertPath: "",
				LetsEncrypt: LetsEncryptConfig{
					Enabled:   false,
					Challenge: "http-01",
				},
			},
			Metrics: MetricsConfig{
				Enabled:       false,
				Endpoint:      "/metrics",
				IncludeSystem: true,
			},
			Logs: LogsConfig{
				Level: "info",
				Debug: DebugLogConfig{
					Enabled:  false,
					Filename: "debug.log",
					Format:   "text",
					Keep:     "none",
					Rotate:   "monthly",
				},
				Access: AccessLogConfig{
					Filename: "access.log",
					Format:   "apache",
					Keep:     "none",
					Rotate:   "monthly",
				},
				Server: ServerLogConfig{
					Filename: "server.log",
					Format:   "text",
					Keep:     "none",
					Rotate:   "weekly,50MB",
				},
				Error: ErrorLogConfig{
					Filename: "error.log",
					Format:   "text",
					Keep:     "none",
					Rotate:   "weekly,50MB",
				},
				Audit: AuditLogConfig{
					Filename: "audit.log",
					Format:   "json",
					Keep:     "none",
					Rotate:   "monthly",
				},
				Security: SecurityLogConfig{
					Filename: "security.log",
					Format:   "fail2ban",
					Keep:     "none",
					Rotate:   "monthly",
				},
			},
			RateLimit: RateLimitConfig{
				Enabled:  true,
				Requests: 120,
				Window:   60,
			},
			Limits: LimitsConfig{
				MaxBodySize:  "10MB",
				ReadTimeout:  "30s",
				WriteTimeout: "30s",
				IdleTimeout:  "120s",
			},
			Compression: CompressionConfig{
				Enabled: true,
				Level:   5,
				Types: []string{
					"text/html",
					"text/css",
					"text/javascript",
					"application/json",
					"application/xml",
				},
			},
			TrustedProxies: TrustedProxiesConfig{
				Additional: []string{},
			},
			SecurityHeaders: SecurityHeadersConfig{
				Enabled:             true,
				HSTS:                true,
				HSTSMaxAge:          31536000,
				XFrameOptions:       "SAMEORIGIN",
				XContentTypeOptions: "nosniff",
				XXSSProtection:      "1; mode=block",
				ReferrerPolicy:      "strict-origin-when-cross-origin",
				CSP:                 "default-src 'self'; img-src 'self' https: data:; style-src 'self' 'unsafe-inline'",
			},
			Session: SessionConfig{
				CookieName: "session_id",
				// 30 days
				MaxAge:   2592000,
				Secure:   "auto",
				HTTPOnly: true,
				SameSite: "lax",
			},
			Cache: CacheConfig{
				Type:   "memory",
				Host:   "localhost",
				Port:   6379,
				DB:     0,
				Prefix: "vidveil:",
				TTL:    3600,
			},
			Database: DatabaseConfig{
				Driver: "file",
				SQLite: SQLiteConfig{
					Dir:         "",
					ServerDB:    "server.db",
					JournalMode: "WAL",
					BusyTimeout: 5000,
				},
			},
			GeoIP: GeoIPConfig{
				Enabled:       true,
				Dir:           "",
				Update:        "weekly",
				DenyCountries: []string{},
				Databases: GeoIPDatabasesConfig{
					ASN:     true,
					Country: true,
					City:    false,
				},
			},
			// Admin-only mode by default per TEMPLATE.md
			Users: UsersConfig{
				Enabled: false,
				Registration: RegistrationConfig{
					Enabled:                  false,
					RequireEmailVerification: true,
					RequireApproval:          false,
					AllowedDomains:           []string{},
					BlockedDomains:           []string{},
				},
				Roles: RolesConfig{
					Available: []string{"admin", "user"},
					Default:   "user",
				},
				Tokens: TokensConfig{
					Enabled:    true,
					MaxPerUser: 5,
					// Never expire
					ExpirationDays: 0,
				},
				Profile: ProfileConfig{
					AllowAvatar:      true,
					AllowDisplayName: true,
					AllowBio:         true,
				},
				Auth: UserAuthConfig{
					SessionDuration:          "30d",
					Require2FA:               false,
					Allow2FA:                 true,
					PasswordMinLength:        8,
					PasswordRequireUppercase: false,
					PasswordRequireNumber:    false,
					PasswordRequireSpecial:   false,
				},
				// Use global rate limits
				Limits: UserLimitsConfig{
					RequestsPerMinute: 0,
					RequestsPerDay:    0,
				},
			},
		},
		Web: WebConfig{
			UI: UIConfig{
				Theme: "dark",
			},
			Announcements: AnnouncementsConfig{
				Enabled:  true,
				Messages: []string{},
			},
			Robots: RobotsConfig{
				Allow: []string{"/"},
				Deny:  []string{"/admin", "/api/v1/admin"},
			},
			Security: WebSecurityConfig{
				Contact: "security@" + fqdn,
			},
			CORS: "*",
			CSRF: CSRFConfig{
				Enabled:     true,
				TokenLength: 32,
				CookieName:  "csrf_token",
				HeaderName:  "X-CSRF-Token",
				Secure:      "auto",
			},
			Footer: FooterConfig{
				CookieConsent: CookieConsentConfig{
					Enabled:    false,
					Message:    "This site uses cookies for age verification.",
					PolicyText: "Privacy Policy",
					PolicyURL:  "/about#privacy",
				},
			},
		},
		Search: SearchConfig{
			DefaultEngines:     []string{},
			ConcurrentRequests: 10,
			EngineTimeout:      15,
			ResultsPerPage:     50,
			MaxPages:           10,
			// No minimum duration by default
			MinDurationSeconds: 0,
			FilterPremium:      true,
			// Disabled by default - can cause issues with some engines
			// Enable only for Cloudflare-protected sites
			SpoofTLS: false,
			Tor: TorConfig{
				Enabled:          false,
				Proxy:            "socks5://127.0.0.1:9050",
				ForceAll:         false,
				RotateCircuit:    false,
				ControlPort:      9051,
				Timeout:          30,
				ClearnetFallback: true,
			},
			AgeVerification: AgeVerificationConfig{
				Enabled:    true,
				CookieDays: 30,
			},
		},
	}
}

// GetPaths returns OS-appropriate paths
func GetPaths(configDir, dataDir string) *Paths {
	isRoot := os.Geteuid() == 0

	paths := &Paths{}

	if configDir != "" {
		paths.Config = configDir
	} else {
		paths.Config = getDefaultConfigDir(isRoot)
	}

	if dataDir != "" {
		paths.Data = dataDir
	} else {
		paths.Data = getDefaultDataDir(isRoot)
	}

	paths.Log = getDefaultLogDir(isRoot)
	paths.Backup = getDefaultBackupDir(isRoot)

	return paths
}

// Load loads configuration from file or creates default
func Load(configDir, dataDir string) (*Config, string, error) {
	paths := GetPaths(configDir, dataDir)

	// Ensure directories exist
	for _, dir := range []string{paths.Config, paths.Data, paths.Log} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	configPath := filepath.Join(paths.Config, "server.yml")

	// Check for .yaml migration
	yamlPath := filepath.Join(paths.Config, "server.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			os.Rename(yamlPath, configPath)
			fmt.Printf("📝 Migrated server.yaml to server.yml\n")
		}
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		cfg := Default()

		// Set paths in config
		cfg.Server.SSL.CertPath = filepath.Join(paths.Config, "ssl", "certs")
		cfg.Server.Database.SQLite.Dir = filepath.Join(paths.Data, "db")

		if err := Save(cfg, configPath); err != nil {
			return nil, "", fmt.Errorf("failed to save default config: %w", err)
		}

		// Console output is handled in main.go per TEMPLATE.md PART 31

		return cfg, configPath, nil
	}

	// Load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read config: %w", err)
	}

	// Start with defaults
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, "", fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, configPath, nil
}

// Save saves configuration to file
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comment
	header := `# =============================================================================
# Vidveil Configuration
# =============================================================================
# This file follows the apimgr TEMPLATE.md specification
# Documentation: https://github.com/apimgr/vidveil
# =============================================================================

`
	fullData := []byte(header + string(data))

	if err := os.WriteFile(path, fullData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Helper functions

// ParseBool parses a boolean value from various string representations
// per TEMPLATE.md PART 6 NON-NEGOTIABLE
// Truthy: 1, yes, true, enable, enabled, on
// Falsy: 0, no, false, disable, disabled, off, "" (empty)
func ParseBool(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "1", "yes", "true", "enable", "enabled", "on":
		return true
	case "0", "no", "false", "disable", "disabled", "off", "":
		return false
	default:
		return false
	}
}

// ParseBoolEnv parses a boolean value from an environment variable
func ParseBoolEnv(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return ParseBool(val)
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return hostname
}

func findUnusedPort() int {
	// Find random port in 64xxx range
	for port := 64000; port < 65000; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	// Fallback
	return 64080
}

func generateToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func getDefaultConfigDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/etc/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", ProjectOrg, ProjectName)
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Application Support/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName)
		}
		return filepath.Join(os.Getenv("APPDATA"), ProjectOrg, ProjectName)
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/usr/local/etc/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", ProjectOrg, ProjectName)
	}
}

func getDefaultDataDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/var/lib/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName)
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Application Support/%s/%s/data", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName, "data")
		}
		return filepath.Join(os.Getenv("LocalAppData"), ProjectOrg, ProjectName)
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/var/db/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName)
	}
}

func getDefaultLogDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/var/log/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName, "logs")
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Logs/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Logs", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), ProjectOrg, ProjectName, "logs")
		}
		return filepath.Join(os.Getenv("LocalAppData"), ProjectOrg, ProjectName, "logs")
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/var/log/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", ProjectOrg, ProjectName, "logs")
	}
}

func getDefaultBackupDir(isRoot bool) string {
	switch runtime.GOOS {
	case "linux":
		if isRoot {
			return fmt.Sprintf("/mnt/Backups/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "backups", ProjectOrg, ProjectName)
	case "darwin":
		if isRoot {
			return fmt.Sprintf("/Library/Backups/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Backups", ProjectOrg, ProjectName)
	case "windows":
		if isRoot {
			return filepath.Join(os.Getenv("ProgramData"), "Backups", ProjectOrg, ProjectName)
		}
		return filepath.Join(os.Getenv("LocalAppData"), "Backups", ProjectOrg, ProjectName)
	// BSD and other Unix-like systems
	default:
		if isRoot {
			return fmt.Sprintf("/var/backups/%s/%s", ProjectOrg, ProjectName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "backups", ProjectOrg, ProjectName)
	}
}

// IsContainer detects if running in a container (tini as PID 1)
func IsContainer() bool {
	// Check if PID 1 is tini
	if data, err := os.ReadFile("/proc/1/comm"); err == nil {
		return strings.TrimSpace(string(data)) == "tini"
	}
	// Check for container environment variables
	if os.Getenv("container") != "" {
		return true
	}
	return false
}

// IsDevelopmentMode returns true if running in development mode
func (c *Config) IsDevelopmentMode() bool {
	mode := strings.ToLower(c.Server.Mode)
	return mode == "development" || mode == "dev"
}

// IsProductionMode returns true if running in production mode
func (c *Config) IsProductionMode() bool {
	return !c.IsDevelopmentMode()
}

// NormalizeMode normalizes the mode string to "production" or "development"
func NormalizeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "dev", "development":
		return "development"
	case "prod", "production", "":
		return "production"
	default:
		return "production"
	}
}

// TEMPLATE.md PART 13: URL/FQDN Detection

// devOnlyTLDs are TLDs allowed only in development mode per TEMPLATE.md
var devOnlyTLDs = []string{
	".localhost", ".test", ".example", ".invalid",
	".local", ".lan", ".internal", ".home", ".localdomain",
	".home.arpa", ".intranet", ".corp", ".private",
}

// IsValidHost validates a host per TEMPLATE.md PART 13
// In production mode, only valid FQDNs are allowed (no IPs, no localhost, no dev TLDs)
// In development mode, localhost and dev TLDs are allowed (still no IPs)
func IsValidHost(host string, devMode bool) bool {
	lower := strings.ToLower(host)

	// Reject IP addresses always
	if net.ParseIP(host) != nil {
		return false
	}

	// Handle localhost - only valid in dev mode
	if lower == "localhost" {
		return devMode
	}

	// Must contain at least one dot (except localhost in dev)
	if !strings.Contains(host, ".") {
		return false
	}

	// In production, reject dev-only TLDs
	if !devMode {
		for _, tld := range devOnlyTLDs {
			if strings.HasSuffix(lower, tld) {
				return false
			}
		}
	}

	return true
}

// IsValidSSLHost validates host for SSL/Let's Encrypt per TEMPLATE.md
// SSL always requires production-valid host (devMode=false)
func IsValidSSLHost(host string) bool {
	return IsValidHost(host, false)
}

// LiveReload per TEMPLATE.md PART 1 NON-NEGOTIABLE
// Watches config file and reloads on changes

// ReloadCallback is called when configuration is reloaded
type ReloadCallback func(*Config)

// ConfigWatcher watches for config file changes
type ConfigWatcher struct {
	configPath string
	cfg        *Config
	callbacks  []ReloadCallback
	stopChan   chan struct{}
	lastMod    int64
}

// NewWatcher creates a new config watcher
func NewWatcher(configPath string, cfg *Config) *ConfigWatcher {
	info, _ := os.Stat(configPath)
	var lastMod int64
	if info != nil {
		lastMod = info.ModTime().UnixNano()
	}

	return &ConfigWatcher{
		configPath: configPath,
		cfg:        cfg,
		callbacks:  make([]ReloadCallback, 0),
		stopChan:   make(chan struct{}),
		lastMod:    lastMod,
	}
}

// OnReload registers a callback for config reload events
func (w *ConfigWatcher) OnReload(callback ReloadCallback) {
	w.callbacks = append(w.callbacks, callback)
}

// Start begins watching for config changes
func (w *ConfigWatcher) Start() {
	go w.watch()
}

// Stop stops watching for config changes
func (w *ConfigWatcher) Stop() {
	close(w.stopChan)
}

// watch polls the config file for changes
func (w *ConfigWatcher) watch() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			info, err := os.Stat(w.configPath)
			if err != nil {
				continue
			}

			modTime := info.ModTime().UnixNano()
			if modTime > w.lastMod {
				w.lastMod = modTime
				w.reload()
			}
		}
	}
}

// reload reloads the configuration and notifies callbacks
func (w *ConfigWatcher) reload() {
	data, err := os.ReadFile(w.configPath)
	if err != nil {
		fmt.Printf("⚠️  Failed to read config for reload: %v\n", err)
		return
	}

	newCfg := Default()
	if err := yaml.Unmarshal(data, newCfg); err != nil {
		fmt.Printf("⚠️  Failed to parse config for reload: %v\n", err)
		return
	}

	// Update the shared config (settings that can live-reload)
	w.cfg.Server.Title = newCfg.Server.Title
	w.cfg.Server.Description = newCfg.Server.Description
	w.cfg.Server.RateLimit = newCfg.Server.RateLimit
	w.cfg.Server.Email = newCfg.Server.Email
	w.cfg.Server.Notifications = newCfg.Server.Notifications
	w.cfg.Server.Schedule = newCfg.Server.Schedule
	w.cfg.Server.SSL.LetsEncrypt = newCfg.Server.SSL.LetsEncrypt
	w.cfg.Server.Metrics = newCfg.Server.Metrics
	w.cfg.Server.Logs = newCfg.Server.Logs
	w.cfg.Server.GeoIP = newCfg.Server.GeoIP
	w.cfg.Web = newCfg.Web
	w.cfg.Search = newCfg.Search

	fmt.Printf("🔄 Configuration reloaded\n")

	// Notify callbacks
	for _, callback := range w.callbacks {
		callback(w.cfg)
	}
}

// Reload forces a configuration reload
func (w *ConfigWatcher) Reload() error {
	w.reload()
	return nil
}

// GetDisplayHost returns the appropriate host for display per TEMPLATE.md lines 2333-2457
// Never shows: 0.0.0.0, 127.0.0.1, localhost, [::]
// Uses global IP if dev TLD or localhost detected
func GetDisplayHost(_ *Config) string {
	fqdn := GetFQDN()

	// If valid production FQDN and not localhost, use it (lines 2443-2445)
	if !isDevTLD(fqdn) && !isLoopback(fqdn) {
		return fqdn
	}

	// Dev TLD or localhost - use global IP instead (lines 2448-2454)
	if ipv6 := getGlobalIPv6(); ipv6 != "" {
		return "[" + ipv6 + "]"
	}
	if ipv4 := getGlobalIPv4(); ipv4 != "" {
		return ipv4
	}

	// Last resort (line 2457)
	return fqdn
}

// GetFQDN returns the FQDN per TEMPLATE.md lines 2333-2366
func GetFQDN() string {
	// 1. DOMAIN env var (explicit user override)
	if domain := os.Getenv("DOMAIN"); domain != "" {
		return domain
	}

	// 2. os.Hostname() - cross-platform
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		if !isLoopback(hostname) {
			return hostname
		}
	}

	// 3. $HOSTNAME env var (skip loopback)
	if hostname := os.Getenv("HOSTNAME"); hostname != "" {
		if !isLoopback(hostname) {
			return hostname
		}
	}

	// 4. Global IPv6 (preferred for modern networks)
	if ipv6 := getGlobalIPv6(); ipv6 != "" {
		return ipv6
	}

	// 5. Global IPv4
	if ipv4 := getGlobalIPv4(); ipv4 != "" {
		return ipv4
	}

	// Last resort (not recommended)
	return "localhost"
}

// isLoopback checks if host is a loopback address per TEMPLATE.md lines 2368-2377
func isLoopback(host string) bool {
	lower := strings.ToLower(host)
	if lower == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// isDevTLD checks if FQDN is a dev TLD per TEMPLATE.md lines 2420-2432
func isDevTLD(fqdn string) bool {
	lower := strings.ToLower(fqdn)
	if lower == "localhost" {
		return true
	}

	// Development TLD suffixes including dynamic project TLD
	devSuffixes := []string{
		".local", ".test", ".example", ".invalid",
		".localhost", ".lan", ".internal", ".home", ".localdomain",
		".home.arpa", ".intranet", ".corp", ".private",
		"." + ProjectName,
	}
	for _, suffix := range devSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// getGlobalIPv6 returns first global unicast IPv6 address per TEMPLATE.md lines 2379-2392
func getGlobalIPv6() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.To4() == nil && ipnet.IP.IsGlobalUnicast() {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// getGlobalIPv4 returns first global unicast IPv4 address per TEMPLATE.md lines 2394-2407
func getGlobalIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ip4 := ipnet.IP.To4(); ip4 != nil && ipnet.IP.IsGlobalUnicast() {
				return ip4.String()
			}
		}
	}
	return ""
}
