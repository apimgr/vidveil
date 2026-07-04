// SPDX-License-Identifier: MIT
package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/paths"
	"gopkg.in/yaml.v3"
)

// paths.ProjectOrg and paths.ProjectName are defined in paths package

// Version is set at build time via ldflags
var Version = "dev"

// Config holds all application configuration per AI.md spec
type AppConfig struct {
	Server  ServerConfig  `yaml:"server"`
	Web     WebConfig     `yaml:"web"`
	Search  SearchConfig  `yaml:"search"`
	Engines EnginesConfig `yaml:"engines"`

	// Runtime-only state (never serialised to YAML)
	// Set by ConfigWatcher when port/address changes require a restart.
	PendingRestart bool     `yaml:"-" json:"-"`
	RestartReasons []string `yaml:"-" json:"-"`
}

// EnginesConfig holds engine-specific settings
type EnginesConfig struct {
	UserAgent UserAgentConfig `yaml:"useragent"`
}

// ServerBrandingConfig holds branding settings per AI.md PART 16
type ServerBrandingConfig struct {
	Title       string `yaml:"title"`
	Tagline     string `yaml:"tagline"`
	Description string `yaml:"description"`
}

// ServerConfig holds server-related settings per AI.md
type ServerConfig struct {
	// Port: single (HTTP) or dual (HTTP,HTTPS) e.g., "8090" or "8090,64453"
	Port    string `yaml:"port"`
	FQDN    string `yaml:"fqdn"`
	Address string `yaml:"address"`

	// Application mode: production or development
	// Can be overridden by MODE env var or --mode CLI flag
	Mode string `yaml:"mode"`

	// Application branding per AI.md PART 16
	Branding ServerBrandingConfig `yaml:"branding"`

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

	// Security (PART 11) - Blocklists, CVE, etc
	Security SecurityConfig `yaml:"security"`

	// Backup (PART 21) - Backup & Restore settings
	Backup BackupConfig `yaml:"backup"`

	// Tor (PART 31) - Hidden service and outbound network settings
	Tor TorConfig `yaml:"tor"`

	// Healthz (PART 13) - Optional root-level alias for /server/healthz
	// Canonical route is /server/healthz; root /healthz is opt-in
	Healthz HealthzConfig `yaml:"healthz"`

	// SEO holds SEO and social metadata settings per AI.md PART 16
	SEO SEOConfig `yaml:"seo"`
}

// HealthzConfig holds health-check route configuration per AI.md PART 13
type HealthzConfig struct {
	// Optional root-level /healthz alias to the canonical /server/healthz handler
	Root HealthzRootConfig `yaml:"root"`
}

// HealthzRootConfig gates the optional /healthz route per AI.md PART 5/13
type HealthzRootConfig struct {
	// When true, mount /healthz to the SAME handler as /server/healthz (NEVER redirect)
	// Default: false. Spec: "Optional root health alias"
	Enabled bool `yaml:"enabled"`
}

// TorConfig holds Tor-related configuration per AI.md PART 31
type TorConfig struct {
	// Binary path (empty = auto-detect from PATH)
	Binary string `yaml:"binary"`

	// --- Outbound Network Settings ---
	// Use Tor network for outbound connections (engine queries)
	// Per PART 31: Particularly relevant for VidVeil to anonymize search queries
	UseNetwork bool `yaml:"use_network"`

	// Allow users to set their own Tor network preference (override server default)
	// Per PART 31: Users can set via cookie to always use Tor, never use Tor, or inherit server default
	AllowUserPreference bool `yaml:"allow_user_preference"`

	// Allow users to opt-in to forwarding their IP address to video sites
	// When enabled, users can set a preference (via cookie) to include their IP
	// in X-Forwarded-For header - useful for geo-targeted content
	// Default: true (feature available), but user preference defaults to disabled
	AllowUserIPForward bool `yaml:"allow_user_ip_forward"`

	// --- Performance Settings ---
	// Maximum circuits to keep open (1-128, default 32)
	MaxCircuits int `yaml:"max_circuits"`

	// Circuit timeout in seconds (10-300, default 60)
	CircuitTimeout int `yaml:"circuit_timeout"`

	// Bootstrap timeout in seconds (30-600, default 180)
	BootstrapTimeout int `yaml:"bootstrap_timeout"`

	// --- Security Settings ---
	// Scrub sensitive info from Tor logs (default true)
	SafeLogging bool `yaml:"safe_logging"`

	// Maximum concurrent streams per circuit (10-500, default 100)
	MaxStreamsPerCircuit int `yaml:"max_streams_per_circuit"`

	// Close circuit when max streams exceeded (default true)
	CloseCircuitOnStreamLimit bool `yaml:"close_circuit_on_stream_limit"`

	// --- Bandwidth Settings ---
	// Maximum bandwidth rate per second (e.g., "1 MB", "500 KB")
	BandwidthRate string `yaml:"bandwidth_rate"`

	// Maximum bandwidth burst per second (e.g., "2 MB", "1 MB")
	BandwidthBurst string `yaml:"bandwidth_burst"`

	// Maximum monthly bandwidth (e.g., "100 GB", "50 TB", "unlimited")
	// AccountingMax in torrc - resets on 1st of each month
	MaxMonthlyBandwidth string `yaml:"max_monthly_bandwidth"`

	// --- Hidden Service Settings ---
	// Number of introduction points (3-10, default 3)
	NumIntroPoints int `yaml:"num_intro_points"`

	// Virtual port for hidden service (1-65535, default 80)
	VirtualPort int `yaml:"virtual_port"`
}

// DefaultTorConfig returns the default Tor configuration per PART 31
func DefaultTorConfig() TorConfig {
	return TorConfig{
		// auto-detect
		Binary: "",
		// disabled by default, user can enable for privacy
		UseNetwork: false,
		// allow users to override outbound Tor routing per PART 31
		AllowUserPreference: true,
		// feature available, but user must opt-in via preferences
		AllowUserIPForward:        true,
		MaxCircuits:               32,
		CircuitTimeout:            60,
		BootstrapTimeout:          180,
		SafeLogging:               true,
		MaxStreamsPerCircuit:      100,
		CloseCircuitOnStreamLimit: true,
		BandwidthRate:             "1 MB",
		BandwidthBurst:            "2 MB",
		MaxMonthlyBandwidth:       "100 GB",
		NumIntroPoints:            3,
		VirtualPort:               80,
	}
}

// AdminConfig holds admin panel settings
type AdminConfig struct {
	// Path is the admin panel URL path (default: "admin") per PART 12
	Path      string          `yaml:"path"`
	Email     string          `yaml:"email"`
	Username  string          `yaml:"username"`
	Password  string          `yaml:"password"`
	Token     string          `yaml:"token"`
	TwoFactor TwoFactorConfig `yaml:"two_factor"`
}

// TwoFactorConfig holds 2FA settings per AI.md PART 11
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

// EmailConfig holds SMTP settings per AI.md PART 17
type EmailConfig struct {
	Enabled        bool     `yaml:"enabled"`
	Autodetect     bool     `yaml:"autodetect"`
	AutodetectHost []string `yaml:"autodetect_hosts"`
	AutodetectPort []int    `yaml:"autodetect_ports"`
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
	// From is the legacy single-string sender address kept for backwards compat.
	// New code should use FromName + FromEmail.
	From      string `yaml:"from"`
	FromName  string `yaml:"from_name"`
	FromEmail string `yaml:"from_email"`
	TLS       string `yaml:"tls"`
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

// ScheduleConfig holds scheduler settings per AI.md PART 18
type ScheduleConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Timezone      string `yaml:"timezone"`
	CatchUpWindow string `yaml:"catch_up_window"`
	CertRenewal   string `yaml:"cert_renewal"`
	Notifications string `yaml:"notifications"`
	Cleanup       string `yaml:"cleanup"`
}

// SSLConfig holds SSL/TLS settings
type SSLConfig struct {
	Enabled     bool              `yaml:"enabled"`
	CertPath    string            `yaml:"cert_path"`
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

// MetricsConfig holds Prometheus metrics settings per AI.md PART 20
type MetricsConfig struct {
	Enabled         bool      `yaml:"enabled"`
	Endpoint        string    `yaml:"endpoint"`
	IncludeSystem   bool      `yaml:"include_system"`
	IncludeRuntime  bool      `yaml:"include_runtime"`
	Token           string    `yaml:"token"`
	DurationBuckets []float64 `yaml:"duration_buckets"`
	SizeBuckets     []float64 `yaml:"size_buckets"`
}

// GeoIPConfig holds GeoIP settings per AI.md PART 19
type GeoIPConfig struct {
	Enabled        bool                 `yaml:"enabled"`
	Dir            string               `yaml:"dir"`
	Update         string               `yaml:"update"`
	DenyCountries  []string             `yaml:"deny_countries"`
	AllowCountries []string             `yaml:"allow_countries"`
	Databases      GeoIPDatabasesConfig `yaml:"databases"`
	// Content restriction for adult content laws
	ContentRestriction ContentRestrictionConfig `yaml:"content_restriction"`
}

// GeoIPDatabasesConfig holds which GeoIP databases to use
type GeoIPDatabasesConfig struct {
	ASN     bool `yaml:"asn"`
	Country bool `yaml:"country"`
	City    bool `yaml:"city"`
}

// ContentRestrictionConfig holds settings for geographic content restrictions
// Some jurisdictions have laws restricting adult content access
type ContentRestrictionConfig struct {
	// Mode: "off", "warn", "soft_block", "hard_block" (default: "warn")
	// - off: no restriction checks
	// - warn: show dismissable warning banner
	// - soft_block: interstitial page requiring acknowledgment
	// - hard_block: completely block access
	Mode string `yaml:"mode"`
	// RestrictedCountries is a list of ISO country codes (e.g., ["IN", "PK"])
	RestrictedCountries []string `yaml:"restricted_countries"`
	// RestrictedRegions is a list of "COUNTRY:REGION" codes (e.g., ["US:TX", "US:UT"])
	// Region names should match GeoIP subdivision names
	RestrictedRegions []string `yaml:"restricted_regions"`
	// BypassTor allows Tor users to bypass restriction checks (default: true)
	BypassTor bool `yaml:"bypass_tor"`
	// WarningMessage is the message shown for warn/soft_block modes
	WarningMessage string `yaml:"warning_message"`
}

// LogsConfig holds logging settings per AI.md PART 11
type LogsConfig struct {
	Level  string          `yaml:"level"`
	Debug  DebugLogConfig  `yaml:"debug"`
	Access AccessLogConfig `yaml:"access"`
	Server ServerLogConfig `yaml:"server"`
	// AI.md PART 11: error.log
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

// ErrorLogConfig holds error log settings per AI.md PART 11
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

// AllowlistEntry represents a trusted IP/CIDR entry per AI.md PART 11
type AllowlistEntry struct {
	// CIDR is an IP or CIDR notation (e.g., "192.168.1.0/24", "2001:db8::1")
	// Single IPs without a prefix are auto-expanded: /32 for IPv4, /128 for IPv6
	CIDR string `yaml:"cidr" json:"cidr"`
	// Description is a human-readable label (required for clarity)
	Description string `yaml:"description" json:"description"`
}

// SecurityConfig holds security-related settings per PART 11
type SecurityConfig struct {
	Dir        string           `yaml:"dir"`
	Allowlist  []AllowlistEntry `yaml:"allowlist"`
	Blocklists BlocklistsConfig `yaml:"blocklists"`
	CVE        CVEConfig        `yaml:"cve"`
}

// BlocklistsConfig holds IP/domain blocklist settings per PART 11
type BlocklistsConfig struct {
	Enabled bool              `yaml:"enabled"`
	Sources []BlocklistSource `yaml:"sources"`
}

// BlocklistSource represents a blocklist source per PART 11
type BlocklistSource struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	// Type is "ip" or "domain"
	Type    string `yaml:"type"`
	Enabled bool   `yaml:"enabled"`
}

// CVEConfig holds CVE database settings per PART 11
type CVEConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Source      string `yaml:"source"`
	FilterByCPE bool   `yaml:"filter_by_cpe"`
}

// BackupConfig holds backup settings per AI.md PART 21
type BackupConfig struct {
	Retention  BackupRetentionConfig  `yaml:"retention"`
	Encryption BackupEncryptionConfig `yaml:"encryption"`
}

// BackupRetentionConfig holds backup retention settings per AI.md PART 21
type BackupRetentionConfig struct {
	// MaxBackups: daily full backups to keep (default: 1)
	MaxBackups int `yaml:"max_backups"`
	// KeepWeekly: weekly backups (Sunday) to keep (0 = disabled)
	KeepWeekly int `yaml:"keep_weekly"`
	// KeepMonthly: monthly backups (1st of month) to keep (0 = disabled)
	KeepMonthly int `yaml:"keep_monthly"`
	// KeepYearly: yearly backups (Jan 1st) to keep (0 = disabled)
	KeepYearly int `yaml:"keep_yearly"`
}

// BackupEncryptionConfig holds backup encryption settings per AI.md PART 21
type BackupEncryptionConfig struct {
	// Enabled: true if backup password was set
	Enabled bool `yaml:"enabled"`
	// PasswordHint: optional hint for password (never store actual password)
	PasswordHint string `yaml:"password_hint,omitempty"`
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

// WebConfig holds frontend settings per AI.md
// Note: Branding is under server.branding per AI.md PART 5/16, not here
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
	// PGPKeyURL is the URL of the published PGP public key (set when a keypair is generated).
	// When non-empty, an Encryption: line is added to security.txt.
	PGPKeyURL string `yaml:"pgp_key_url"`
}

// SEOCustomTag holds a custom site verification meta tag per AI.md PART 16
type SEOCustomTag struct {
	Name     string `yaml:"name"`
	Property string `yaml:"property"`
	Content  string `yaml:"content"`
}

// SEOVerificationConfig holds search engine verification codes per AI.md PART 16
// All codes are validated before rendering (empty = skip, invalid = error logged, not rendered)
type SEOVerificationConfig struct {
	// Google: alphanumeric+hyphen+underscore, max 43 chars
	Google string `yaml:"google"`
	// Bing: uppercase hex, max 32 chars
	Bing string `yaml:"bing"`
	// Yandex: lowercase hex, max 32 chars
	Yandex string `yaml:"yandex"`
	// Baidu: alphanumeric, max 32 chars
	Baidu string `yaml:"baidu"`
	// Pinterest: lowercase hex, max 32 chars
	Pinterest string `yaml:"pinterest"`
	// Facebook: lowercase alphanumeric, max 64 chars
	Facebook string `yaml:"facebook"`
	// Custom: additional verification tags (validated before rendering)
	Custom []SEOCustomTag `yaml:"custom"`
}

// SEOConfig holds SEO/social metadata per AI.md PART 16
type SEOConfig struct {
	// Keywords for <meta name="keywords"> (if non-empty)
	Keywords []string `yaml:"keywords"`
	// Author for <meta name="author"> (if non-empty)
	Author string `yaml:"author"`
	// OGImage is the OpenGraph/Twitter card image URL
	OGImage string `yaml:"og_image"`
	// TwitterHandle is the @handle for twitter:site card
	TwitterHandle string `yaml:"twitter_handle"`
	// Verification holds search engine verification codes
	Verification SEOVerificationConfig `yaml:"verification"`
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
// Per PART 31: Tor supports hidden service and optional outbound network routing
type SearchConfig struct {
	DefaultEngines     []string `yaml:"default_engines"`
	ConcurrentRequests int      `yaml:"concurrent_requests"`
	EngineTimeout      int      `yaml:"engine_timeout"`
	ResultsPerPage     int      `yaml:"results_per_page"`
	MaxPages           int      `yaml:"max_pages"`
	// Minimum video duration in seconds (default 600 = 10 minutes)
	MinDurationSeconds int `yaml:"min_duration_seconds"`
	// Minimum relevance score for results (default 10.0 = at least one word match)
	// Results below this score are filtered out. Set to 0 to disable filtering.
	MinRelevanceScore float64 `yaml:"min_relevance_score"`
	// Filter out premium/gold content
	FilterPremium bool `yaml:"filter_premium"`
	// Use spoofed TLS fingerprint (Chrome) to bypass Cloudflare
	SpoofTLS        bool                  `yaml:"spoof_tls"`
	AgeVerification AgeVerificationConfig `yaml:"age_verification"`
	// Custom autocomplete terms to ADD to built-in suggestions
	CustomTerms []string `yaml:"custom_terms"`
	// AI content filter (deepfakes, AI-generated)
	AIFilter AIFilterConfig `yaml:"ai_filter"`
	// Per-engine timeout overrides in seconds (e.g., pornhub: 20)
	// Engines not listed use the global engine_timeout
	EngineTimeouts map[string]int `yaml:"engine_timeouts"`
	// EngineRequestInterval is the minimum time in milliseconds between outbound
	// requests to the same engine. Prevents triggering engine rate limits.
	// Default 0 (no throttle). Recommended: 500-2000ms.
	EngineRequestInterval int `yaml:"engine_request_interval"`
	// Per-engine request interval overrides in milliseconds.
	// Engines not listed use EngineRequestInterval.
	EngineRequestIntervals map[string]int `yaml:"engine_request_intervals"`
	// ThumbnailCacheTTL is the time-to-live for the on-disk thumbnail cache in minutes.
	// Default 1440 (24 hours). Set to 0 to disable disk caching.
	ThumbnailCacheTTL int `yaml:"thumbnail_cache_ttl"`
}

// AIFilterConfig holds settings for filtering AI-generated content
type AIFilterConfig struct {
	// Enabled: server-wide default for AI content filtering (default: true = blocked)
	Enabled bool `yaml:"enabled"`
	// AllowUserOverride: let users enable AI content via preferences (default: true)
	AllowUserOverride bool `yaml:"allow_user_override"`
	// Keywords to detect AI-generated content in titles/tags
	// Default includes: ai generated, ai porn, deepfake, etc.
	Keywords []string `yaml:"keywords"`
}

// UserAgentConfig holds user agent settings for engine requests
// Configurable to allow updating without rebuild
type UserAgentConfig struct {
	// OS: windows, macos, linux (default: windows)
	OS string `yaml:"os"`
	// Version: OS version number (default: 11 for Windows)
	Version string `yaml:"version"`
	// Browser: chrome, firefox, edge (default: chrome)
	Browser string `yaml:"browser"`
	// BrowserVersion: browser version (default: latest stable)
	BrowserVersion string `yaml:"browser_version"`
}

// String returns the formatted user agent string
// Generates Chrome/Firefox/Edge user agent based on config
func (ua UserAgentConfig) String() string {
	// Map OS to NT version
	var osString string
	switch ua.OS {
	case "windows":
		// Windows 11 = NT 10.0, Windows 10 = NT 10.0
		// Windows versions 10 and 11 both report as NT 10.0
		osString = "Windows NT 10.0; Win64; x64"
	case "macos":
		// macOS version format: 10_15_7
		version := ua.Version
		if version == "" {
			version = "14_0"
		}
		osString = "Macintosh; Intel Mac OS X " + version
	case "linux":
		osString = "X11; Linux x86_64"
	default:
		osString = "Windows NT 10.0; Win64; x64"
	}

	// Map browser to user agent format
	browserVersion := ua.BrowserVersion
	if browserVersion == "" {
		browserVersion = "131"
	}

	switch ua.Browser {
	case "firefox":
		return "Mozilla/5.0 (" + osString + "; rv:" + browserVersion + ".0) Gecko/20100101 Firefox/" + browserVersion + ".0"
	case "edge":
		return "Mozilla/5.0 (" + osString + ") AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + browserVersion + ".0.0.0 Safari/537.36 Edg/" + browserVersion + ".0.0.0"
	case "chrome":
		fallthrough
	default:
		return "Mozilla/5.0 (" + osString + ") AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + browserVersion + ".0.0.0 Safari/537.36"
	}
}

// SecChUa returns the Sec-Ch-Ua header value for Chrome/Edge
// Returns empty string for Firefox (doesn't send this header)
func (ua UserAgentConfig) SecChUa() string {
	browserVersion := ua.BrowserVersion
	if browserVersion == "" {
		browserVersion = "131"
	}

	switch ua.Browser {
	case "firefox":
		// Firefox doesn't send Sec-Ch-Ua
		return ""
	case "edge":
		return `"Microsoft Edge";v="` + browserVersion + `", "Chromium";v="` + browserVersion + `", "Not_A Brand";v="24"`
	case "chrome":
		fallthrough
	default:
		return `"Google Chrome";v="` + browserVersion + `", "Chromium";v="` + browserVersion + `", "Not_A Brand";v="24"`
	}
}

// SecChUaPlatform returns the Sec-Ch-Ua-Platform header value
func (ua UserAgentConfig) SecChUaPlatform() string {
	switch ua.OS {
	case "macos":
		return `"macOS"`
	case "linux":
		return `"Linux"`
	case "windows":
		fallthrough
	default:
		return `"Windows"`
	}
}

// IsChromiumBased returns true if the browser is Chromium-based (Chrome, Edge)
// Used to determine if Sec-Ch-* headers should be sent
func (ua UserAgentConfig) IsChromiumBased() bool {
	return ua.Browser != "firefox"
}

// AgeVerificationConfig holds age verification settings
type AgeVerificationConfig struct {
	Enabled    bool `yaml:"enabled"`
	CookieDays int  `yaml:"cookie_days"`
}

// AppPaths holds resolved directory paths
// AppPaths is now defined in paths package
type AppPaths = paths.AppPaths

// DefaultAppConfig returns an AppConfig with sensible defaults per AI.md
func DefaultAppConfig() *AppConfig {
	fqdn := getHostname()
	// Per AI.md PART 5: Default port is random 64xxx (non-privileged, no root required)
	defaultPort := fmt.Sprintf("%d", findUnusedPort())

	return &AppConfig{
		Server: ServerConfig{
			Port:    defaultPort,
			FQDN:    fqdn,
			Address: "[::]",
			Mode:    "production",
			Branding: ServerBrandingConfig{
				Title:       "Vidveil",
				Tagline:     "Privacy-first video search",
				Description: "Privacy-respecting adult video search",
			},
			User:    "",
			Group:   "",
			PIDFile: true,
			Admin: AdminConfig{
				Path:     "admin",
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
				Timezone:      "America/New_York",
				CatchUpWindow: "1h",
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
				Enabled:         false,
				Endpoint:        "/metrics",
				IncludeSystem:   true,
				IncludeRuntime:  true,
				DurationBuckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				SizeBuckets:     []float64{100, 1000, 10000, 100000, 1000000, 10000000},
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
				Requests: 500,
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
				Enabled:        true,
				Dir:            "",
				Update:         "weekly",
				DenyCountries:  []string{},
				AllowCountries: []string{},
				Databases: GeoIPDatabasesConfig{
					ASN:     true,
					Country: true,
					// Need city for region-level restriction
					City: true,
				},
				ContentRestriction: ContentRestrictionConfig{
					Mode:      "warn",
					BypassTor: true,
					// Default restricted regions based on adult content laws
					// US states with age verification laws
					RestrictedRegions: []string{
						"US:Texas", "US:Utah", "US:Louisiana", "US:Arkansas",
						"US:Montana", "US:Mississippi", "US:Virginia", "US:North Carolina",
					},
					// Countries with strict adult content bans
					RestrictedCountries: []string{},
					WarningMessage:      "Adult content may be restricted or require age verification in your region.",
				},
			},
			// Backup settings per AI.md PART 21
			// Default per PART 21: 1 daily backup, weekly/monthly/yearly disabled
			Backup: BackupConfig{
				Retention: BackupRetentionConfig{
					MaxBackups:  1,
					KeepWeekly:  0,
					KeepMonthly: 0,
					KeepYearly:  0,
				},
				Encryption: BackupEncryptionConfig{
					// Not encrypted by default
					Enabled: false,
				},
			},
			// Tor settings per AI.md PART 31
			// Hidden service auto-enabled if tor binary found
			// Outbound network disabled by default - can be enabled for privacy
			Tor: DefaultTorConfig(),
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
				Deny:  []string{"/server/admin", "/api/v1/server/admin"},
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
			// Default minimum duration: 10 minutes (600 seconds)
			MinDurationSeconds: 600,
			// Default minimum relevance: 10.0 ensures at least one query word matches
			MinRelevanceScore: 10.0,
			FilterPremium:     true,
			// Disabled by default - can cause issues with some engines
			// Enable only for Cloudflare-protected sites
			SpoofTLS: false,
			AgeVerification: AgeVerificationConfig{
				Enabled:    true,
				CookieDays: 30,
			},
			// AI content filter - block deepfakes/AI-generated by default
			AIFilter: AIFilterConfig{
				Enabled:           true,
				AllowUserOverride: true,
				Keywords: []string{
					"ai generated", "ai-generated", "ai porn", "ai-porn",
					"deepfake", "deep fake", "deep-fake",
					"ai model", "ai celebrity", "ai fake",
					"generated porn", "synthetic", "artificially generated",
					"neural network", "machine learning porn",
					"fake celebrity", "celebrity deepfake",
				},
			},
			// Thumbnail disk cache TTL: 24 hours by default
			ThumbnailCacheTTL: 1440,
		},
		Engines: EnginesConfig{
			UserAgent: UserAgentConfig{
				OS:             "windows",
				Version:        "11",
				Browser:        "chrome",
				BrowserVersion: "131",
			},
		},
	}
}

// GetAppPaths returns OS-appropriate paths (delegated to paths package)
func GetAppPaths(configDir, dataDir string) *AppPaths {
	return paths.GetAppPaths(configDir, dataDir)
}

// GetDatabaseDir returns the SQLite database directory.
func GetDatabaseDir(dataDir string) string {
	return paths.GetDatabaseDir(dataDir)
}

// LoadAppConfig loads configuration from file or creates default
func LoadAppConfig(configDir, dataDir string) (*AppConfig, string, error) {
	paths := GetAppPaths(configDir, dataDir)
	dbDir := GetDatabaseDir(paths.Data)

	// Ensure directories exist per AI.md PART 8 and PART 23
	// Binary handles ALL directory creation with proper permissions
	// Permissions: root=0755, user=0700 per AI.md PART 4
	dirPerm := os.FileMode(0755)
	if os.Getuid() != 0 {
		dirPerm = 0700
	}
	for _, dir := range []string{paths.Config, paths.Data, paths.Cache, paths.Log, dbDir} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return nil, "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	configPath := filepath.Join(paths.Config, "server.yml")

	// Check for .yaml migration
	yamlPath := filepath.Join(paths.Config, "server.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if renameErr := os.Rename(yamlPath, configPath); renameErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to migrate server.yaml to server.yml: %v\n", renameErr)
			} else {
				fmt.Printf("Migrated server.yaml to server.yml\n")
			}
		}
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		cfg := DefaultAppConfig()

		// Set paths in config
		cfg.Server.SSL.CertPath = filepath.Join(paths.Config, "ssl", "certs")
		cfg.Server.Database.SQLite.Dir = dbDir

		if err := SaveAppConfig(cfg, configPath); err != nil {
			return nil, "", fmt.Errorf("failed to save default config: %w", err)
		}

		// Console output is handled in main.go per AI.md PART 7

		// Apply VIDVEIL_* env var overrides per AI.md (env overrides config file)
		ApplyEnvOverrides(cfg)

		return cfg, configPath, nil
	}

	// Load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read config: %w", err)
	}

	// Start with defaults; unknown YAML keys are errors per AI.md PART 5
	cfg := DefaultAppConfig()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, "", fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate and fix invalid config values per AI.md PART 12
	validateConfig(cfg)

	if cfg.Server.Database.SQLite.Dir == "" || os.Getenv("DATABASE_DIR") != "" {
		cfg.Server.Database.SQLite.Dir = dbDir
	}

	// Apply VIDVEIL_* env var overrides per AI.md (env overrides config file)
	ApplyEnvOverrides(cfg)

	return cfg, configPath, nil
}

// validateConfig validates all config values, replacing invalid with defaults per AI.md PART 12
// Rule: If config setting is invalid, warn and replace with default. Never fail startup.
func validateConfig(cfg *AppConfig) {
	defaults := DefaultAppConfig()

	// Validate port (must be valid or use random per PART 8/12)
	if cfg.Server.Port != "" {
		// Parse port(s) - could be "8080" or "8080,8443"
		ports := strings.Split(cfg.Server.Port, ",")
		for _, p := range ports {
			port := strings.TrimSpace(p)
			if port == "" {
				continue
			}
			portNum := 0
			fmt.Sscanf(port, "%d", &portNum)
			if portNum < 1 || portNum > 65535 {
				randomPort := findUnusedPort()
				fmt.Fprintf(os.Stderr, "Warning: invalid port %s, using random port %d\n", port, randomPort)
				cfg.Server.Port = fmt.Sprintf("%d", randomPort)
				break
			}
		}
	}

	// Validate mode (must be production or development)
	if cfg.Server.Mode != "" && cfg.Server.Mode != "production" && cfg.Server.Mode != "development" {
		fmt.Fprintf(os.Stderr, "Warning: invalid mode %q, using default %q\n", cfg.Server.Mode, defaults.Server.Mode)
		cfg.Server.Mode = defaults.Server.Mode
	}

	// Validate rate limit window (must be positive)
	if cfg.Server.RateLimit.Window < 0 {
		fmt.Fprintf(os.Stderr, "Warning: invalid rate_limit.window %d, using default 60\n", cfg.Server.RateLimit.Window)
		cfg.Server.RateLimit.Window = 60
	}

	// Validate rate limit requests (must be positive)
	if cfg.Server.RateLimit.Requests < 0 {
		fmt.Fprintf(os.Stderr, "Warning: invalid rate_limit.requests %d, using default 500\n", cfg.Server.RateLimit.Requests)
		cfg.Server.RateLimit.Requests = 500
	}

	// Validate SSL settings
	if cfg.Server.SSL.Enabled && cfg.Server.SSL.LetsEncrypt.Enabled {
		if cfg.Server.SSL.LetsEncrypt.Email == "" {
			fmt.Fprintf(os.Stderr, "Warning: SSL Let's Encrypt enabled but no email configured\n")
		}
	}

	// Validate session same_site (must be strict, lax, or none)
	sameSite := strings.ToLower(cfg.Server.Session.SameSite)
	if sameSite != "" && sameSite != "strict" && sameSite != "lax" && sameSite != "none" {
		fmt.Fprintf(os.Stderr, "Warning: invalid session.same_site %q, using default 'lax'\n", cfg.Server.Session.SameSite)
		cfg.Server.Session.SameSite = "lax"
	}

	// Validate compression level (1-9)
	if cfg.Server.Compression.Level < 0 || cfg.Server.Compression.Level > 9 {
		fmt.Fprintf(os.Stderr, "Warning: invalid compression.level %d, using default 5\n", cfg.Server.Compression.Level)
		cfg.Server.Compression.Level = 5
	}

	// Enforce audit log format as JSON only per AI.md PART 11
	// "audit: format: json only (text not supported for audit - must be machine-parseable)"
	if cfg.Server.Logs.Audit.Format != "" && cfg.Server.Logs.Audit.Format != "json" {
		fmt.Fprintf(os.Stderr, "Warning: audit log format must be 'json', ignoring %q\n", cfg.Server.Logs.Audit.Format)
		cfg.Server.Logs.Audit.Format = "json"
	}
}

// SaveAppConfig saves configuration to file
func SaveAppConfig(cfg *AppConfig, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comment
	header := `# =============================================================================
# Vidveil Configuration
# =============================================================================
# This file follows the apimgr AI.md specification
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

// ParseBoolEnv parses a boolean value from an environment variable
// Uses the full truthy/falsy value set from bool.go per AI.md PART 5
func ParseBoolEnv(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	result, err := ParseBool(val, defaultVal)
	if err != nil {
		return defaultVal
	}
	return result
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return hostname
}

func findUnusedPort() int {
	// Spec (AI.md PART 5): random unused port in 64000-64999, never sequential
	const portMin = 64000
	const portRange = 1000
	var startOffset int
	b := make([]byte, 2)
	if _, err := rand.Read(b); err == nil {
		startOffset = (int(b[0])<<8 | int(b[1])) % portRange
	}
	for i := 0; i < portRange; i++ {
		port := portMin + (startOffset+i)%portRange
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	return 64080
}

func generateToken(length int) string {
	bytes := make([]byte, length)
	// A CSPRNG failure must never silently produce a predictable token
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("config: failed to read from crypto/rand: %v", err))
	}
	return hex.EncodeToString(bytes)
}

// IsRunningInContainer detects if running in a container (tini as PID 1)
func IsRunningInContainer() bool {
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
func (c *AppConfig) IsDevelopmentMode() bool {
	mode := strings.ToLower(c.Server.Mode)
	return mode == "development" || mode == "dev"
}

// IsProductionMode returns true if running in production mode
func (c *AppConfig) IsProductionMode() bool {
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

// AI.md PART 8: URL/FQDN Detection

// devOnlyTLDs are TLDs allowed only in development mode per AI.md
var devOnlyTLDs = []string{
	".localhost", ".test", ".example", ".invalid",
	".local", ".lan", ".internal", ".home", ".localdomain",
	".home.arpa", ".intranet", ".corp", ".private",
}

// IsValidHost validates a host per AI.md PART 8
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

// IsValidSSLHost validates host for SSL/Let's Encrypt per AI.md
// SSL always requires production-valid host (devMode=false)
func IsValidSSLHost(host string) bool {
	return IsValidHost(host, false)
}

// LiveReload per AI.md PART 8 NON-NEGOTIABLE
// Watches config file and reloads on changes

// ReloadCallback is called when configuration is reloaded
type ReloadCallback func(*AppConfig)

// ConfigWatcher watches for config file changes
type ConfigWatcher struct {
	configPath string
	appConfig  *AppConfig
	callbacks  []ReloadCallback
	stopChan   chan struct{}
	lastMod    int64
}

// NewWatcher creates a new config watcher
func NewWatcher(configPath string, appConfig *AppConfig) *ConfigWatcher {
	info, _ := os.Stat(configPath)
	var lastMod int64
	if info != nil {
		lastMod = info.ModTime().UnixNano()
	}

	return &ConfigWatcher{
		configPath: configPath,
		appConfig:  appConfig,
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

	// Unknown YAML keys are errors per AI.md PART 5
	newCfg := DefaultAppConfig()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(newCfg); err != nil {
		fmt.Printf("⚠️  Failed to parse config for reload: %v\n", err)
		return
	}

	// Update the shared config — all settings that can live-reload without restart.
	// Port and Address changes are intentionally excluded: they require a listener
	// rebind and must log a pending-restart notice instead.
	var restartReasons []string
	if newCfg.Server.Port != w.appConfig.Server.Port {
		restartReasons = append(restartReasons, "server.port")
	}
	if newCfg.Server.Address != w.appConfig.Server.Address {
		restartReasons = append(restartReasons, "server.address")
	}
	pendingRestart := len(restartReasons) > 0

	w.appConfig.Server.Branding = newCfg.Server.Branding
	w.appConfig.Server.RateLimit = newCfg.Server.RateLimit
	w.appConfig.Server.Email = newCfg.Server.Email
	w.appConfig.Server.Notifications = newCfg.Server.Notifications
	w.appConfig.Server.Schedule = newCfg.Server.Schedule
	w.appConfig.Server.SSL = newCfg.Server.SSL
	w.appConfig.Server.Metrics = newCfg.Server.Metrics
	w.appConfig.Server.Logs = newCfg.Server.Logs
	w.appConfig.Server.GeoIP = newCfg.Server.GeoIP
	w.appConfig.Server.Admin = newCfg.Server.Admin
	w.appConfig.Server.Session = newCfg.Server.Session
	w.appConfig.Server.SecurityHeaders = newCfg.Server.SecurityHeaders
	w.appConfig.Server.Compression = newCfg.Server.Compression
	w.appConfig.Server.Limits = newCfg.Server.Limits
	w.appConfig.Server.TrustedProxies = newCfg.Server.TrustedProxies
	w.appConfig.Server.Cache = newCfg.Server.Cache
	w.appConfig.Server.Security = newCfg.Server.Security
	w.appConfig.Server.Backup = newCfg.Server.Backup
	w.appConfig.Server.Tor = newCfg.Server.Tor
	w.appConfig.Server.Healthz = newCfg.Server.Healthz
	w.appConfig.Server.FQDN = newCfg.Server.FQDN
	w.appConfig.Server.Mode = newCfg.Server.Mode
	w.appConfig.Web = newCfg.Web
	w.appConfig.Search = newCfg.Search

	w.appConfig.PendingRestart = pendingRestart
	w.appConfig.RestartReasons = restartReasons

	if pendingRestart {
		fmt.Printf("⚠️  Port/address change detected (%s) — restart required for network changes to take effect\n",
			strings.Join(restartReasons, ", "))
	}
	fmt.Printf("🔄 Configuration reloaded\n")

	// Notify callbacks
	for _, callback := range w.callbacks {
		callback(w.appConfig)
	}
}

// Reload forces a configuration reload
func (w *ConfigWatcher) Reload() error {
	w.reload()
	return nil
}

// GetDisplayHost returns the appropriate host for display per AI.md PART 8
// Never shows: 0.0.0.0, 127.0.0.1, localhost, [::]
// Uses global IP if dev TLD or localhost detected
func GetDisplayHost(_ *AppConfig) string {
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

// GetFQDN returns the FQDN per AI.md PART 8
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

// isLoopback checks if host is a loopback address per AI.md PART 8
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

// isDevTLD checks if FQDN is a dev TLD per AI.md PART 8
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
		"." + paths.ProjectName,
	}
	for _, suffix := range devSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// getGlobalIPv6 returns first global unicast IPv6 address per AI.md PART 8
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

// getGlobalIPv4 returns first global unicast IPv4 address per AI.md PART 8
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

// AdminURLPrefix returns the spec-canonical admin route prefix per AI.md PART 14/17.
// Form: "/server/{admin_path}". The legacy form (without "/server/") is no longer accepted.
func (c *AppConfig) AdminURLPrefix() string {
	adminPath := c.Server.Admin.Path
	if adminPath == "" {
		adminPath = "admin"
	}
	return "/server/" + adminPath
}

// AdminAPIPrefix returns the canonical admin API prefix without the "/api/{ver}" leader.
// Used as a relative subpath under "/api/v1": result is "/server/{admin_path}".
func (c *AppConfig) AdminAPIPrefix() string {
	adminPath := c.Server.Admin.Path
	if adminPath == "" {
		adminPath = "admin"
	}
	return "/server/" + adminPath
}

// GetPublicURL returns the public-facing URL for this server
// Used by /api/autodiscover endpoint per AI.md PART 14
func (c *AppConfig) GetPublicURL() string {
	// Use FQDN if configured
	if c.Server.FQDN != "" {
		return fmt.Sprintf("https://%s", c.Server.FQDN)
	}

	// Otherwise, build from address and port
	scheme := "http"
	host := c.Server.Address
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}

	// Port is a string, parse it
	port := c.Server.Port

	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

// validateSEOVerification validates SEO verification codes per AI.md PART 16.
// Returns a list of fields with invalid values; empty = all OK.
// Invalid codes are logged but NOT rejected (server continues with them skipped).
func validateSEOVerification(v SEOVerificationConfig) []string {
	var bad []string
	if v.Google != "" && !seoVerifyPattern(`^[a-zA-Z0-9_-]{1,43}$`, v.Google) {
		bad = append(bad, "seo.verification.google")
	}
	if v.Bing != "" && !seoVerifyPattern(`^[A-F0-9]{1,32}$`, v.Bing) {
		bad = append(bad, "seo.verification.bing")
	}
	if v.Yandex != "" && !seoVerifyPattern(`^[a-f0-9]{1,32}$`, v.Yandex) {
		bad = append(bad, "seo.verification.yandex")
	}
	if v.Baidu != "" && !seoVerifyPattern(`^[a-zA-Z0-9]{1,32}$`, v.Baidu) {
		bad = append(bad, "seo.verification.baidu")
	}
	if v.Pinterest != "" && !seoVerifyPattern(`^[a-f0-9]{1,32}$`, v.Pinterest) {
		bad = append(bad, "seo.verification.pinterest")
	}
	if v.Facebook != "" && !seoVerifyPattern(`^[a-z0-9]{1,64}$`, v.Facebook) {
		bad = append(bad, "seo.verification.facebook")
	}
	for i, ct := range v.Custom {
		key := fmt.Sprintf("seo.verification.custom[%d]", i)
		if ct.Name == "" && ct.Property == "" {
			bad = append(bad, key+".name_or_property")
			continue
		}
		nameOrProp := ct.Name
		if nameOrProp == "" {
			nameOrProp = ct.Property
		}
		if !seoVerifyPattern(`^[a-zA-Z0-9_:-]{1,64}$`, nameOrProp) {
			bad = append(bad, key+".name_or_property")
		}
		if ct.Content == "" || len(ct.Content) > 256 {
			bad = append(bad, key+".content")
		}
	}
	return bad
}

func seoVerifyPattern(pattern, value string) bool {
	matched, err := regexp.MatchString(pattern, value)
	return err == nil && matched
}
