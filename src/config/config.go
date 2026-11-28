// SPDX-License-Identifier: MIT
package config

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	paths "github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server/service/system"
	"golang.org/x/net/publicsuffix"
	"gopkg.in/yaml.v3"
)

// paths.ProjectOrg and paths.ProjectName are defined in paths package

// apiVersionPattern validates Server.APIVersion per AI.md PART 14 ({api_version}
// examples: "v1", "v2"). Lowercase "v" followed by one or more digits.
var apiVersionPattern = regexp.MustCompile(`^v[0-9]+$`)
var seoMetaNamePattern = regexp.MustCompile(`^[a-z0-9:_.-]+$`)
var seoOpenGraphTypePattern = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)

// Config holds all application configuration per AI.md spec
type ServerConfig struct {
	Server  ServerSettings `yaml:"server"`
	CLI     CLIConfig      `yaml:"cli"`
	Output  OutputConfig   `yaml:"output"`
	Web     WebConfig      `yaml:"web"`
	Search  SearchConfig   `yaml:"search"`
	Engines EnginesConfig  `yaml:"engines"`

	// Runtime-only state (never serialised to YAML)
	// Set by ConfigWatcher when port/address changes require a restart.
	PendingRestart       bool              `yaml:"-" json:"-"`
	RestartReasons       []string          `yaml:"-" json:"-"`
	GeneratedSecretKeys  []string          `yaml:"-" json:"-"`
	GeneratedServerToken bool              `yaml:"-" json:"-"`
	LegacyAppSecrets     map[string]string `yaml:"-" json:"-"`
	Runtime              RuntimeConfig     `yaml:"-" json:"-"`
	Version              string            `yaml:"-" json:"-"`
	CommitID             string            `yaml:"-" json:"-"`
	BuildEpoch           string            `yaml:"-" json:"-"`
	OfficialSite         string            `yaml:"-" json:"-"`
}

// OutputConfig controls server console color and emoji output. Pointers retain
// whether each value was explicitly set so environment defaults remain usable.
type OutputConfig struct {
	Color *bool `yaml:"color"`
	Emoji *bool `yaml:"emoji"`
}

type AppConfig = ServerConfig

type RuntimeConfig struct {
	ConfigPath    string
	ConfigDir     string
	DataDir       string
	CacheDir      string
	LogDir        string
	BackupDir     string
	PIDFile       string
	Address       string
	Port          string
	BaseURL       string
	Mode          string
	Color         string
	Language      string
	Debug         bool
	DebugSet      bool
	I2P           bool
	I2PSet        bool
	Daemon        bool
	PIDFileSet    bool
	PeekDaemonize bool
	Arguments     []string
	Command       []string
	Help          bool
	Version       bool
	Listener      net.Listener
	Ready         chan struct{}
	TestMode      bool
}

// CLIConfig holds server-side companion CLI download settings per AI.md PART 32.
type CLIConfig struct {
	BinaryDownload CLIBinaryDownloadConfig `yaml:"binary_download"`
}

// CLIBinaryDownloadConfig controls whether the server token is required when
// streaming companion CLI release binaries.
type CLIBinaryDownloadConfig struct {
	Dir         string `yaml:"dir"`
	RequireAuth bool   `yaml:"require_auth"`
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
	// Logo is a local file path or remote URL for the header logo; empty = embedded default.
	Logo string `yaml:"logo"`
	// Favicon is a local file path or remote URL for the favicon; empty = embedded default.
	Favicon string `yaml:"favicon"`
	// RefreshInterval controls the internal scheduler refresh interval for remote images.
	// Defaults to daily; local image sources are also reprocessed at this interval.
	RefreshInterval string                `yaml:"refresh_interval"`
	Content         BrandingContentConfig `yaml:"content"`
}

type BrandingContentConfig struct {
	Header  BrandingPageContent `yaml:"header"`
	About   BrandingPageContent `yaml:"about"`
	Privacy BrandingPageContent `yaml:"privacy"`
	Footer  BrandingPageContent `yaml:"footer"`
	Terms   BrandingPageContent `yaml:"terms"`
}

type BrandingPageContent struct {
	Format string               `yaml:"format"`
	Body   LocalizedContentBody `yaml:"body"`
}

type LocalizedContentBody struct {
	Text   string            `yaml:"-"`
	Values map[string]string `yaml:"-"`
	Order  []string          `yaml:"-"`
}

func (body *LocalizedContentBody) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		body.Text = node.Value
		body.Values = nil
		body.Order = nil
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("content body must be text or a language map")
	}
	values := make(map[string]string, len(node.Content)/2)
	order := make([]string, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i+1].Kind != yaml.ScalarNode {
			return fmt.Errorf("content body language values must be text")
		}
		values[node.Content[i].Value] = node.Content[i+1].Value
		order = append(order, node.Content[i].Value)
	}
	body.Text = ""
	body.Values = values
	body.Order = order
	return nil
}

func (body LocalizedContentBody) MarshalYAML() (any, error) {
	if len(body.Values) == 0 {
		return body.Text, nil
	}
	order := append([]string(nil), body.Order...)
	if len(order) == 0 {
		for key := range body.Values {
			order = append(order, key)
		}
		sort.Strings(order)
	}
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, key := range order {
		value, ok := body.Values[key]
		if !ok {
			continue
		}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
		)
	}
	return node, nil
}

func (body LocalizedContentBody) ResolveWithFallback(language, fallback string) string {
	if len(body.Values) == 0 {
		if body.Text != "" {
			return body.Text
		}
		return fallback
	}
	if value := body.Values[language]; value != "" {
		return value
	}
	if value := body.Values["default"]; value != "" {
		return value
	}
	return fallback
}

func (body LocalizedContentBody) Resolve(language string) string {
	if len(body.Values) == 0 {
		return body.Text
	}
	if value := body.Values[language]; value != "" {
		return value
	}
	if value := body.Values[strings.SplitN(language, "-", 2)[0]]; value != "" {
		return value
	}
	return body.Values["default"]
}

// ServerSettings holds the server section of the aggregate runtime configuration.
type ServerSettings struct {
	// Port: single (HTTP) or dual (HTTP,HTTPS) e.g., "8090" or "8090,64453"
	Port    string `yaml:"port"`
	FQDN    string `yaml:"fqdn"`
	Address string `yaml:"address"`
	// BaseURL is the URL path prefix per AI.md PART 12.
	// Priority: X-Forwarded-Prefix > X-Forwarded-Path > X-Script-Name > this value > "/"
	// CLI flag: --baseurl PATH; env var: BASEURL
	BaseURL string `yaml:"baseurl"`

	// APIVersion is the versioned API path segment per AI.md PART 14
	// ("Always version ALL API routes: /api/{api_version}/..."). Defaults to "v1".
	// Access via AppConfig.APIBasePath() ("/api/{version}") - never hardcode "/api/v1".
	APIVersion string `yaml:"api_version"`

	// Application mode: production or development
	// Can be overridden by MODE env var or --mode CLI flag
	Mode string `yaml:"mode"`

	// Pages holds configurable Markdown appended to server help pages per AI.md PART 16.
	Pages ServerPagesConfig `yaml:"pages"`

	// Tokens controls anonymous owner-token lifecycle per AI.md PART 11.
	Tokens TokensConfig `yaml:"tokens"`

	// Token is the global operator token per AI.md PART 11 ("server.token").
	// Auto-generated and written to secrets.yml on first run if absent or empty.
	// Grants full access to every resource and every sensitive server management
	// operation (PART 5 "Sensitive Operations": restore, mode change, PGP key ops).
	// Validated by SHA-256-hashing the inbound value and comparing against
	// SHA-256(server.token) with crypto/subtle.ConstantTimeCompare - never with ==.
	Token string `yaml:"token"`

	// Application branding per AI.md PART 16
	Branding ServerBrandingConfig `yaml:"branding"`

	// Daemonize is honored only for a manual start. Service managers and
	// containers keep the server in the foreground.
	Daemonize bool `yaml:"daemonize"`

	// PID file
	PIDFile bool `yaml:"pidfile"`

	// Contact routing: admin/security/abuse/general roles with email + webhooks
	Contact ContactConfig `yaml:"contact"`

	// Notifications (email SMTP lives here per AI.md PART 17)
	Notifications NotificationsConfig `yaml:"notifications"`

	// Scheduler
	Schedule ScheduleConfig `yaml:"scheduler"`

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

	// CORS controls browser cross-origin access per AI.md PART 16.
	CORS CORSConfig `yaml:"cors"`

	// Session
	Session SessionConfig `yaml:"session"`

	// Database
	Database DatabaseConfig `yaml:"database"`

	// Cache (optional; default memory) per AI.md PART 12
	Cache CacheConfig `yaml:"cache"`

	// GeoIP
	GeoIP GeoIPConfig `yaml:"geoip"`

	// Security (PART 11) - Blocklists, CVE, etc
	Security SecurityConfig `yaml:"security"`

	// Backup (PART 21) - Backup & Restore settings
	Backup BackupConfig `yaml:"backup"`
	// Maintenance controls read-only recovery and disk-pressure cleanup per PART 13.
	Maintenance MaintenanceConfig `yaml:"maintenance"`
	// Restore archive resource limits per AI.md PART 21.
	Restore RestoreConfig `yaml:"restore"`

	// Compliance holds regulatory-standard toggles per AI.md "Compliance Standards".
	// All disabled by default; enable individually as needed.
	Compliance ComplianceConfig `yaml:"compliance"`

	// Tor (PART 31) - Hidden service and outbound network settings
	Tor TorConfig `yaml:"tor"`

	// I2P (PART 31.2) - eepsite provider settings; default depends on runtime.
	I2P I2PConfig `yaml:"i2p"`

	// Healthz (PART 13) - Optional root-level alias for /server/healthz
	// Canonical route is /server/healthz; root /healthz is opt-in
	Healthz HealthzConfig `yaml:"healthz"`

	// SEO holds SEO and social metadata settings per AI.md PART 16
	SEO SEOConfig `yaml:"seo"`

	// Update holds release-channel and auto-install settings per AI.md PART 22
	Update UpdateConfig `yaml:"update"`

	// Tracking configures the optional analytics provider per AI.md PART 12.
	Tracking TrackingConfig `yaml:"tracking"`

	// Privacy holds the cookie-consent banner text and data-sale disclosure
	// per AI.md PART 12 "Cookie Consent Banner" (server.privacy.consent.*,
	// server.privacy.data.sold)
	Privacy PrivacyConfig `yaml:"privacy"`
}

type ServerPagesConfig struct {
	About   ServerPageContentConfig  `yaml:"about"`
	Privacy ServerPageContentConfig  `yaml:"privacy"`
	Contact ServerContactPagesConfig `yaml:"contact"`
	Help    ServerHelpPagesConfig    `yaml:"help"`
	Terms   ServerPageContentConfig  `yaml:"terms"`
}

type ServerPageContentConfig struct {
	Content string `yaml:"content"`
}

type ServerContactPagesConfig struct {
	Enabled          bool                  `yaml:"enabled"`
	Captcha          string                `yaml:"captcha"`
	CaptchaSiteKey   string                `yaml:"captcha_site_key"`
	CaptchaSecretKey string                `yaml:"captcha_secret_key"`
	SuccessMessage   string                `yaml:"success_message"`
	Antispam         ContactAntispamConfig `yaml:"antispam"`
}

type ContactAntispamConfig struct {
	Honeypot             bool     `yaml:"honeypot"`
	MinFillSeconds       int      `yaml:"min_fill_seconds"`
	MaxAgeSeconds        int      `yaml:"max_age_seconds"`
	PerIP                int      `yaml:"per_ip"`
	PerEmail             int      `yaml:"per_email"`
	Global               int      `yaml:"global"`
	Window               string   `yaml:"window"`
	MaxLinks             int      `yaml:"max_links"`
	ReviewMinSeconds     int      `yaml:"review_min_seconds"`
	ReviewMaxSeconds     int      `yaml:"review_max_seconds"`
	RejectMarkup         bool     `yaml:"reject_markup"`
	BlockedWords         []string `yaml:"blocked_words"`
	BlockedEmailDomains  []string `yaml:"blocked_email_domains"`
	Strikes              int      `yaml:"strikes"`
	BanHours             int      `yaml:"ban_hours"`
	DuplicateWindowHours int      `yaml:"duplicate_window_hours"`
}

type ServerHelpPagesConfig struct {
	Content LocalizedContentBody `yaml:"content"`
	MCP     string               `yaml:"mcp"`
}

// DefaultHelpContent is the built-in fallback for server help localization.
const DefaultHelpContent = "VidVeil searches across configured video engines. Search with a phrase, use bang shortcuts such as `!ph`, and manage saved engines and display options under `/server/preferences`."

const vidveilMCPHelp = "VidVeil searches across 27 registered video engines by default and streams results as they respond. Use `searchVideos` for a phrase, optionally with a bang such as `!ph` to target Pornhub; use `listEngines` to inspect engine availability, `listBangs` to discover shortcuts, and `autocompleteBangs` to find matching shortcuts or suggestions. For example: search for ocean sunsets with `!ph`, then compare the returned titles and source engines. Favorites are private owner resources; use a linked token for automation and never share a primary or operator token."

// TokensConfig contains limits and retention for anonymous owner tokens.
type TokensConfig struct {
	MaxLinked     int    `yaml:"max_linked"`
	CookieMaxAge  string `yaml:"cookie_max_age"`
	IdlePurgeDays int    `yaml:"idle_purge_days"`
}

// TrackingConfig controls the optional analytics provider per AI.md PART 12.
type TrackingConfig struct {
	Type string `yaml:"type"`
	ID   string `yaml:"id"`
	URL  string `yaml:"url"`
}

// PrivacyConfig holds cookie-consent and data-sale disclosure settings per
// AI.md PART 12 "Cookie Consent Banner → Implementation" (server.privacy.*)
type PrivacyConfig struct {
	Consent    ConsentConfig          `yaml:"consent"`
	Data       PrivacyDataConfig      `yaml:"data"`
	Retention  PrivacyRetentionConfig `yaml:"retention"`
	Cookies    CookieCategoriesConfig `yaml:"cookies"`
	ThirdParty ThirdPartyConfig       `yaml:"third_party"`
	Content    PrivacyContentConfig   `yaml:"-"`
}

// PrivacyDataConfig holds data-handling disclosures per AI.md PART 12/16.
// Sold gates the CCPA "Do Not Sell" opt-out section and selects the
// cookie-consent banner message (MessageIfSold vs Message).
type PrivacyDataConfig struct {
	Sold bool `yaml:"sold"`
	// StoredOnServer records whether user data stays on this server rather
	// than a third-party cloud; surfaced in the privacy page summary.
	StoredOnServer bool `yaml:"stored_on_server"`
	// Sharing lists the conditions under which data may reach a third party.
	Sharing []SharingCondition `yaml:"sharing"`
}

// SharingCondition describes one situation where data may be shared
// (AI.md PART 12 "Privacy & Consent" → data.sharing)
type SharingCondition struct {
	Condition string `yaml:"condition"`
	When      string `yaml:"when"`
	Data      string `yaml:"data"`
}

// PrivacyRetentionConfig holds the data-retention disclosure and the
// export/deletion rights advertised on the privacy page (AI.md PART 12)
type PrivacyRetentionConfig struct {
	Period            string `yaml:"period"`
	ExportAvailable   bool   `yaml:"export_available"`
	DeletionAvailable bool   `yaml:"deletion_available"`
}

// CookieCategoriesConfig holds the three consent categories rendered in the
// privacy page Cookie Policy section (AI.md PART 12)
type CookieCategoriesConfig struct {
	Essential   CookieCategoryConfig  `yaml:"essential"`
	Preferences CookieCategoryConfig  `yaml:"preferences"`
	Analytics   AnalyticsCookieConfig `yaml:"analytics"`
}

// CookieCategoryConfig holds one cookie category's state and description
type CookieCategoryConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Description string `yaml:"description"`
}

// AnalyticsCookieConfig extends a cookie category with the two data-sale
// dependent description suffixes selected by GetAnalyticsDescription
type AnalyticsCookieConfig struct {
	Enabled                  bool   `yaml:"enabled"`
	Description              string `yaml:"description"`
	DescriptionSuffixNotSold string `yaml:"description_suffix_not_sold"`
	DescriptionSuffixSold    string `yaml:"description_suffix_sold"`
}

// ThirdPartyConfig lists third-party services that receive user data
type ThirdPartyConfig struct {
	Services []ThirdPartyService `yaml:"services"`
}

// ThirdPartyService describes one third-party recipient of user data
type ThirdPartyService struct {
	Name      string `yaml:"name"`
	Purpose   string `yaml:"purpose"`
	DataSent  string `yaml:"data_sent"`
	PolicyURL string `yaml:"policy_url"`
}

// PrivacyContentConfig holds built-in project text for the privacy page.
type PrivacyContentConfig struct {
	DataCollection  string `yaml:"-"`
	DataUsage       string `yaml:"-"`
	DataUsageIfSold string `yaml:"-"`
	DataSecurity    string `yaml:"-"`
}

// ConsentConfig holds the cookie-consent banner text per AI.md PART 12
// "Cookie Consent Banner → Implementation" table (server.privacy.consent.*)
type ConsentConfig struct {
	Message         string               `yaml:"message"`
	MessageIfSold   string               `yaml:"message_if_sold"`
	Policy          ConsentPolicyConfig  `yaml:"policy"`
	Buttons         ConsentButtonsConfig `yaml:"buttons"`
	PreferencesText string               `yaml:"preferences_text"`
}

// ConsentPolicyConfig holds the privacy-policy link shown in the consent banner
type ConsentPolicyConfig struct {
	URL  string `yaml:"url"`
	Text string `yaml:"text"`
}

// ConsentButtonsConfig holds the accept/decline button labels for the consent banner
type ConsentButtonsConfig struct {
	Decline string `yaml:"decline"`
	Accept  string `yaml:"accept"`
}

// GetConsentMessage returns the cookie-consent banner message, selecting
// Consent.MessageIfSold when Data.Sold is true, otherwise Consent.Message,
// per AI.md PART 12 "Cookie Consent Banner → Dynamic Message Selection".
func (p PrivacyConfig) GetConsentMessage() string {
	if p.Data.Sold && p.Consent.MessageIfSold != "" {
		return p.Consent.MessageIfSold
	}
	return p.Consent.Message
}

// GetAnalyticsDescription returns the analytics cookie description with the
// data-sale suffix appended, selecting the sold suffix when Data.Sold is true,
// per AI.md PART 12 "Dynamic Privacy Messaging".
func (p PrivacyConfig) GetAnalyticsDescription() string {
	suffix := p.Cookies.Analytics.DescriptionSuffixNotSold
	if p.Data.Sold {
		suffix = p.Cookies.Analytics.DescriptionSuffixSold
	}
	if suffix == "" {
		return p.Cookies.Analytics.Description
	}
	if p.Cookies.Analytics.Description == "" {
		return suffix
	}
	return p.Cookies.Analytics.Description + " " + suffix
}

// GetDataUsageContent returns the data-usage Markdown body, selecting
// Content.DataUsageIfSold when Data.Sold is true, per AI.md PART 12
// "Dynamic Privacy Messaging".
func (p PrivacyConfig) GetDataUsageContent() string {
	if p.Data.Sold && p.Content.DataUsageIfSold != "" {
		return p.Content.DataUsageIfSold
	}
	return p.Content.DataUsage
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
	Expose bool   `yaml:"expose"`

	// --- Outbound Network Settings ---
	// Use Tor network for outbound connections (engine queries)
	// Per PART 31: Particularly relevant for VidVeil to anonymize search queries
	UseNetwork bool `yaml:"use_network"`

	// Allow users to opt-in to forwarding their IP address to video sites
	// When enabled, users can set a preference (via cookie) to include their IP
	// in X-Forwarded-For header - useful for geo-targeted content
	// Default: true (feature available), but user preference defaults to disabled
	AllowUserIPForward bool `yaml:"allow_user_ip_forward"`

	// --- Performance Settings ---
	// Time to wait for Tor's control port during startup (10-300, default 60)
	StartupTimeout int `yaml:"startup_timeout"`

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

	// OnionAddress is the .onion hostname for this service (without http:// prefix).
	// When set, requests whose Host header matches this value are treated as Tor requests.
	// Set automatically by the Tor service on startup; can also be set manually.
	OnionAddress string `yaml:"onion_address"`

	// ContactEmail is the contact address shown in Tor responses (security.txt, contact pages).
	// If unset, no email is shown on Tor responses — never falls back to the clearnet email.
	ContactEmail string `yaml:"contact_email"`
}

// I2PConfig configures the I2P eepsite provider per AI.md PART 31.2.
type I2PConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Expose           bool   `yaml:"expose"`
	Binary           string `yaml:"binary"`
	SAMAddress       string `yaml:"sam_address"`
	VirtualPort      int    `yaml:"virtual_port"`
	InboundLength    int    `yaml:"inbound_length"`
	OutboundLength   int    `yaml:"outbound_length"`
	InboundQuantity  int    `yaml:"inbound_quantity"`
	OutboundQuantity int    `yaml:"outbound_quantity"`
	SignatureType    int    `yaml:"signature_type"`
	BootstrapTimeout string `yaml:"bootstrap_timeout"`
	Address          string `yaml:"-" json:"-"`
}

// DefaultI2PConfig returns the PART 31.2 settings with I2P disabled. The
// application config enables it by default only in an application container.
func DefaultI2PConfig() I2PConfig {
	return I2PConfig{
		SAMAddress:       "127.0.0.1:7656",
		VirtualPort:      80,
		InboundLength:    3,
		OutboundLength:   3,
		InboundQuantity:  5,
		OutboundQuantity: 5,
		SignatureType:    7,
		BootstrapTimeout: "5m",
	}
}

// DefaultTorConfig returns the default Tor configuration per PART 31
func DefaultTorConfig() TorConfig {
	return TorConfig{
		// auto-detect
		Binary: "",
		// disabled by default, user can enable for privacy
		UseNetwork:     false,
		StartupTimeout: 60,
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

// SMTPConfig holds SMTP connection settings per AI.md PART 17
type SMTPConfig struct {
	// If empty: autodetect on first run. If set: test connection on every startup.
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// TLS mode: auto, starttls, tls, none
	TLS string `yaml:"tls"`
}

// EmailFromConfig holds sender address settings per AI.md PART 17
type EmailFromConfig struct {
	// Default: app title (Branding.Title)
	Name string `yaml:"name"`
	// Default: no-reply@{fqdn}
	Email string `yaml:"email"`
}

// EmailNotificationsConfig holds email notification config per AI.md PART 17.
// SMTP detection is automatic: empty Host triggers autodetect; a set Host triggers connection test.
// Enabled is set at runtime (true when SMTP test passes); it is never stored in YAML.
type EmailNotificationsConfig struct {
	// Enabled is set at runtime by the startup SMTP check. Not stored in config file.
	Enabled bool            `yaml:"-"`
	SMTP    SMTPConfig      `yaml:"smtp"`
	From    EmailFromConfig `yaml:"from"`
	// ReplyTo is optional. If set, it is included as a Reply-To header on all emails.
	ReplyTo string `yaml:"reply_to,omitempty"`
}

// NotificationsConfig holds notification settings per AI.md PART 17
type NotificationsConfig struct {
	Email         EmailNotificationsConfig `yaml:"email"`
	DatabaseError DatabaseErrorConfig      `yaml:"database_error"`
	DiskLow       DiskLowConfig            `yaml:"disk_low"`
}

// DatabaseErrorConfig controls the database connection failure trigger.
type DatabaseErrorConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Failures int    `yaml:"failures"`
	Repeat   string `yaml:"repeat"`
}

// DiskLowConfig controls low disk space checks and notification thresholds.
type DiskLowConfig struct {
	Enabled          bool   `yaml:"enabled"`
	ThresholdPercent int    `yaml:"threshold_percent"`
	Repeat           string `yaml:"repeat"`
}

// ContactRoleConfig holds contact settings for one notification role per AI.md PART 12.
// Each role has an email address and an open map of named webhook transports.
// Empty fields fall back to the admin role values at dispatch time.
type ContactRoleConfig struct {
	// Email address for this role. Empty string triggers fallback chain.
	Email string `yaml:"email"`
	// Webhooks maps transport name (telegram, discord, slack, mattermost,
	// pushover, gotify, generic, …) to the destination URL/token.
	// Each key also has a companion "<name>_secret" key that holds the
	// per-webhook HMAC-SHA256 signing secret (auto-generated on first save).
	Webhooks map[string]string `yaml:"webhooks,omitempty"`
}

// ContactConfig holds the unified notification-routing tree per AI.md PART 12.
// One config tree for every "where do messages go" decision.
type DPOContactConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

type ContactConfig struct {
	Admin    ContactRoleConfig `yaml:"admin"`
	Security ContactRoleConfig `yaml:"security"`
	Abuse    ContactRoleConfig `yaml:"abuse"`
	DPO      DPOContactConfig  `yaml:"dpo"`
	General  ContactRoleConfig `yaml:"general"`
}

// ScheduleConfig holds scheduler settings per AI.md PART 18.
// The scheduler is ALWAYS running — there is no top-level enable/disable.
// Individual tasks can be toggled via Tasks[id].Enabled.
type ScheduleConfig struct {
	Timezone      string                        `yaml:"timezone"`
	CatchUpWindow string                        `yaml:"catch_up_window"`
	Tasks         map[string]ScheduleTaskConfig `yaml:"tasks"`
}

// ScheduleTaskConfig holds per-task scheduler settings per AI.md PART 18
type ScheduleTaskConfig struct {
	Schedule      string                   `yaml:"schedule"`
	Enabled       bool                     `yaml:"enabled"`
	RetryOnFail   bool                     `yaml:"retry_on_fail,omitempty"`
	RetryDelay    string                   `yaml:"retry_delay,omitempty"`
	Verify        bool                     `yaml:"verify,omitempty"`
	RestartOnFail bool                     `yaml:"restart_on_fail,omitempty"`
	Retention     *ScheduleRetentionConfig `yaml:"retention,omitempty"`
}

// ScheduleRetentionConfig holds backup retention settings per AI.md PART 18
type ScheduleRetentionConfig struct {
	MaxBackups   int    `yaml:"max_backups"`
	KeepWeekly   int    `yaml:"keep_weekly"`
	KeepMonthly  int    `yaml:"keep_monthly"`
	KeepYearly   int    `yaml:"keep_yearly"`
	MaxTotalSize string `yaml:"max_total_size"`
}

// SSLConfig holds SSL/TLS settings
type SSLConfig struct {
	Enabled     bool              `yaml:"enabled"`
	CertPath    string            `yaml:"cert_path"`
	LetsEncrypt LetsEncryptConfig `yaml:"letsencrypt"`
}

// LetsEncryptConfig holds Let's Encrypt settings
type LetsEncryptConfig struct {
	Enabled        bool              `yaml:"enabled"`
	Domain         string            `yaml:"domain"`
	Email          string            `yaml:"email"`
	Challenge      string            `yaml:"challenge"`
	DNSProvider    string            `yaml:"dns_provider"`
	DNSCredentials map[string]string `yaml:"dns_credentials" json:"-"`
}

// MetricsConfig holds Prometheus metrics settings per AI.md PART 20
type MetricsConfig struct {
	Enabled         bool              `yaml:"enabled"`
	Root            MetricsRootConfig `yaml:"root"`
	Auth            MetricsAuthConfig `yaml:"auth"`
	IncludeSystem   bool              `yaml:"include_system"`
	IncludeRuntime  bool              `yaml:"include_runtime"`
	Loki            MetricsLokiConfig `yaml:"loki"`
	DurationBuckets []float64         `yaml:"duration_buckets"`
	SizeBuckets     []float64         `yaml:"size_buckets"`
}

// MetricsRootConfig gates the /metrics[/{service}] root alias per AI.md PART 20
type MetricsRootConfig struct {
	Enabled bool `yaml:"enabled"`
}

// MetricsAuthConfig holds metrics bearer-token auth settings per AI.md PART 20
type MetricsAuthConfig struct {
	// AllowUnauthenticated skips token checks for ALL metrics services -
	// firewalled internal networks only, never on a public server
	AllowUnauthenticated bool                `yaml:"allow_unauthenticated"`
	Tokens               MetricsTokensConfig `yaml:"tokens"`
}

// MetricsTokensConfig holds the per-service metrics bearer tokens per AI.md PART 20.
// An empty token disables that service's endpoints (403).
type MetricsTokensConfig struct {
	Prometheus string `yaml:"prometheus"`
	Grafana    string `yaml:"grafana"`
	Loki       string `yaml:"loki"`
}

// MetricsLokiConfig controls how much recent log the loki metrics service serves
type MetricsLokiConfig struct {
	MaxEntries int    `yaml:"max_entries"`
	MaxAge     string `yaml:"max_age"`
}

// GeoIPConfig holds GeoIP settings per AI.md PART 19
type GeoIPConfig struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
	Update  string `yaml:"update"`
	// CountryMode is "none" (default), "deny" (blocklist), or "allow" (allowlist-only)
	CountryMode    string   `yaml:"country_mode"`
	DenyCountries  []string `yaml:"deny_countries"`
	AllowCountries []string `yaml:"allow_countries"`
	// Presets are named, operator-authored country lists (name -> []code) kept
	// purely for reuse across allow/deny fields and environments. Per AI.md
	// PART 19 they ship empty, are never auto-applied, and never drive
	// enforcement — only deny_countries/allow_countries do.
	Presets   map[string][]string  `yaml:"presets"`
	Databases GeoIPDatabasesConfig `yaml:"databases"`
	// Content restriction for adult content laws
	ContentRestriction ContentRestrictionConfig `yaml:"content_restriction"`
}

// GeoIPDatabasesConfig holds which GeoIP databases to use per AI.md PART 19
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
	Level    string            `yaml:"level"`
	Defaults LogDefaultsConfig `yaml:"defaults"`
	Debug    DebugLogConfig    `yaml:"debug"`
	Access   AccessLogConfig   `yaml:"access"`
	Server   ServerLogConfig   `yaml:"server"`
	// AI.md PART 11: error.log
	Error    ErrorLogConfig    `yaml:"error"`
	Audit    AuditLogConfig    `yaml:"audit"`
	Security SecurityLogConfig `yaml:"security"`
	// AI.md PART 11: auth.log (authentication events, syslog format)
	Auth AuthLogConfig `yaml:"auth"`
	// AI.md PART 11: app.log / vidveil.log (general info/warn, logfmt format)
	App AppLogConfig `yaml:"app"`
}

// LogDefaultsConfig supplies the inherited custom format, rotation, and
// retention policy for each log kind.
type LogDefaultsConfig struct {
	Custom string `yaml:"custom"`
	Rotate string `yaml:"rotate"`
	Keep   string `yaml:"keep"`
}

func (logs *LogsConfig) UnmarshalYAML(node *yaml.Node) error {
	type plainLogs LogsConfig
	defaults := plainLogs(defaultLogsConfig())
	if err := node.Decode(&defaults); err != nil {
		return err
	}
	resolved := LogsConfig(defaults)
	if yamlMapValue(yamlMapValue(node, "defaults"), "rotate") != nil {
		if yamlMapValue(yamlMapValue(node, "debug"), "rotate") == nil {
			resolved.Debug.Rotate = ""
		}
		if yamlMapValue(yamlMapValue(node, "access"), "rotate") == nil {
			resolved.Access.Rotate = ""
		}
		if yamlMapValue(yamlMapValue(node, "server"), "rotate") == nil {
			resolved.Server.Rotate = ""
		}
		if yamlMapValue(yamlMapValue(node, "error"), "rotate") == nil {
			resolved.Error.Rotate = ""
		}
		if yamlMapValue(yamlMapValue(node, "audit"), "rotate") == nil {
			resolved.Audit.Rotate = ""
		}
		if yamlMapValue(yamlMapValue(node, "security"), "rotate") == nil {
			resolved.Security.Rotate = ""
		}
		if yamlMapValue(yamlMapValue(node, "auth"), "rotate") == nil {
			resolved.Auth.Rotate = ""
		}
		if yamlMapValue(yamlMapValue(node, "app"), "rotate") == nil {
			resolved.App.Rotate = ""
		}
	}
	applyLogDefaults(&resolved)
	*logs = resolved
	return nil
}

func defaultLogsConfig() LogsConfig {
	return LogsConfig{
		Level:    "info",
		Defaults: LogDefaultsConfig{Rotate: "weekly,50MB", Keep: "none"},
		Debug:    DebugLogConfig{Format: "text"},
		Access:   AccessLogConfig{Format: "apache", Rotate: "monthly"},
		Server:   ServerLogConfig{Format: "text"},
		Error:    ErrorLogConfig{Format: "text"},
		Audit: AuditLogConfig{Enabled: true, Format: "json", Rotate: "daily", Events: AuditLogEventsConfig{
			Configuration: true, Security: true, Backup: true, Server: true,
		}, IncludeUserAgent: true},
		Security: SecurityLogConfig{Enabled: true, Format: "fail2ban"},
		Auth:     AuthLogConfig{Enabled: true, Format: "syslog"},
		App:      AppLogConfig{Enabled: true, Format: "logfmt"},
	}
}

func applyLogDefaults(logs *LogsConfig) {
	apply := func(custom, keep, rotate *string) {
		if *custom == "" {
			*custom = logs.Defaults.Custom
		}
		if *keep == "" {
			*keep = logs.Defaults.Keep
		}
		if *rotate == "" {
			*rotate = logs.Defaults.Rotate
		}
	}
	apply(&logs.Debug.Custom, &logs.Debug.Keep, &logs.Debug.Rotate)
	apply(&logs.Access.Custom, &logs.Access.Keep, &logs.Access.Rotate)
	apply(&logs.Server.Custom, &logs.Server.Keep, &logs.Server.Rotate)
	apply(&logs.Error.Custom, &logs.Error.Keep, &logs.Error.Rotate)
	apply(&logs.Audit.Custom, &logs.Audit.Keep, &logs.Audit.Rotate)
	apply(&logs.Security.Custom, &logs.Security.Keep, &logs.Security.Rotate)
	apply(&logs.Auth.Custom, &logs.Auth.Keep, &logs.Auth.Rotate)
	apply(&logs.App.Custom, &logs.App.Keep, &logs.App.Rotate)
	if logs.Debug.Filename == "" {
		logs.Debug.Filename = "debug.log"
	}
	if logs.Access.Filename == "" {
		logs.Access.Filename = "access.log"
	}
	if logs.Server.Filename == "" {
		logs.Server.Filename = "server.log"
	}
	if logs.Error.Filename == "" {
		logs.Error.Filename = "error.log"
	}
	if logs.Audit.Filename == "" {
		logs.Audit.Filename = "audit.log"
	}
	if logs.Security.Filename == "" {
		logs.Security.Filename = "security.log"
	}
	if logs.Auth.Filename == "" {
		logs.Auth.Filename = "auth.log"
	}
	if logs.App.Filename == "" {
		logs.App.Filename = "vidveil.log"
	}
}

// DebugLogConfig holds debug log settings
type DebugLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename,omitempty"`
	Format   string `yaml:"format"`
	Custom   string `yaml:"custom,omitempty"`
	Keep     string `yaml:"keep,omitempty"`
	Rotate   string `yaml:"rotate,omitempty"`
}

// AccessLogConfig holds access log settings
type AccessLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename,omitempty"`
	Format   string `yaml:"format"`
	Custom   string `yaml:"custom,omitempty"`
	Keep     string `yaml:"keep,omitempty"`
	Rotate   string `yaml:"rotate,omitempty"`
}

// ServerLogConfig holds server log settings
type ServerLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename,omitempty"`
	Format   string `yaml:"format"`
	Custom   string `yaml:"custom,omitempty"`
	Keep     string `yaml:"keep,omitempty"`
	Rotate   string `yaml:"rotate,omitempty"`
}

// ErrorLogConfig holds error log settings per AI.md PART 11
type ErrorLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename,omitempty"`
	Format   string `yaml:"format"`
	Custom   string `yaml:"custom,omitempty"`
	Keep     string `yaml:"keep,omitempty"`
	Rotate   string `yaml:"rotate,omitempty"`
}

// AuditLogEventsConfig controls which event categories are written to audit.log
type AuditLogEventsConfig struct {
	Configuration bool `yaml:"configuration"`
	Security      bool `yaml:"security"`
	Backup        bool `yaml:"backup"`
	Server        bool `yaml:"server"`
}

// AuditLogConfig holds audit log settings per AI.md PART 11
type AuditLogConfig struct {
	Enabled          bool                 `yaml:"enabled"`
	Filename         string               `yaml:"filename,omitempty"`
	Format           string               `yaml:"format"`
	Custom           string               `yaml:"custom,omitempty"`
	Keep             string               `yaml:"keep,omitempty"`
	Rotate           string               `yaml:"rotate,omitempty"`
	Compress         bool                 `yaml:"compress"`
	Events           AuditLogEventsConfig `yaml:"events"`
	IncludeUserAgent bool                 `yaml:"include_user_agent"`
}

// SecurityLogConfig holds security log settings
type SecurityLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename,omitempty"`
	Format   string `yaml:"format"`
	Custom   string `yaml:"custom,omitempty"`
	Keep     string `yaml:"keep,omitempty"`
	Rotate   string `yaml:"rotate,omitempty"`
}

// AuthLogConfig holds authentication log settings per AI.md PART 11
// Default format: syslog (RFC 3164) for Fail2ban/SIEM integration
type AuthLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename,omitempty"`
	// Format: syslog (default), json
	Format string `yaml:"format"`
	Custom string `yaml:"custom,omitempty"`
	Keep   string `yaml:"keep,omitempty"`
	Rotate string `yaml:"rotate,omitempty"`
}

// AppLogConfig holds general application log settings per AI.md PART 11
// Default format: logfmt (key=value pairs)
// Also known as {project_name}.log
type AppLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Filename string `yaml:"filename,omitempty"`
	// Format: logfmt (default), json
	Format string `yaml:"format"`
	Custom string `yaml:"custom,omitempty"`
	Keep   string `yaml:"keep,omitempty"`
	Rotate string `yaml:"rotate,omitempty"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	Requests int `yaml:"requests"`
	Window   int `yaml:"window"`
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

// CORSConfig controls which browser origins may access the server.
// An exact empty-string origin disables CORS; an empty list uses discovered
// domains and trusted reverse-proxy hosts before falling back to '*'.
type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
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
}

// CSPConfig holds Content-Security-Policy settings per AI.md PART 11
// "Content Security Policy". The default policy (see server.buildCSPHeader)
// already covers typical apps (CDN images, web fonts, PWA workers) with zero
// config. Operators extend individual directives via the *_extra keys below —
// these values are APPENDED to the fixed default, never replacing it. The
// *_override keys REPLACE a directive entirely and should be used sparingly
// (e.g. nonce-based script-src tightening).
type CSPConfig struct {
	// Enabled toggles CSP header emission entirely (default true)
	Enabled bool `yaml:"enabled"`
	// Mode is "enforce" (Content-Security-Policy) or "report-only"
	// (Content-Security-Policy-Report-Only)
	Mode string `yaml:"mode"`
	// Per-directive append — added to the fixed default value.
	ScriptSrcExtra  string `yaml:"script_src_extra"`
	StyleSrcExtra   string `yaml:"style_src_extra"`
	ImgSrcExtra     string `yaml:"img_src_extra"`
	FontSrcExtra    string `yaml:"font_src_extra"`
	ConnectSrcExtra string `yaml:"connect_src_extra"`
	FrameSrcExtra   string `yaml:"frame_src_extra"`
	FormActionExtra string `yaml:"form_action_extra"`
	// Override-style — REPLACES the directive instead of appending.
	ScriptSrcOverride string `yaml:"script_src_override"`
	StyleSrcOverride  string `yaml:"style_src_override"`
	// ReportsEnabled controls POSTing violations to /api/{api_version}/server/reports/csp
	ReportsEnabled bool `yaml:"reports_enabled"`
	// ReportsSampleRate is 0.0..1.0 — sample to control volume on busy sites
	ReportsSampleRate float64 `yaml:"reports_sample_rate"`
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
	Dir        string                `yaml:"dir"`
	Allowlist  []AllowlistEntry      `yaml:"allowlist"`
	Blocklists BlocklistsConfig      `yaml:"blocklists"`
	CVE        CVEConfig             `yaml:"cve"`
	Reports    SecurityReportsConfig `yaml:"reports"`
	// EncryptionKey is the canonical 32-byte AES-256-GCM at-rest key (hex-encoded).
	// Per AI.md PART 11 "Cryptographic Keys": auto-generated on first run and
	// persisted in secrets.yml; used for ALL at-rest encryption of sensitive
	// server data (API token hashes, security report bodies as the AES
	// fallback when no PGP keypair exists, and any future at-rest data).
	EncryptionKey        string `yaml:"encryption_key"`
	EncryptionKeyVersion int    `yaml:"encryption_key_version"`
}

// SecurityReportsConfig controls retention for closed coordinated-disclosure
// reports. The hall-of-fame rows are retained independently.
type SecurityReportsConfig struct {
	RetentionDays int `yaml:"retention_days"`
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
	Retention          BackupRetentionConfig  `yaml:"retention"`
	Encryption         BackupEncryptionConfig `yaml:"encryption"`
	EncryptionPassword string                 `yaml:"encryption_password,omitempty" json:"-"`
	PasswordUnreadable bool                   `yaml:"-" json:"-"`
}

// MaintenanceConfig controls automatic recovery and disk-pressure cleanup.
type MaintenanceConfig struct {
	SelfHealing MaintenanceSelfHealingConfig `yaml:"self_healing"`
	Cleanup     MaintenanceCleanupConfig     `yaml:"cleanup"`
	Notify      MaintenanceNotifyConfig      `yaml:"notify"`
}

type MaintenanceSelfHealingConfig struct {
	RetryInterval string `yaml:"retry_interval"`
	MaxAttempts   uint64 `yaml:"max_attempts"`
}

type MaintenanceCleanupConfig struct {
	DiskThreshold    int `yaml:"disk_threshold"`
	LogRetentionDays int `yaml:"log_retention_days"`
}

type MaintenanceNotifyConfig struct {
	OnEnter bool `yaml:"on_enter"`
	OnExit  bool `yaml:"on_exit"`
}

// RestoreConfig bounds compressed and expanded backup archive processing per AI.md PART 21.
type RestoreConfig struct {
	MaxArchiveSize  string `yaml:"max_archive_size"`
	MaxExpandedSize string `yaml:"max_expanded_size"`
	MaxFiles        int    `yaml:"max_files"`
	MaxFileSize     string `yaml:"max_file_size"`
	MaxRatio        int    `yaml:"max_ratio"`
}

// DefaultRestoreConfig returns the archive limits required by AI.md PART 21.
func DefaultRestoreConfig() RestoreConfig {
	return RestoreConfig{
		MaxArchiveSize:  "1G",
		MaxExpandedSize: "4G",
		MaxFiles:        10000,
		MaxFileSize:     "512M",
		MaxRatio:        100,
	}
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
	// MaxTotalSize: hard cap on total backup directory size; percent ("10%") or absolute ("50G"); "" or "0" = disabled
	MaxTotalSize string `yaml:"max_total_size"`
}

// BackupEncryptionConfig holds backup encryption settings per AI.md PART 21
type BackupEncryptionConfig struct {
	// Enabled: true if backup password was set
	Enabled bool `yaml:"enabled"`
	// PasswordHint: optional hint for password (never store actual password)
	PasswordHint string `yaml:"password_hint,omitempty"`
}

// ComplianceConfig holds regulatory-standard toggles per AI.md "Compliance Standards".
// All standards are disabled by default; enable individually as needed. When any
// standard is enabled, PART 21's "server.compliance.enabled" backup-encryption
// enforcement (Backup Encryption / Compliance Mode Enforcement) is active - see
// IsEnabled().
type ComplianceConfig struct {
	GDPR     bool `yaml:"gdpr"`
	CCPA     bool `yaml:"ccpa"`
	HIPAA    bool `yaml:"hipaa"`
	SOC2     bool `yaml:"soc2"`
	PCIDSS   bool `yaml:"pci_dss"`
	ISO27001 bool `yaml:"iso27001"`
	FedRAMP  bool `yaml:"fedramp"`
	LGPD     bool `yaml:"lgpd"`
	PIPEDA   bool `yaml:"pipeda"`
	APPI     bool `yaml:"appi"`
	PDPA     bool `yaml:"pdpa"`
}

// IsEnabled reports whether "compliance mode" is active - true when any single
// regulatory standard is enabled. Per AI.md PART 21, this is what gates backup
// encryption being mandatory ("server.compliance.enabled: true").
func (c ComplianceConfig) IsEnabled() bool {
	return c.GDPR || c.CCPA || c.HIPAA || c.SOC2 || c.PCIDSS || c.ISO27001 ||
		c.FedRAMP || c.LGPD || c.PIPEDA || c.APPI || c.PDPA
}

// ComplianceStandard pairs a regulatory standard's config key with its display
// name and whether it is currently enabled. Used by the
// "--maintenance compliance report" summary (AI.md PART 5 "Compliance Routes").
type ComplianceStandard struct {
	Key     string
	Name    string
	Enabled bool
}

// Standards returns every regulatory standard with its enabled state, in a
// stable order matching the AI.md "Compliance-Specific Behaviors" section. The
// order is deterministic so the compliance report is reproducible.
func (c ComplianceConfig) Standards() []ComplianceStandard {
	return []ComplianceStandard{
		{"gdpr", "GDPR (EU General Data Protection Regulation)", c.GDPR},
		{"ccpa", "CCPA (California Consumer Privacy Act)", c.CCPA},
		{"hipaa", "HIPAA (US Health Insurance Portability and Accountability Act)", c.HIPAA},
		{"soc2", "SOC 2 (Service Organization Control 2)", c.SOC2},
		{"pci_dss", "PCI-DSS (Payment Card Industry Data Security Standard)", c.PCIDSS},
		{"iso27001", "ISO 27001 (Information Security Management)", c.ISO27001},
		{"fedramp", "FedRAMP (US Federal Risk and Authorization Management)", c.FedRAMP},
		{"lgpd", "LGPD (Brazil Lei Geral de Proteção de Dados)", c.LGPD},
		{"pipeda", "PIPEDA (Canada Personal Information Protection)", c.PIPEDA},
		{"appi", "APPI (Japan Act on Protection of Personal Information)", c.APPI},
		{"pdpa", "PDPA (Singapore Personal Data Protection Act)", c.PDPA},
	}
}

// EnabledStandards returns the display names of the standards that are enabled,
// in the same stable order as Standards.
func (c ComplianceConfig) EnabledStandards() []string {
	var names []string
	for _, s := range c.Standards() {
		if s.Enabled {
			names = append(names, s.Name)
		}
	}
	return names
}

// UpdateConfig holds update settings per AI.md PART 22
type UpdateConfig struct {
	// Branch: release channel — stable | beta | devel (default: stable)
	Branch string `yaml:"branch"`
	// AutoInstall: when true the update_check scheduler task installs eligible updates automatically
	// Default: false — the task notifies only; installing is always an explicit operator decision
	AutoInstall bool `yaml:"auto_install"`
	// DeferDays: a release must be at least this many days old before the task considers it eligible
	// 0 = immediately eligible; 30 = adopt only after 30 days of public availability
	DeferDays int `yaml:"defer_days"`
	// CLIMinVersion blocks CLI versions below the operator-configured floor.
	CLIMinVersion string `yaml:"cli_min_version"`
}

// NormalizeUpdateChannel maps supported aliases to the canonical stored
// channel names defined in AI.md PART 22.
func NormalizeUpdateChannel(channel string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "", "stable":
		return "stable", true
	case "beta":
		return "beta", true
	case "devel", "daily", "rolling", "dev", "development":
		return "devel", true
	default:
		return "", false
	}
}

// SessionConfig holds session settings
type SessionConfig struct {
	CookieName string `yaml:"cookie_name"`
	MaxAge     int    `yaml:"max_age"`
	Secure     string `yaml:"secure"`
	HTTPOnly   bool   `yaml:"http_only"`
	SameSite   string `yaml:"same_site"`
}

// DatabaseConfig holds database settings per AI.md PART 10.
// Supported drivers: sqlite (aliases sqlite2/sqlite3/file) and libsql (alias turso).
type DatabaseConfig struct {
	Driver string       `yaml:"driver"`
	SQLite SQLiteConfig `yaml:"sqlite"`
	// URL is the connection URL for libsql/Turso (remote-only)
	URL string `yaml:"url"`
	// Token is the libsql/Turso auth token; appended as authToken when not in URL
	Token string `yaml:"token"`
}

// CacheConfig holds the optional cache backend settings per AI.md PART 12.
// Type defaults to "memory" (in-process). "valkey"/"redis" enable an external
// cache; connect via url OR host/port (url takes precedence).
type CacheConfig struct {
	// Type: none (disabled), memory (default), valkey, redis
	Type string `yaml:"type"`
	// URL takes precedence over host/port when set (redis:// or valkey://)
	URL string `yaml:"url"`
	// Individual connection settings (alternative to url)
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	// TLS settings for secure connections
	TLS           bool `yaml:"tls"`
	TLSSkipVerify bool `yaml:"tls_skip_verify"`
	// Connection pool
	PoolSize int `yaml:"pool_size"`
	MinIdle  int `yaml:"min_idle"`
	// Timeout in seconds
	Timeout int `yaml:"timeout"`
	// Prefix avoids key collisions across apps
	Prefix string `yaml:"prefix"`
	// TTL for the API response (search) cache, in seconds
	TTL int `yaml:"ttl"`
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
	CSP           CSPConfig           `yaml:"csp"`
	CSRF          CSRFConfig          `yaml:"csrf"`
	Footer        FooterConfig        `yaml:"footer"`
	// HonorDNT opts in to honoring the legacy DNT:1 request header as a privacy
	// opt-out (default false) per AI.md PART 11 "Privacy Signal Headers". Sec-GPC
	// is always honored regardless of this setting.
	HonorDNT bool `yaml:"honor_dnt"`
	// LLMS configures the AI-agent discovery document per AI.md PART 14.
	LLMS LLMSConfig `yaml:"llms"`
	// APIContact is the contact address advertised to API consumers.
	APIContact string `yaml:"api_contact"`
}

// LLMSConfig controls generated llms.txt output per AI.md PART 14.
type LLMSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	IncludeEndpoints bool     `yaml:"include_endpoints"`
	IncludeSchemas   bool     `yaml:"include_schemas"`
	CustomSections   []string `yaml:"custom_sections"`
}

// UIConfig holds UI settings
type UIConfig struct {
	Theme string `yaml:"theme"`
}

// AnnouncementsConfig holds announcement settings per AI.md PART 16/25 "Announcements".
type AnnouncementsConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Messages []Announcement `yaml:"messages"`
}

// Announcement is a single site-wide banner message per AI.md PART 16 "Site Banner".
type Announcement struct {
	ID string `yaml:"id"`
	// Type is one of: info, warning, error, success.
	Type    string `yaml:"type"`
	Title   string `yaml:"title"`
	Message string `yaml:"message"`
	// Start is the ISO 8601 UTC time the banner begins showing; empty = always active from the start.
	Start string `yaml:"start"`
	// End is the ISO 8601 UTC time the banner stops showing; empty = no end.
	End string `yaml:"end"`
	// Dismissible allows visitors to close the banner (tracked via the dismissed_announcements cookie).
	Dismissible bool `yaml:"dismissible"`
}

// RobotsConfig holds robots.txt settings
type RobotsConfig struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
	// AIBots controls per-AI-crawler access per AI.md PART 11 "robots.txt".
	AIBots AIBotsConfig `yaml:"ai_bots"`
}

// AIBotsConfig holds per-AI-crawler robots.txt policy. Default posture is
// allow: a bot only gets its own Disallow stanza when it (or Default) is deny.
type AIBotsConfig struct {
	// Default applies to any recognized AI bot not listed in Bots ("allow" or "deny").
	Default string `yaml:"default"`
	// Bots holds explicit per-bot overrides, which always beat Default.
	Bots map[string]string `yaml:"bots"`
}

// KnownAIBots lists the AI crawler user agents recognized by robots.txt
// generation, in the order they are rendered (AI.md PART 11).
var KnownAIBots = []string{
	"GPTBot",
	"ChatGPT-User",
	"ClaudeBot",
	"anthropic-ai",
	"Claude-Web",
	"CCBot",
	"Google-Extended",
	"Bytespider",
	"PerplexityBot",
	"Applebot-Extended",
	"Amazonbot",
	"Diffbot",
	"FacebookBot",
	"cohere-ai",
}

// DeniedAIBots returns the recognized AI crawlers that must receive their own
// "Disallow: /" stanza, resolving explicit per-bot entries over the default.
func (r RobotsConfig) DeniedAIBots() []string {
	denyByDefault := strings.EqualFold(strings.TrimSpace(r.AIBots.Default), "deny")
	var denied []string
	for _, bot := range KnownAIBots {
		policy := ""
		for name, value := range r.AIBots.Bots {
			if strings.EqualFold(name, bot) {
				policy = strings.ToLower(strings.TrimSpace(value))
				break
			}
		}
		switch policy {
		case "deny":
			denied = append(denied, bot)
		case "allow":
		default:
			if denyByDefault {
				denied = append(denied, bot)
			}
		}
	}
	return denied
}

// WebSecurityConfig holds security.txt settings
type WebSecurityConfig struct {
	Contact string `yaml:"contact"`
	Expires string `yaml:"expires"`
	// PGPKeyURL is the URL of the published PGP public key (set when a keypair is generated).
	// When non-empty, an Encryption: line is added to security.txt.
	PGPKeyURL string `yaml:"pgp_key_url"`
	// Keyservers lists the OpenPGP keyservers the public key is published to
	// via `--maintenance pgp publish` (AI.md PART 12 "GPG Keypair Management").
	Keyservers []string `yaml:"keyservers"`
	// PublishPGPKey is set true once a keypair has been generated and the public
	// key is being served, gating the security.txt Encryption line and endpoints.
	PublishPGPKey bool `yaml:"publish_pgp_key"`
	// ReportURL is the repo-level (source-code) vulnerability reporting channel
	// (e.g. GitHub private vulnerability reporting). Per AI.md PART 11
	// "Security Reports": this is the FIRST entry in the security.txt Contact
	// preference order, ahead of the /server/contact security-id mode and the
	// mailto CC address.
	ReportURL string `yaml:"report_url"`
	// DisclosureWindowDays is the default coordinated-disclosure window (days)
	// offered to researchers; researcher-submitted preference may differ and
	// is negotiated by maintainers. Per AI.md PART 11 "Security-mode form
	// fields" default 90.
	DisclosureWindowDays int `yaml:"disclosure_window_days"`
	// PolicyText overrides the default /server/security/policy body when set
	// (config-file-driven per AI.md PART 11 "Security Administration" — there
	// is no admin web route/API for this content).
	PolicyText string `yaml:"policy_text"`
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

// SEOSitemapConfig controls generated public sitemap output per AI.md PART 16.
type SEOSitemapConfig struct {
	MaxURLs       int  `yaml:"max_urls"`
	IncludeImages bool `yaml:"include_images"`
}

type SEOFeedConfig struct {
	MaxItems int    `yaml:"max_items"`
	Title    string `yaml:"title"`
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
	TwitterHandle  string            `yaml:"twitter_handle"`
	TitleSeparator string            `yaml:"title_separator"`
	MetaTags       map[string]string `yaml:"meta_tags"`
	Canonical      string            `yaml:"canonical"`
	NoIndex        bool              `yaml:"noindex"`
	OpenGraph      OpenGraphConfig   `yaml:"opengraph"`
	Twitter        TwitterCardConfig `yaml:"twitter"`
	// Verification holds search engine verification codes
	Verification SEOVerificationConfig `yaml:"verification"`
	// Sitemap controls generated sitemap output.
	Sitemap SEOSitemapConfig `yaml:"sitemap"`
	Feed    SEOFeedConfig    `yaml:"feed"`
}

type OpenGraphConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Type     string `yaml:"type"`
	SiteName string `yaml:"site_name"`
}

type TwitterCardConfig struct {
	Enabled bool   `yaml:"enabled"`
	Card    string `yaml:"card"`
	Creator string `yaml:"creator"`
}

// CSRFConfig holds CSRF settings per AI.md PART 16 → CSRF Protection
type CSRFConfig struct {
	Enabled     bool   `yaml:"enabled"`
	TokenLength int    `yaml:"token_length"`
	CookieName  string `yaml:"cookie_name"`
	HeaderName  string `yaml:"header_name"`
	// Secure sets the Secure cookie flag: "auto" (https only), "true", or "false"
	Secure string `yaml:"secure"`
	// ExemptPaths lists endpoints exempt from CSRF (OAuth callbacks, webhook receivers).
	// Glob patterns supported. Default exempts /api/{api_version}/webhooks/*.
	ExemptPaths []string `yaml:"exempt_paths"`
}

// FooterConfig holds footer settings. Cookie-consent banner text lives at
// server.privacy.consent and analytics configuration at server.tracking.
type FooterConfig struct {
	CustomHTML string `yaml:"custom_html"`
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
	// DetailEnrichment fetches each result's own detail page to extract
	// schema.org VideoObject JSON-LD (description/tags/performer/rating/
	// published) that listing pages never carry. Disabled by default: it adds
	// one extra upstream request per enriched result on every search.
	DetailEnrichment DetailEnrichmentConfig `yaml:"detail_enrichment"`
}

// DetailEnrichmentConfig controls the optional per-result detail-page fetch
// used to backfill schema.org VideoObject JSON-LD fields that only appear on
// individual video pages, never on search/listing pages.
type DetailEnrichmentConfig struct {
	// Enabled turns on the extra per-result detail-page fetch. Off by default.
	Enabled bool `yaml:"enabled"`
	// MaxResults caps how many of the top results per engine are enriched,
	// bounding the extra request volume against the upstream site.
	MaxResults int `yaml:"max_results"`
	// Timeout in seconds for each individual detail-page fetch. Kept short by
	// design - enrichment must never make results feel slow to the user; a
	// slow/unresponsive detail page just misses enrichment, it never delays
	// the search response beyond this budget.
	Timeout int `yaml:"timeout"`
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

// ValidateTracking validates an optional analytics provider configuration.
func ValidateTracking(cfg *TrackingConfig) error {
	if cfg == nil || cfg.Type == "" || cfg.Type == "none" {
		return nil
	}
	validURL := func(raw string) bool {
		parsed, err := url.Parse(raw)
		return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != ""
	}
	switch strings.ToLower(cfg.Type) {
	case "google":
		if !regexp.MustCompile(`^(UA-[0-9]+-[0-9]+|G-[A-Z0-9]+)$`).MatchString(cfg.ID) {
			return errors.New("google analytics ID must match UA-N-N or G-XXXXXXXX")
		}
	case "matomo", "piwik":
		if _, err := strconv.Atoi(cfg.ID); err != nil {
			return errors.New("matomo/piwik ID must be an integer")
		}
		if !validURL(cfg.URL) {
			return errors.New("matomo/piwik requires a valid HTTP(S) URL")
		}
	case "owa":
		if cfg.ID == "" || !validURL(cfg.URL) {
			return errors.New("owa requires a site ID and valid HTTP(S) URL")
		}
	case "fathom":
		if cfg.ID == "" || (cfg.URL != "" && !validURL(cfg.URL)) {
			return errors.New("fathom requires a site ID and an optional valid HTTP(S) URL")
		}
	case "plausible":
		if !validAnalyticsDomain(cfg.ID) || (cfg.URL != "" && !validURL(cfg.URL)) {
			return errors.New("plausible requires a domain and an optional valid HTTP(S) URL")
		}
	case "umami":
		if !validAnalyticsUUID(cfg.ID) || !validURL(cfg.URL) {
			return errors.New("umami requires a UUID and valid HTTP(S) URL")
		}
	case "simple":
	case "cloudflare":
		if cfg.ID == "" {
			return errors.New("cloudflare web analytics requires a beacon token")
		}
	default:
		return fmt.Errorf("unknown analytics type %q", cfg.Type)
	}
	if cfg.URL != "" && !validURL(cfg.URL) {
		return errors.New("tracking URL must be a valid HTTP(S) URL")
	}
	cfg.Type = strings.ToLower(cfg.Type)
	return nil
}

var analyticsDomainPattern = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
var analyticsUUIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func validAnalyticsDomain(domain string) bool { return analyticsDomainPattern.MatchString(domain) }
func validAnalyticsUUID(id string) bool       { return analyticsUUIDPattern.MatchString(id) }

func validateContactCaptcha(cfg *ServerContactPagesConfig, defaults ServerContactPagesConfig) {
	if cfg == nil {
		return
	}
	cfg.Captcha = strings.ToLower(strings.TrimSpace(cfg.Captcha))
	switch cfg.Captcha {
	case "", "simple":
		cfg.Captcha = "simple"
		cfg.CaptchaSiteKey = ""
		cfg.CaptchaSecretKey = ""
	case "recaptcha", "hcaptcha":
		if cfg.CaptchaSiteKey == "" || cfg.CaptchaSecretKey == "" {
			fmt.Fprintf(os.Stderr, "Warning: %s requires captcha_site_key and captcha_secret_key; using simple captcha instead\n", cfg.Captcha)
			cfg.Captcha = defaults.Captcha
			cfg.CaptchaSiteKey = defaults.CaptchaSiteKey
			cfg.CaptchaSecretKey = defaults.CaptchaSecretKey
		}
	default:
		fmt.Fprintf(os.Stderr, "Warning: unknown contact captcha %q; using simple captcha instead\n", cfg.Captcha)
		cfg.Captcha = defaults.Captcha
		cfg.CaptchaSiteKey = defaults.CaptchaSiteKey
		cfg.CaptchaSecretKey = defaults.CaptchaSecretKey
	}
}

// DefaultAppConfig returns an AppConfig with sensible defaults per AI.md
func DefaultAppConfig() *AppConfig {
	fqdn := getHostname()
	// Per AI.md PART 5: Default port is random 64xxx (non-privileged, no root required)
	defaultPort := fmt.Sprintf("%d", findUnusedPort())
	// Per AI.md PART 14: default {api_version}; never hardcode "/api/v1" elsewhere.
	defaultAPIVersion := "v1"
	defaultAPIBase := "/api/" + defaultAPIVersion

	return &AppConfig{
		CLI: CLIConfig{BinaryDownload: CLIBinaryDownloadConfig{Dir: "{data_dir}/cli"}},
		Server: ServerSettings{
			Port:       defaultPort,
			FQDN:       fqdn,
			Address:    "[::]",
			BaseURL:    "/",
			APIVersion: defaultAPIVersion,
			Mode:       "production",
			Pages: ServerPagesConfig{
				Contact: ServerContactPagesConfig{
					Enabled: true, Captcha: "simple", SuccessMessage: "Thank you for your message. We'll respond soon.",
					Antispam: ContactAntispamConfig{
						Honeypot: true, MinFillSeconds: 3, MaxAgeSeconds: 43200,
						PerIP: 5, PerEmail: 3, Global: 60, Window: "1h", MaxLinks: 10,
						ReviewMinSeconds: 2, ReviewMaxSeconds: 900,
						RejectMarkup: true, Strikes: 3, BanHours: 24, DuplicateWindowHours: 24,
					},
				},
				Help: ServerHelpPagesConfig{
					Content: LocalizedContentBody{Text: DefaultHelpContent},
					MCP:     vidveilMCPHelp,
				},
			},
			Tokens: TokensConfig{MaxLinked: 25, CookieMaxAge: "1y", IdlePurgeDays: 0},
			Token:  defaultBearerToken(),
			Security: SecurityConfig{
				EncryptionKey: generateToken(32),
				Reports:       SecurityReportsConfig{RetentionDays: 365},
			},
			Branding: ServerBrandingConfig{
				Title:           "Vidveil",
				Tagline:         "Privacy-first video search",
				Description:     "Privacy-respecting adult video search",
				RefreshInterval: "24h",
			},
			SEO: SEOConfig{
				TitleSeparator: " - ",
				MetaTags:       map[string]string{},
				OpenGraph:      OpenGraphConfig{Enabled: true, Type: "website"},
				Twitter:        TwitterCardConfig{Enabled: true, Card: "summary_large_image"},
				Sitemap:        SEOSitemapConfig{MaxURLs: 50000},
				Feed:           SEOFeedConfig{MaxItems: 50},
			},
			PIDFile: true,
			Contact: ContactConfig{
				Admin: ContactRoleConfig{
					Email:    "admin@" + fqdn,
					Webhooks: map[string]string{},
				},
				Security: ContactRoleConfig{
					Email:    "security@" + fqdn,
					Webhooks: map[string]string{},
				},
				Abuse: ContactRoleConfig{
					Email:    "",
					Webhooks: map[string]string{},
				},
				DPO: DPOContactConfig{},
				General: ContactRoleConfig{
					Email:    "",
					Webhooks: map[string]string{},
				},
			},
			Notifications: NotificationsConfig{
				DatabaseError: DatabaseErrorConfig{
					Enabled: true, Failures: 3, Repeat: "24h",
				},
				DiskLow: DiskLowConfig{
					Enabled: true, ThresholdPercent: 85, Repeat: "24h",
				},
				Email: EmailNotificationsConfig{
					SMTP: SMTPConfig{
						Port: 587,
						TLS:  "auto",
					},
					From: EmailFromConfig{
						Email: "no-reply@" + fqdn,
					},
				},
			},
			Schedule: ScheduleConfig{
				Timezone:      "America/New_York",
				CatchUpWindow: "1h",
				Tasks: map[string]ScheduleTaskConfig{
					"ssl_renewal":      {Schedule: "0 3 * * *", Enabled: true},
					"database_check":   {Schedule: "@every 1m", Enabled: true},
					"disk_check":       {Schedule: "@every 5m", Enabled: true},
					"geoip_update":     {Schedule: "0 3 * * 0", Enabled: true, RetryOnFail: true, RetryDelay: "1h"},
					"blocklist_update": {Schedule: "0 4 * * *", Enabled: true, RetryOnFail: true, RetryDelay: "1h"},
					"cve_update":       {Schedule: "0 5 * * *", Enabled: true, RetryOnFail: true, RetryDelay: "1h"},
					"update_check":     {Schedule: "0 6 * * *", Enabled: true},
					"token_cleanup":    {Schedule: "@every 15m", Enabled: true},
					"keyserver_retry":  {Schedule: "@every 15m", Enabled: true},
					"log_rotation":     {Schedule: "0 0 * * *", Enabled: true},
					"backup_daily": {
						Schedule: "0 2 * * *", Enabled: true, Verify: true,
						Retention: &ScheduleRetentionConfig{
							MaxBackups: 1, KeepWeekly: 0, KeepMonthly: 0, KeepYearly: 0,
							MaxTotalSize: "10%",
						},
					},
					"backup_hourly":    {Schedule: "@hourly", Enabled: false},
					"healthcheck_self": {Schedule: "@every 5m", Enabled: true},
					"tor_health":       {Schedule: "@every 10m", Enabled: true, RestartOnFail: true},
				},
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
				Enabled: false,
				Root: MetricsRootConfig{
					Enabled: true,
				},
				Auth: MetricsAuthConfig{
					AllowUnauthenticated: false,
					Tokens:               MetricsTokensConfig{},
				},
				IncludeSystem:  true,
				IncludeRuntime: true,
				Loki: MetricsLokiConfig{
					MaxEntries: 1000,
					MaxAge:     "1h",
				},
				DurationBuckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				SizeBuckets:     []float64{100, 1000, 10000, 100000, 1000000, 10000000},
			},
			Logs: LogsConfig{
				Level: "info",
				Defaults: LogDefaultsConfig{
					Custom: "",
					Keep:   "none",
					Rotate: "weekly,50MB",
				},
				Debug: DebugLogConfig{
					Enabled:  false,
					Filename: "debug.log",
					Format:   "text",
					Keep:     "none",
					Rotate:   "weekly,50MB",
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
					Enabled:  true,
					Filename: "audit.log",
					Format:   "json",
					Keep:     "none",
					// AI.md PART 11: audit.log rotates daily
					Rotate:           "daily",
					Compress:         false,
					IncludeUserAgent: true,
					Events: AuditLogEventsConfig{
						Configuration: true,
						Security:      true,
						Backup:        true,
						Server:        true,
					},
				},
				Security: SecurityLogConfig{
					Enabled:  true,
					Filename: "security.log",
					Format:   "fail2ban",
					Keep:     "none",
					Rotate:   "weekly,50MB",
				},
				Auth: AuthLogConfig{
					Enabled:  true,
					Filename: "auth.log",
					// AI.md PART 11: auth.log default format syslog (RFC 3164)
					Format: "syslog",
					Keep:   "none",
					Rotate: "weekly,50MB",
				},
				App: AppLogConfig{
					Enabled:  true,
					Filename: "vidveil.log",
					// AI.md PART 11: app.log default format logfmt
					Format: "logfmt",
					Keep:   "none",
					Rotate: "weekly,50MB",
				},
			},
			RateLimit: RateLimitConfig{
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
			CORS: CORSConfig{
				AllowedOrigins:   []string{"*"},
				AllowCredentials: false,
				MaxAge:           86400,
			},
			SecurityHeaders: SecurityHeadersConfig{
				Enabled:             true,
				HSTS:                true,
				HSTSMaxAge:          31536000,
				XFrameOptions:       "SAMEORIGIN",
				XContentTypeOptions: "nosniff",
				XXSSProtection:      "1; mode=block",
				ReferrerPolicy:      "strict-origin-when-cross-origin",
			},
			Session: SessionConfig{
				CookieName: "session_id",
				// 30 days
				MaxAge:   2592000,
				Secure:   "auto",
				HTTPOnly: true,
				// Strict per AI.md PART 16: blocks cross-site cookie attachment entirely,
				// neutralizing most CSRF before the token check even runs.
				SameSite: "strict",
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
			Cache: CacheConfig{
				Type:     "memory",
				Host:     "localhost",
				Port:     6379,
				PoolSize: 10,
				MinIdle:  2,
				Timeout:  5,
				Prefix:   "vidveil:",
				TTL:      30,
			},
			GeoIP: GeoIPConfig{
				Enabled:        true,
				Dir:            "",
				Update:         "weekly",
				CountryMode:    "none",
				DenyCountries:  []string{},
				AllowCountries: []string{},
				Presets:        map[string][]string{},
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
					MaxBackups:   1,
					KeepWeekly:   0,
					KeepMonthly:  0,
					KeepYearly:   0,
					MaxTotalSize: "10%",
				},
				Encryption: BackupEncryptionConfig{
					// Not encrypted by default
					Enabled: false,
				},
			},
			Maintenance: MaintenanceConfig{
				SelfHealing: MaintenanceSelfHealingConfig{RetryInterval: "30s", MaxAttempts: 0},
				Cleanup:     MaintenanceCleanupConfig{DiskThreshold: 90, LogRetentionDays: 7},
				Notify:      MaintenanceNotifyConfig{OnEnter: true, OnExit: true},
			},
			Restore: DefaultRestoreConfig(),
			// Tor settings per AI.md PART 31
			// Hidden service auto-enabled if tor binary found
			// Outbound network disabled by default - can be enabled for privacy
			Tor: DefaultTorConfig(),
			I2P: func() I2PConfig {
				cfg := DefaultI2PConfig()
				cfg.Enabled = system.IsApplicationContainer()
				return cfg
			}(),
			// Update settings per AI.md PART 22
			// Branch: stable by default; auto_install: false (notify-only); defer_days: 0
			Update: UpdateConfig{
				Branch:        "stable",
				AutoInstall:   false,
				DeferDays:     0,
				CLIMinVersion: "",
			},
			// Privacy/cookie-consent banner text per AI.md PART 12.
			// Data.Sold defaults false (VidVeil per IDEA.md does not sell data);
			// MessageIfSold is left empty since GetConsentMessage() only uses it
			// when Data.Sold is true.
			Privacy: PrivacyConfig{
				Consent: ConsentConfig{
					Message: "This site uses essential cookies and, with your consent, preference and analytics cookies. You can change your choice at any time.",
					Policy: ConsentPolicyConfig{
						URL:  "/server/privacy",
						Text: "Privacy Policy",
					},
					Buttons: ConsentButtonsConfig{
						Decline: "Decline",
						Accept:  "Accept",
					},
					PreferencesText: "Cookie Preferences",
				},
				Data: PrivacyDataConfig{
					Sold:           false,
					StoredOnServer: true,
					Sharing: []SharingCondition{
						{Condition: "analytics", When: "Tracking configured and user consents", Data: "Anonymized page views, browser type, and country"},
						{Condition: "email", When: "SMTP configured for sending emails", Data: "Email address and message content"},
						{Condition: "user_initiated", When: "User explicitly shares content", Data: "The content the user chooses to share"},
					},
				},
				Retention: PrivacyRetentionConfig{
					Period:            "Favorites remain until you delete them with your owner token. Preferences are stored in browser cookies. Operational logs are kept for up to 30 days. Anonymized analytics data may be retained for up to 12 months when tracking is configured.",
					ExportAvailable:   true,
					DeletionAvailable: true,
				},
				Cookies: CookieCategoriesConfig{
					Essential: CookieCategoryConfig{
						Enabled:     true,
						Description: "Required for the site to function: age acknowledgement, cookie consent state and CSRF protection.",
					},
					Preferences: CookieCategoryConfig{
						Enabled:     true,
						Description: "Remember your theme, language and search settings between visits.",
					},
					Analytics: AnalyticsCookieConfig{
						Enabled:                  false,
						Description:              "Measure aggregate usage of the site.",
						DescriptionSuffixNotSold: "This data is anonymized and never sold.",
						DescriptionSuffixSold:    "This data may be shared with third parties.",
					},
				},
				ThirdParty: ThirdPartyConfig{
					Services: []ThirdPartyService{},
				},
				Content: PrivacyContentConfig{
					DataCollection:  "We process request metadata such as IP addresses for security and abuse prevention. Search queries are sent to the video engines selected for a search. Queries that pass privacy filters may appear in public recent-search feeds; these bounded entries stay in process memory and are cleared on restart. Configured access logs can include request URLs. Optional analytics records usage only after consent.",
					DataUsage:       "Data is used to provide search, maintain service security, and improve reliability. Data is not sold or used for targeted advertising.",
					DataUsageIfSold: "Data is used to provide and secure the service. Some data may be shared or sold as described in the configured third-party services and applicable rights below.",
					DataSecurity:    "Security controls protect service data. Access logs are retained for up to 30 days, and API token identifiers are stored as hashes.",
				},
			},
		},
		Web: WebConfig{
			LLMS:       LLMSConfig{Enabled: true, IncludeEndpoints: true, IncludeSchemas: false, CustomSections: []string{}},
			APIContact: "api@" + fqdn,
			UI: UIConfig{
				Theme: "dark",
			},
			Announcements: AnnouncementsConfig{
				Enabled:  true,
				Messages: []Announcement{},
			},
			Robots: RobotsConfig{
				Allow: []string{"/"},
				Deny:  []string{"/search", "/api/"},
				// Default posture per AI.md PART 11: no AI crawler is blocked
				// unless the operator explicitly denies it.
				AIBots: AIBotsConfig{
					Default: "allow",
					Bots:    map[string]string{},
				},
			},
			Security: WebSecurityConfig{
				Contact:              "security@" + fqdn,
				Keyservers:           []string{"keys.openpgp.org"},
				DisclosureWindowDays: 90,
			},
			CSP: CSPConfig{
				Enabled:           true,
				Mode:              "enforce",
				ReportsEnabled:    true,
				ReportsSampleRate: 1.0,
			},
			CSRF: CSRFConfig{
				Enabled:     true,
				TokenLength: 32,
				CookieName:  "csrf_token",
				HeaderName:  "X-CSRF-Token",
				Secure:      "auto",
				ExemptPaths: []string{defaultAPIBase + "/webhooks/*"},
			},
			Footer: FooterConfig{},
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
			// Detail-page JSON-LD enrichment: off by default (extra upstream
			// request per enriched result); when enabled, cap to the top 5
			// results per engine with a 2s per-fetch budget so it can never
			// make search results feel slow (fetches run concurrently and are
			// bounded by the per-engine search deadline regardless)
			DetailEnrichment: DetailEnrichmentConfig{
				Enabled:    false,
				MaxResults: 5,
				Timeout:    2,
			},
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
		cfg.Server.Token = ""
		cfg.Server.Security.EncryptionKey = ""
		cfg.Server.Security.EncryptionKeyVersion = 0

		// Set paths in config
		cfg.Server.SSL.CertPath = filepath.Join(paths.Config, "ssl", "certs")
		cfg.Server.Database.SQLite.Dir = dbDir
		seedFirstRunEnvironment(cfg)

		if err := saveInitialServerConfig(cfg, configPath); err != nil {
			return nil, "", fmt.Errorf("failed to save default config: %w", err)
		}

	}
	legacySecrets, migrationErr := migrateConfigLayout(configPath)
	if migrationErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: configuration migration skipped: %v\n", migrationErr)
	}

	// Load existing config
	data, dropins, overrides, err := loadMergedConfig(configPath)
	if err != nil {
		if strings.Contains(err.Error(), "yaml:") {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s (%v), using default configuration\n", configPath, err)
			fallback := DefaultAppConfig()
			clearGeneratedSecretDefaults(fallback)
			data, err = marshalYAML(fallback)
			dropins, overrides = nil, nil
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Start with defaults and overlay the file contents.
	// Per AI.md PART 5: never fail startup on invalid config - warn and use defaults.
	// Unknown or legacy keys are ignored rather than treated as fatal.
	cfg := DefaultAppConfig()
	cfg.Server.Token = ""
	cfg.Server.Security.EncryptionKey = ""
	cfg.Server.Security.EncryptionKeyVersion = 0
	cfg.Server.Metrics.Auth.Tokens = MetricsTokensConfig{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to parse %s (%v), using default configuration\n", configPath, err)
		cfg = DefaultAppConfig()
		clearGeneratedSecretDefaults(cfg)
	}
	cfg.LegacyAppSecrets = legacySecrets
	if cfg.Server.Backup.EncryptionPassword == "********" {
		cfg.Server.Backup.EncryptionPassword = ""
	}
	if len(cfg.LegacyAppSecrets) == 0 {
		cfg.LegacyAppSecrets = readLegacySecretsFromBackup(configPath)
	}
	if err := ensureGeneratedConfigSecrets(paths.Config, cfg); err != nil {
		return nil, "", fmt.Errorf("ensure generated config secrets: %w", err)
	}
	credentialFiles := append([]string{configPath}, dropins...)
	if _, err := os.Stat(filepath.Join(paths.Config, "secrets.yml")); err == nil {
		credentialFiles = append(credentialFiles, filepath.Join(paths.Config, "secrets.yml"))
	}
	if err := persistEncryptedCredentials(credentialFiles, cfg.Server.Security.EncryptionKey); err != nil {
		return nil, "", fmt.Errorf("protect encrypted credentials: %w", err)
	}
	for name, value := range cfg.Server.SSL.LetsEncrypt.DNSCredentials {
		plain, decryptErr := decryptDNSCredential(value, cfg.Server.Security.EncryptionKey)
		if decryptErr != nil {
			return nil, "", fmt.Errorf("decrypt DNS credential %s: %w", name, decryptErr)
		}
		cfg.Server.SSL.LetsEncrypt.DNSCredentials[name] = plain
	}
	backupPassword, backupPasswordErr := decryptDNSCredential(cfg.Server.Backup.EncryptionPassword, cfg.Server.Security.EncryptionKey)
	if backupPasswordErr != nil {
		fmt.Fprintln(os.Stderr, "WARN: backup.password_unreadable: scheduled backups are disabled until the configured encryption key is restored")
		cfg.Server.Backup.EncryptionPassword = ""
		cfg.Server.Backup.PasswordUnreadable = true
	} else {
		cfg.Server.Backup.EncryptionPassword = backupPassword
	}

	// Validate and fix invalid config values per AI.md PART 12
	validateConfig(cfg)
	logConfigSources(dropins, overrides, cfg.Server.Mode == "debug")

	if cfg.Server.Database.SQLite.Dir == "" || os.Getenv("DATABASE_DIR") != "" {
		cfg.Server.Database.SQLite.Dir = dbDir
	}

	applyRuntimeEnvironmentOverrides(cfg)
	validateUpdateChannel(&cfg.Server.Update, DefaultAppConfig().Server.Update)
	validateRestoreConfig(&cfg.Server.Restore, DefaultRestoreConfig())

	return cfg, configPath, nil
}

func clearGeneratedSecretDefaults(cfg *AppConfig) {
	if cfg == nil {
		return
	}
	cfg.Server.Token = ""
	cfg.Server.Security.EncryptionKey = ""
	cfg.Server.Security.EncryptionKeyVersion = 0
	cfg.Server.Metrics.Auth.Tokens = MetricsTokensConfig{}
}

// PeekServerListenConfig reads listen settings from the same merged server.yml
// and yaml.d configuration used by the full loader. It is safe for early
// startup address selection and intentionally returns empty values on error.
func PeekServerListenConfig(configPath string) (address, port string) {
	address, port, _ = PeekServerStartupConfig(configPath)
	return address, port
}

// PeekServerStartupConfig reads the listen settings and daemonize flag from
// the merged server configuration for startup decisions made before loading
// the complete application config.
func PeekServerStartupConfig(configPath string) (address, port string, daemonize bool) {
	data, _, _, err := loadMergedConfigFiles(configPath, false)
	if err != nil {
		return "", "", false
	}
	var document yaml.Node
	if yaml.Unmarshal(data, &document) != nil || len(document.Content) == 0 {
		return "", "", false
	}
	lookup := func(node *yaml.Node, key string) *yaml.Node {
		if node == nil || node.Kind != yaml.MappingNode {
			return nil
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return node.Content[i+1]
			}
		}
		return nil
	}
	serverNode := lookup(document.Content[0], "server")
	if serverNode == nil || serverNode.Kind != yaml.MappingNode {
		return "", "", false
	}
	if value := lookup(serverNode, "address"); value != nil && value.Kind == yaml.ScalarNode {
		address = value.Value
	}
	if value := lookup(serverNode, "port"); value != nil && value.Kind == yaml.ScalarNode {
		port = value.Value
	}
	if value := lookup(serverNode, "daemonize"); value != nil && value.Kind == yaml.ScalarNode {
		daemonize = strings.EqualFold(value.Value, "true")
	}
	return address, port, daemonize
}

func loadMergedConfig(configPath string) ([]byte, []string, []string, error) {
	return loadMergedConfigFiles(configPath, true)
}

func loadMergedConfigFiles(configPath string, includeSecrets bool) ([]byte, []string, []string, error) {
	baseInfo, err := os.Stat(configPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("inspect %s: %w", configPath, err)
	}
	if !baseInfo.Mode().IsRegular() {
		return nil, nil, nil, fmt.Errorf("%s must be a regular file", configPath)
	}
	if baseInfo.Mode().Perm() != 0600 {
		if err := os.Chmod(configPath, 0600); err != nil {
			return nil, nil, nil, fmt.Errorf("secure %s: %w", configPath, err)
		}
	}
	baseData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read %s: %w", configPath, err)
	}
	var merged yaml.Node
	if err := yaml.Unmarshal(baseData, &merged); err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", configPath, err)
	}
	baseInfo, statErr := os.Stat(configPath)
	if statErr != nil {
		return nil, nil, nil, fmt.Errorf("inspect %s: %w", configPath, statErr)
	}
	if len(merged.Content) > 0 && yamlNodeHasSecrets(merged.Content[0]) && baseInfo.Mode().Perm()&0077 != 0 {
		return nil, nil, nil, fmt.Errorf("%s contains secrets and must have mode 0600", configPath)
	}
	if len(merged.Content) == 0 {
		return nil, nil, nil, fmt.Errorf("%s: empty YAML document", configPath)
	}
	if err := validateYAMLNode(merged.Content[0], configPath, ""); err != nil {
		return nil, nil, nil, err
	}
	dropDir := filepath.Join(filepath.Dir(configPath), "yaml.d")
	dropInfo, err := os.Lstat(dropDir)
	if errors.Is(err, os.ErrNotExist) {
		if includeSecrets {
			if err := mergeSecretsFile(&merged, filepath.Join(filepath.Dir(configPath), "secrets.yml")); err != nil {
				return nil, nil, nil, err
			}
		}
		encoded, marshalErr := marshalYAML(&merged)
		return encoded, nil, nil, marshalErr
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("inspect %s: %w", dropDir, err)
	}
	if dropInfo.Mode()&os.ModeSymlink != 0 || !dropInfo.IsDir() {
		return nil, nil, nil, fmt.Errorf("%s must be a real directory", dropDir)
	}
	if dropInfo.Mode().Perm() != 0700 {
		if err := os.Chmod(dropDir, 0700); err != nil {
			return nil, nil, nil, fmt.Errorf("secure %s: %w", dropDir, err)
		}
	}
	entries, err := os.ReadDir(dropDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read %s: %w", dropDir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var loaded []string
	overriddenBy := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) || strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".orig") {
			continue
		}
		file := filepath.Join(dropDir, name)
		info, infoErr := os.Lstat(file)
		if infoErr != nil {
			return nil, nil, nil, fmt.Errorf("inspect %s: %w", file, infoErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, evalErr := filepath.EvalSymlinks(file)
			if evalErr != nil {
				return nil, nil, nil, fmt.Errorf("resolve %s: %w", file, evalErr)
			}
			rel, relErr := filepath.Rel(dropDir, target)
			if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, nil, nil, fmt.Errorf("%s symlink target must stay inside yaml.d", file)
			}
			info, infoErr = os.Stat(file)
			if infoErr != nil {
				return nil, nil, nil, fmt.Errorf("inspect %s target: %w", file, infoErr)
			}
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Mode().Perm() != 0600 {
			if chmodErr := os.Chmod(file, 0600); chmodErr != nil {
				log.Printf("config: ignoring %s because mode 0600 could not be set: %v", file, chmodErr)
				continue
			}
			info, infoErr = os.Stat(file)
			if infoErr != nil {
				log.Printf("config: ignoring %s because its mode could not be verified: %v", file, infoErr)
				continue
			}
		}
		if info.Mode().Perm()&0022 != 0 {
			return nil, nil, nil, fmt.Errorf("%s is group or world writable", file)
		}
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, nil, nil, fmt.Errorf("read %s: %w", file, readErr)
		}
		var overlay yaml.Node
		if decodeErr := yaml.Unmarshal(data, &overlay); decodeErr != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", file, decodeErr)
		}
		if len(overlay.Content) == 0 {
			return nil, nil, nil, fmt.Errorf("%s: empty YAML document", file)
		}
		if validateErr := validateYAMLNode(overlay.Content[0], file, ""); validateErr != nil {
			return nil, nil, nil, validateErr
		}
		if yamlNodeHasSecrets(overlay.Content[0]) && info.Mode().Perm()&0077 != 0 {
			return nil, nil, nil, fmt.Errorf("%s contains secrets and must have mode 0600", file)
		}
		var fileOverrides []string
		collectYAMLOverrides(merged.Content[0], overlay.Content[0], "", &fileOverrides)
		for _, key := range fileOverrides {
			overriddenBy[key] = filepath.Base(file)
		}
		mergeYAMLNodes(merged.Content[0], overlay.Content[0])
		loaded = append(loaded, file)
	}
	if includeSecrets {
		if err := mergeSecretsFile(&merged, filepath.Join(filepath.Dir(configPath), "secrets.yml")); err != nil {
			return nil, nil, nil, err
		}
	}
	encoded, err := marshalYAML(&merged)
	keys := make([]string, 0, len(overriddenBy))
	for key := range overriddenBy {
		keys = append(keys, key+" ("+overriddenBy[key]+")")
	}
	sort.Strings(keys)
	return encoded, loaded, keys, err
}

func mergeSecretsFile(merged *yaml.Node, secretsPath string) error {
	linkInfo, linkErr := os.Lstat(secretsPath)
	if errors.Is(linkErr, os.ErrNotExist) {
		return nil
	}
	if linkErr != nil {
		return fmt.Errorf("inspect secrets.yml: %w", linkErr)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() {
		return fmt.Errorf("secrets.yml must be a regular file")
	}
	data, err := os.ReadFile(secretsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read secrets.yml: %w", err)
	}
	info, err := os.Stat(secretsPath)
	if err != nil {
		return fmt.Errorf("inspect secrets.yml: %w", err)
	}
	if info.Mode().Perm() != 0600 {
		if chmodErr := os.Chmod(secretsPath, 0600); chmodErr != nil {
			return fmt.Errorf("secrets.yml must have mode 0600: %w", chmodErr)
		}
	}
	var overlay yaml.Node
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return fmt.Errorf("parse secrets.yml: %w", err)
	}
	if len(overlay.Content) == 0 {
		return fmt.Errorf("secrets.yml: empty YAML document")
	}
	if err := validateYAMLNode(overlay.Content[0], secretsPath, ""); err != nil {
		return err
	}
	if merged == nil || len(merged.Content) == 0 {
		return fmt.Errorf("merged configuration is empty")
	}
	mergeYAMLNodes(merged.Content[0], overlay.Content[0])
	return nil
}

func yamlNodeHasSecrets(node *yaml.Node) bool {
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := strings.ToLower(node.Content[i].Value)
			if key == "dns_credentials" {
				credentials := node.Content[i+1]
				if credentials.Kind == yaml.MappingNode {
					for j := 1; j < len(credentials.Content); j += 2 {
						if strings.HasPrefix(credentials.Content[j].Value, "enc:v1:") {
							return true
						}
					}
				}
				continue
			}
			if strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "password") || strings.Contains(key, "private_key") || strings.Contains(key, "encryption_key") || strings.Contains(key, "api_key") {
				return true
			}
			if yamlNodeHasSecrets(node.Content[i+1]) {
				return true
			}
		}
	}
	for _, child := range node.Content {
		if child.Kind == yaml.SequenceNode && yamlNodeHasSecrets(child) {
			return true
		}
		if child.Kind == yaml.MappingNode && node.Kind != yaml.MappingNode && yamlNodeHasSecrets(child) {
			return true
		}
	}
	return false
}

func validateYAMLNode(node *yaml.Node, file, key string) error {
	if node.Tag == "!!null" {
		return fmt.Errorf("%s:%d: explicit null is not allowed at %s", file, node.Line, key)
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			name := node.Content[i].Value
			childKey := name
			if key != "" {
				childKey = key + "." + name
			}
			if seen[name] {
				return fmt.Errorf("%s:%d: duplicate key %s", file, node.Content[i].Line, childKey)
			}
			seen[name] = true
			if err := validateYAMLNode(node.Content[i+1], file, childKey); err != nil {
				return err
			}
		}
	}
	for _, child := range node.Content {
		if child.Kind != yaml.MappingNode && child.Kind != yaml.SequenceNode {
			continue
		}
		if err := validateYAMLNode(child, file, key); err != nil {
			return err
		}
	}
	return nil
}

func mergeYAMLNodes(base, overlay *yaml.Node) {
	if base.Kind != yaml.MappingNode || overlay.Kind != yaml.MappingNode {
		*base = *overlay
		return
	}
	for i := 0; i+1 < len(overlay.Content); i += 2 {
		key, value := overlay.Content[i], overlay.Content[i+1]
		found := -1
		for j := 0; j+1 < len(base.Content); j += 2 {
			if base.Content[j].Value == key.Value {
				found = j + 1
				break
			}
		}
		if found < 0 {
			base.Content = append(base.Content, key, value)
		} else if base.Content[found].Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			mergeYAMLNodes(base.Content[found], value)
		} else {
			base.Content[found] = value
		}
	}
}

func collectYAMLOverrides(base, overlay *yaml.Node, prefix string, output *[]string) {
	if base.Kind != yaml.MappingNode || overlay.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(overlay.Content); i += 2 {
		key, value := overlay.Content[i], overlay.Content[i+1]
		path := key.Value
		if prefix != "" {
			path = prefix + "." + path
		}
		for j := 0; j+1 < len(base.Content); j += 2 {
			if base.Content[j].Value == key.Value {
				if base.Content[j+1].Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
					collectYAMLOverrides(base.Content[j+1], value, path, output)
				} else {
					*output = append(*output, path)
				}
				break
			}
		}
	}
}

// validateConfig validates all config values, replacing invalid with defaults per AI.md PART 12
// Rule: If config setting is invalid, warn and replace with default. Never fail startup.
func validateConfig(cfg *AppConfig) {
	defaults := DefaultAppConfig()
	validateTorConfig(&cfg.Server.Tor, defaults.Server.Tor)
	validateI2PConfig(&cfg.Server.I2P, defaults.Server.I2P)
	validateUpdateChannel(&cfg.Server.Update, defaults.Server.Update)
	validatePGPKeyservers(cfg)
	if err := ValidateTracking(&cfg.Server.Tracking); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid tracking configuration: %v; analytics disabled\n", err)
		cfg.Server.Tracking = defaults.Server.Tracking
	}
	validateContactCaptcha(&cfg.Server.Pages.Contact, defaults.Server.Pages.Contact)
	if cfg.Server.SEO.Feed.MaxItems < 1 || cfg.Server.SEO.Feed.MaxItems > 200 {
		fmt.Fprintf(os.Stderr, "Warning: invalid seo.feed.max_items %d, using default %d\n", cfg.Server.SEO.Feed.MaxItems, defaults.Server.SEO.Feed.MaxItems)
		cfg.Server.SEO.Feed.MaxItems = defaults.Server.SEO.Feed.MaxItems
	}
	validateSEOOptions(&cfg.Server.SEO, defaults.Server.SEO)

	// Validate port (must be valid or use random per PART 8/12)
	if cfg.Server.Port != "" {
		// Parse port(s) - could be "8080" or "8080,8443"
		ports := strings.Split(cfg.Server.Port, ",")
		for _, p := range ports {
			port := strings.TrimSpace(p)
			if port == "" {
				continue
			}
			portNum, err := strconv.Atoi(port)
			if err != nil || portNum < 1 || portNum > 65535 {
				randomPort := findUnusedPort()
				fmt.Fprintf(os.Stderr, "Warning: invalid port %s, using random port %d\n", port, randomPort)
				cfg.Server.Port = fmt.Sprintf("%d", randomPort)
				break
			}
		}
	}

	// Validate mode per AI.md PART 6 Mode Shortcuts.
	// Accepted: production (prod), development (dev, devel), debug.
	// Shortcuts normalize to the canonical name; "debug" is preserved because
	// it also defaults the debug flag on (PART 6).
	if cfg.Server.Mode != "" {
		switch strings.ToLower(cfg.Server.Mode) {
		case "prod", "production":
			cfg.Server.Mode = "production"
		case "dev", "devel", "development":
			cfg.Server.Mode = "development"
		case "debug":
			cfg.Server.Mode = "debug"
		default:
			fmt.Fprintf(os.Stderr, "Warning: invalid mode %q, using default %q\n", cfg.Server.Mode, defaults.Server.Mode)
			cfg.Server.Mode = defaults.Server.Mode
		}
	}

	// Validate API version (must be non-empty, lowercase "v" + digits, per PART 14)
	if !apiVersionPattern.MatchString(cfg.Server.APIVersion) {
		fmt.Fprintf(os.Stderr, "Warning: invalid api_version %q, using default %q\n", cfg.Server.APIVersion, defaults.Server.APIVersion)
		cfg.Server.APIVersion = defaults.Server.APIVersion
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
		fmt.Fprintf(os.Stderr, "Warning: invalid session.same_site %q, using default 'strict'\n", cfg.Server.Session.SameSite)
		cfg.Server.Session.SameSite = "strict"
	}

	// Validate compression level (1-9)
	if cfg.Server.Compression.Level < 0 || cfg.Server.Compression.Level > 9 {
		fmt.Fprintf(os.Stderr, "Warning: invalid compression.level %d, using default 5\n", cfg.Server.Compression.Level)
		cfg.Server.Compression.Level = 5
	}

	validateCORS(&cfg.Server.CORS, defaults.Server.CORS)
	for _, field := range sanitizeSEOVerification(&cfg.Server.SEO.Verification) {
		fmt.Fprintf(os.Stderr, "Warning: invalid %s value; omitting it from rendered SEO metadata\n", field)
	}
	if cfg.Server.SEO.Sitemap.MaxURLs < 1 || cfg.Server.SEO.Sitemap.MaxURLs > 50000 {
		fmt.Fprintf(os.Stderr, "Warning: invalid seo.sitemap.max_urls %d, using default %d\n", cfg.Server.SEO.Sitemap.MaxURLs, defaults.Server.SEO.Sitemap.MaxURLs)
		cfg.Server.SEO.Sitemap.MaxURLs = defaults.Server.SEO.Sitemap.MaxURLs
	}

	// Enforce audit log format as JSON only per AI.md PART 11
	// "audit: format: json only (text not supported for audit - must be machine-parseable)"
	if cfg.Server.Logs.Audit.Format != "" && cfg.Server.Logs.Audit.Format != "json" {
		fmt.Fprintf(os.Stderr, "Warning: audit log format must be 'json', ignoring %q\n", cfg.Server.Logs.Audit.Format)
		cfg.Server.Logs.Audit.Format = "json"
	}

	// Validate backup retention settings (warn, don't error - server must start) per AI.md PART 21
	validateBackupRetention(cfg)
	if cfg.Server.Security.Reports.RetentionDays < 1 {
		fmt.Fprintf(os.Stderr, "Warning: invalid server.security.reports.retention_days %d, using default %d\n", cfg.Server.Security.Reports.RetentionDays, defaults.Server.Security.Reports.RetentionDays)
		cfg.Server.Security.Reports.RetentionDays = defaults.Server.Security.Reports.RetentionDays
	}
	validateMaintenanceConfig(&cfg.Server.Maintenance, defaults.Server.Maintenance)
	validateRestoreConfig(&cfg.Server.Restore, defaults.Server.Restore)
}

func validatePGPKeyservers(cfg *AppConfig) {
	devMode := cfg.Server.Mode == "development" || cfg.Server.Mode == "debug"
	for _, host := range cfg.Web.Security.Keyservers {
		if host != strings.TrimSpace(host) || strings.ContainsAny(host, ":/@?#") || !IsValidHost(host, devMode, paths.ProjectName) {
			defaults := DefaultAppConfig().Web.Security.Keyservers
			fmt.Fprintf(os.Stderr, "Warning: invalid web.security.keyservers host %q, using defaults\n", host)
			cfg.Web.Security.Keyservers = defaults
			return
		}
	}
}

func validateMaintenanceConfig(cfg *MaintenanceConfig, defaults MaintenanceConfig) {
	interval, err := time.ParseDuration(cfg.SelfHealing.RetryInterval)
	if err != nil || interval < time.Second {
		fmt.Fprintf(os.Stderr, "Warning: invalid maintenance.self_healing.retry_interval %q, using default %s\n", cfg.SelfHealing.RetryInterval, defaults.SelfHealing.RetryInterval)
		cfg.SelfHealing.RetryInterval = defaults.SelfHealing.RetryInterval
	}
	if cfg.Cleanup.DiskThreshold < 1 || cfg.Cleanup.DiskThreshold > 99 {
		fmt.Fprintf(os.Stderr, "Warning: invalid maintenance.cleanup.disk_threshold %d, using default %d\n", cfg.Cleanup.DiskThreshold, defaults.Cleanup.DiskThreshold)
		cfg.Cleanup.DiskThreshold = defaults.Cleanup.DiskThreshold
	}
	if cfg.Cleanup.LogRetentionDays < 1 {
		fmt.Fprintf(os.Stderr, "Warning: invalid maintenance.cleanup.log_retention_days %d, using default %d\n", cfg.Cleanup.LogRetentionDays, defaults.Cleanup.LogRetentionDays)
		cfg.Cleanup.LogRetentionDays = defaults.Cleanup.LogRetentionDays
	}
}

func validateTorConfig(cfg *TorConfig, defaults TorConfig) {
	if cfg.StartupTimeout < 10 || cfg.StartupTimeout > 300 {
		fmt.Fprintf(os.Stderr, "Warning: invalid tor.startup_timeout %d, using default %d\n", cfg.StartupTimeout, defaults.StartupTimeout)
		cfg.StartupTimeout = defaults.StartupTimeout
	}
}

func validateUpdateChannel(cfg *UpdateConfig, defaults UpdateConfig) {
	if channel, valid := NormalizeUpdateChannel(cfg.Branch); valid {
		cfg.Branch = channel
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: invalid update.branch %q, using default %q\n", cfg.Branch, defaults.Branch)
	cfg.Branch = defaults.Branch
}

func validateRestoreConfig(cfg *RestoreConfig, defaults RestoreConfig) {
	maxInt64 := uint64(^uint64(0) >> 1)
	for _, field := range []struct {
		name  string
		value *string
		def   string
	}{{"max_archive_size", &cfg.MaxArchiveSize, defaults.MaxArchiveSize}, {"max_expanded_size", &cfg.MaxExpandedSize, defaults.MaxExpandedSize}, {"max_file_size", &cfg.MaxFileSize, defaults.MaxFileSize}} {
		bytes, err := ParseSizeBytes(*field.value)
		invalid := err != nil || bytes == 0
		if !invalid && field.name == "max_archive_size" && bytes > maxInt64-(1<<20) {
			invalid = true
		}
		if !invalid && field.name == "max_expanded_size" && bytes >= maxInt64 {
			invalid = true
		}
		if invalid {
			fmt.Fprintf(os.Stderr, "Warning: invalid restore.%s %q, using default %q\n", field.name, *field.value, field.def)
			*field.value = field.def
		}
	}
	if cfg.MaxFiles < 1 {
		fmt.Fprintf(os.Stderr, "Warning: invalid restore.max_files %d, using default %d\n", cfg.MaxFiles, defaults.MaxFiles)
		cfg.MaxFiles = defaults.MaxFiles
	}
	if cfg.MaxRatio < 1 {
		fmt.Fprintf(os.Stderr, "Warning: invalid restore.max_ratio %d, using default %d\n", cfg.MaxRatio, defaults.MaxRatio)
		cfg.MaxRatio = defaults.MaxRatio
	}
}

// ParseSizeBytes parses a positive byte count with optional K/KB, M/MB, G/GB,
// or T/TB suffixes using binary units.
func ParseSizeBytes(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	upper := strings.ToUpper(value)
	multiplier := uint64(1)
	for _, unit := range []struct {
		suffix string
		factor uint64
	}{{"TB", 1 << 40}, {"T", 1 << 40}, {"GB", 1 << 30}, {"G", 1 << 30}, {"MB", 1 << 20}, {"M", 1 << 20}, {"KB", 1 << 10}, {"K", 1 << 10}} {
		if strings.HasSuffix(upper, unit.suffix) {
			upper = strings.TrimSpace(upper[:len(upper)-len(unit.suffix)])
			multiplier = unit.factor
			break
		}
	}
	number, err := strconv.ParseFloat(upper, 64)
	if err != nil || number <= 0 || math.IsInf(number, 0) || math.IsNaN(number) || number > float64(^uint64(0))/float64(multiplier) {
		return 0, fmt.Errorf("invalid positive byte size %q", value)
	}
	bytes := number * float64(multiplier)
	if bytes < 1 || bytes >= math.Exp2(63) {
		return 0, fmt.Errorf("byte size %q is outside the supported range", value)
	}
	return uint64(bytes), nil
}

// validateCORS normalizes malformed CORS settings to the documented defaults.
func validateI2PConfig(cfg *I2PConfig, defaults I2PConfig) {
	if cfg.SAMAddress == "" {
		cfg.SAMAddress = defaults.SAMAddress
	}
	if cfg.VirtualPort < 1 || cfg.VirtualPort > 65535 {
		fmt.Fprintf(os.Stderr, "Warning: invalid i2p.virtual_port %d, using default %d\n", cfg.VirtualPort, defaults.VirtualPort)
		cfg.VirtualPort = defaults.VirtualPort
	}
	if cfg.InboundLength < 0 || cfg.InboundLength > 7 {
		fmt.Fprintf(os.Stderr, "Warning: invalid i2p.inbound_length %d, using default %d\n", cfg.InboundLength, defaults.InboundLength)
		cfg.InboundLength = defaults.InboundLength
	}
	if cfg.OutboundLength < 0 || cfg.OutboundLength > 7 {
		fmt.Fprintf(os.Stderr, "Warning: invalid i2p.outbound_length %d, using default %d\n", cfg.OutboundLength, defaults.OutboundLength)
		cfg.OutboundLength = defaults.OutboundLength
	}
	if cfg.InboundQuantity < 1 || cfg.InboundQuantity > 16 {
		fmt.Fprintf(os.Stderr, "Warning: invalid i2p.inbound_quantity %d, using default %d\n", cfg.InboundQuantity, defaults.InboundQuantity)
		cfg.InboundQuantity = defaults.InboundQuantity
	}
	if cfg.OutboundQuantity < 1 || cfg.OutboundQuantity > 16 {
		fmt.Fprintf(os.Stderr, "Warning: invalid i2p.outbound_quantity %d, using default %d\n", cfg.OutboundQuantity, defaults.OutboundQuantity)
		cfg.OutboundQuantity = defaults.OutboundQuantity
	}
	if cfg.SignatureType != 7 {
		fmt.Fprintf(os.Stderr, "Warning: unsupported i2p.signature_type %d, using default %d\n", cfg.SignatureType, defaults.SignatureType)
		cfg.SignatureType = defaults.SignatureType
	}
	if duration, err := time.ParseDuration(cfg.BootstrapTimeout); err != nil || duration < 30*time.Second || duration > 10*time.Minute {
		fmt.Fprintf(os.Stderr, "Warning: invalid i2p.bootstrap_timeout %q, using default %q\n", cfg.BootstrapTimeout, defaults.BootstrapTimeout)
		cfg.BootstrapTimeout = defaults.BootstrapTimeout
	}
}

func validateCORS(corsCfg *CORSConfig, defaults CORSConfig) {
	if corsCfg.MaxAge <= 0 {
		corsCfg.MaxAge = defaults.MaxAge
	}
	for _, origin := range corsCfg.AllowedOrigins {
		if origin == "" || origin == "*" {
			continue
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			fmt.Fprintf(os.Stderr, "Warning: invalid cors.allowed_origins entry %q, using default origins\n", origin)
			corsCfg.AllowedOrigins = append([]string(nil), defaults.AllowedOrigins...)
			return
		}
	}
}

// validateBackupRetention validates cfg.Server.Backup.Retention per AI.md PART 21.
// Invalid values (negative, or max_backups == 0) are reset to their defaults with a WARN.
// Values above the recommended threshold are accepted but still generate a WARN.
func validateBackupRetention(cfg *AppConfig) {
	r := &cfg.Server.Backup.Retention

	if r.MaxBackups < 0 {
		fmt.Fprintf(os.Stderr, "WARN: max_backups: %d invalid, using default 1\n", r.MaxBackups)
		r.MaxBackups = 1
	} else if r.MaxBackups == 0 {
		fmt.Fprintf(os.Stderr, "WARN: max_backups: 0 invalid, using default 1\n")
		r.MaxBackups = 1
	} else if r.MaxBackups > 7 {
		fmt.Fprintf(os.Stderr, "WARN: max_backups: %d exceeds recommended 7 (30 days of daily backups)\n", r.MaxBackups)
	}

	if r.KeepWeekly < 0 {
		fmt.Fprintf(os.Stderr, "WARN: keep_weekly: %d invalid, using default 0\n", r.KeepWeekly)
		r.KeepWeekly = 0
	} else if r.KeepWeekly > 8 {
		fmt.Fprintf(os.Stderr, "WARN: keep_weekly: %d exceeds recommended 8 (2 months of weekly backups)\n", r.KeepWeekly)
	}

	if r.KeepMonthly < 0 {
		fmt.Fprintf(os.Stderr, "WARN: keep_monthly: %d invalid, using default 0\n", r.KeepMonthly)
		r.KeepMonthly = 0
	} else if r.KeepMonthly > 12 {
		fmt.Fprintf(os.Stderr, "WARN: keep_monthly: %d exceeds recommended 12 (2 years of monthly backups)\n", r.KeepMonthly)
	}

	if r.KeepYearly < 0 {
		fmt.Fprintf(os.Stderr, "WARN: keep_yearly: %d invalid, using default 0\n", r.KeepYearly)
		r.KeepYearly = 0
	} else if r.KeepYearly > 2 {
		fmt.Fprintf(os.Stderr, "WARN: keep_yearly: %d exceeds recommended 2 (2 years of yearly backups)\n", r.KeepYearly)
	}
}

// SaveAppConfig saves configuration to file
func encryptDNSCredential(value, key string) (string, error) {
	if strings.HasPrefix(value, "enc:v1:") {
		return value, nil
	}
	keyBytes, err := hex.DecodeString(key)
	if err != nil || len(keyBytes) != 32 {
		return "", errors.New("server.security.encryption_key must be 32-byte hex")
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(value), nil)
	return "enc:v1:" + base64.StdEncoding.EncodeToString(sealed), nil
}

func decryptDNSCredential(value, key string) (string, error) {
	if !strings.HasPrefix(value, "enc:v1:") {
		return value, nil
	}
	keyBytes, err := hex.DecodeString(key)
	if err != nil || len(keyBytes) != 32 {
		return "", errors.New("server.security.encryption_key must be 32-byte hex")
	}
	sealed, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "enc:v1:"))
	if err != nil {
		return "", errors.New("invalid encrypted credential")
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < aead.NonceSize() {
		return "", errors.New("invalid encrypted credential")
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], nil)
	if err != nil {
		return "", errors.New("cannot decrypt credential with configured encryption key")
	}
	return string(plain), nil
}

func persistEncryptedCredentials(files []string, key string) error {
	if len(files) == 0 {
		return nil
	}
	for index, file := range files {
		writePath := file
		var err error
		if index > 0 {
			writePath, err = safeConfigDropinTarget(file)
		} else if resolved, resolveErr := filepath.EvalSymlinks(file); resolveErr == nil {
			writePath = resolved
		} else {
			err = resolveErr
		}
		if err != nil {
			return err
		}
		if writePath == "" {
			continue
		}
		data, err := os.ReadFile(writePath)
		if err != nil {
			return err
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
			continue
		}
		root := doc.Content[0]
		server := yamlMapValue(root, "server")
		ssl := yamlMapValue(server, "ssl")
		le := yamlMapValue(ssl, "letsencrypt")
		credentials := yamlMapValue(le, "dns_credentials")
		changed := false
		if credentials != nil && credentials.Kind == yaml.MappingNode {
			for i := 1; i < len(credentials.Content); i += 2 {
				value := credentials.Content[i]
				if strings.HasPrefix(value.Value, "enc:v1:") {
					continue
				}
				encrypted, err := encryptDNSCredential(value.Value, key)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
				value.Value, value.Tag, changed = encrypted, "!!str", true
			}
		}
		backup := yamlMapValue(server, "backup")
		password := yamlMapValue(backup, "encryption_password")
		if password != nil && password.Kind == yaml.ScalarNode && password.Value == "********" {
			password.Value, password.Tag, changed = "", "!!str", true
		} else if password != nil && password.Kind == yaml.ScalarNode && password.Value != "" && !strings.HasPrefix(password.Value, "enc:v1:") {
			encrypted, err := encryptDNSCredential(password.Value, key)
			if err != nil {
				return fmt.Errorf("%s: encrypt backup password: %w", file, err)
			}
			password.Value, password.Tag, changed = encrypted, "!!str", true
		}
		if !changed {
			continue
		}
		encoded, err := marshalYAML(&doc)
		if err != nil {
			return err
		}
		if err := atomicConfigWrite(writePath, encoded); err != nil {
			return err
		}
	}
	return nil
}

func safeConfigDropinTarget(file string) (string, error) {
	info, err := os.Lstat(file)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		if !info.Mode().IsRegular() {
			return "", nil
		}
		return file, nil
	}
	target, err := filepath.EvalSymlinks(file)
	if err != nil {
		return "", err
	}
	base := filepath.Dir(file)
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s symlink target must stay inside yaml.d", file)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if !targetInfo.Mode().IsRegular() {
		return "", nil
	}
	return target, nil
}

func yamlMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func atomicConfigWrite(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm() == 0600 {
		if existing, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(existing, data) {
			return nil
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func marshalYAML(value interface{}) ([]byte, error) {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func sparseLogsConfig(logs LogsConfig) LogsConfig {
	clearDerivedFilename := func(filename *string, defaultName string) {
		if filepath.Clean(*filename) == defaultName {
			*filename = ""
		}
	}
	clearDerivedFilename(&logs.Debug.Filename, "debug.log")
	clearDerivedFilename(&logs.Access.Filename, "access.log")
	clearDerivedFilename(&logs.Server.Filename, "server.log")
	clearDerivedFilename(&logs.Error.Filename, "error.log")
	clearDerivedFilename(&logs.Audit.Filename, "audit.log")
	clearDerivedFilename(&logs.Security.Filename, "security.log")
	clearDerivedFilename(&logs.Auth.Filename, "auth.log")
	clearDerivedFilename(&logs.App.Filename, "vidveil.log")
	if logs.Debug.Custom == logs.Defaults.Custom {
		logs.Debug.Custom = ""
	}
	if logs.Access.Custom == logs.Defaults.Custom {
		logs.Access.Custom = ""
	}
	if logs.Server.Custom == logs.Defaults.Custom {
		logs.Server.Custom = ""
	}
	if logs.Error.Custom == logs.Defaults.Custom {
		logs.Error.Custom = ""
	}
	if logs.Audit.Custom == logs.Defaults.Custom {
		logs.Audit.Custom = ""
	}
	if logs.Security.Custom == logs.Defaults.Custom {
		logs.Security.Custom = ""
	}
	if logs.Auth.Custom == logs.Defaults.Custom {
		logs.Auth.Custom = ""
	}
	if logs.App.Custom == logs.Defaults.Custom {
		logs.App.Custom = ""
	}
	if logs.Debug.Keep == logs.Defaults.Keep {
		logs.Debug.Keep = ""
	}
	if logs.Access.Keep == logs.Defaults.Keep {
		logs.Access.Keep = ""
	}
	if logs.Server.Keep == logs.Defaults.Keep {
		logs.Server.Keep = ""
	}
	if logs.Error.Keep == logs.Defaults.Keep {
		logs.Error.Keep = ""
	}
	if logs.Audit.Keep == logs.Defaults.Keep {
		logs.Audit.Keep = ""
	}
	if logs.Security.Keep == logs.Defaults.Keep {
		logs.Security.Keep = ""
	}
	if logs.Auth.Keep == logs.Defaults.Keep {
		logs.Auth.Keep = ""
	}
	if logs.App.Keep == logs.Defaults.Keep {
		logs.App.Keep = ""
	}
	if logs.Debug.Rotate == logs.Defaults.Rotate {
		logs.Debug.Rotate = ""
	}
	if logs.Access.Rotate == logs.Defaults.Rotate {
		logs.Access.Rotate = ""
	}
	if logs.Server.Rotate == logs.Defaults.Rotate {
		logs.Server.Rotate = ""
	}
	if logs.Error.Rotate == logs.Defaults.Rotate {
		logs.Error.Rotate = ""
	}
	if logs.Audit.Rotate == logs.Defaults.Rotate {
		logs.Audit.Rotate = ""
	}
	if logs.Security.Rotate == logs.Defaults.Rotate {
		logs.Security.Rotate = ""
	}
	if logs.Auth.Rotate == logs.Defaults.Rotate {
		logs.Auth.Rotate = ""
	}
	if logs.App.Rotate == logs.Defaults.Rotate {
		logs.App.Rotate = ""
	}
	return logs
}

func SaveAppConfig(cfg *AppConfig, path string) (saveErr error) {
	configToSave := *cfg
	if (len(configToSave.Server.SSL.LetsEncrypt.DNSCredentials) > 0 || configToSave.Server.Backup.EncryptionPassword != "") && configToSave.Server.Security.EncryptionKey == "" {
		configToSave.Server.Security.EncryptionKey = generateToken(32)
		configToSave.Server.Security.EncryptionKeyVersion = 1
		cfg.Server.Security.EncryptionKey = configToSave.Server.Security.EncryptionKey
		cfg.Server.Security.EncryptionKeyVersion = 1
		cfg.GeneratedSecretKeys = append(cfg.GeneratedSecretKeys, "server.security.encryption_key")
	}
	configToSave.Server.SSL.LetsEncrypt.DNSCredentials = make(map[string]string, len(cfg.Server.SSL.LetsEncrypt.DNSCredentials))
	for name, value := range cfg.Server.SSL.LetsEncrypt.DNSCredentials {
		encrypted, encryptErr := encryptDNSCredential(value, cfg.Server.Security.EncryptionKey)
		if encryptErr != nil {
			return fmt.Errorf("encrypt DNS credential %s: %w", name, encryptErr)
		}
		configToSave.Server.SSL.LetsEncrypt.DNSCredentials[name] = encrypted
	}
	if configToSave.Server.Backup.EncryptionPassword != "" {
		encrypted, encryptErr := encryptDNSCredential(configToSave.Server.Backup.EncryptionPassword, cfg.Server.Security.EncryptionKey)
		if encryptErr != nil {
			return fmt.Errorf("encrypt backup password: %w", encryptErr)
		}
		configToSave.Server.Backup.EncryptionPassword = encrypted
	}
	configToSave.Server.Logs = sparseLogsConfig(configToSave.Server.Logs)
	return saveConfigLayout(&configToSave, path)
}

func saveInitialServerConfig(cfg *AppConfig, path string) error {
	return SaveAppConfig(cfg, path)
}

func saveConfigLayout(cfg *AppConfig, path string) error {
	encoded, err := marshalYAML(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	root, err := decodeConfigMap(encoded)
	if err != nil {
		return fmt.Errorf("failed to prepare config layout: %w", err)
	}
	secrets := extractSecretSettings(root)
	parts, _ := partitionConfig(root, "server.yml")
	defaults := DefaultAppConfig()
	defaults.Server.Logs = sparseLogsConfig(defaults.Server.Logs)
	defaultData, err := marshalYAML(defaults)
	if err != nil {
		return fmt.Errorf("prepare default config layout: %w", err)
	}
	defaultRoot, err := decodeConfigMap(defaultData)
	if err != nil {
		return fmt.Errorf("decode default config layout: %w", err)
	}
	extractSecretSettings(defaultRoot)
	defaultParts, _ := partitionConfig(defaultRoot, "server.yml")
	configDir := filepath.Dir(path)
	dropDir := filepath.Join(configDir, "yaml.d")
	areaFiles := make(map[string][]byte)
	for _, name := range orderedAreaFiles {
		area := parts[name]
		if len(area) == 0 {
			continue
		}
		data, err := marshalYAML(area)
		if err != nil {
			return fmt.Errorf("encode %s: %w", name, err)
		}
		areaPath := filepath.Join(dropDir, name)
		if _, statErr := os.Lstat(areaPath); errors.Is(statErr, os.ErrNotExist) {
			defaultData, defaultErr := marshalYAML(defaultParts[name])
			if defaultErr == nil && bytes.Equal(data, defaultData) {
				continue
			}
		} else if statErr != nil {
			return fmt.Errorf("inspect %s: %w", areaPath, statErr)
		}
		areaFiles[name] = data
	}
	if len(areaFiles) > 0 {
		if err := os.MkdirAll(dropDir, 0700); err != nil {
			return fmt.Errorf("create yaml.d: %w", err)
		}
		if err := os.Chmod(dropDir, 0700); err != nil {
			return fmt.Errorf("secure yaml.d: %w", err)
		}
		for name, data := range areaFiles {
			if err := atomicConfigWrite(filepath.Join(dropDir, name), data); err != nil {
				return fmt.Errorf("write %s: %w", name, err)
			}
		}
	}
	if len(secrets) > 0 {
		secretsPath := filepath.Join(configDir, "secrets.yml")
		existing, readErr := os.ReadFile(secretsPath)
		secretsExists := readErr == nil
		if secretsExists {
			prior, decodeErr := decodeConfigMap(existing)
			if decodeErr != nil {
				return fmt.Errorf("parse secrets.yml: %w", decodeErr)
			}
			mergeConfigMap(prior, secrets)
			secrets = prior
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("read secrets.yml: %w", readErr)
		}
		if secretsExists || hasConfiguredSecret(secrets) {
			data, err := marshalYAML(secrets)
			if err != nil {
				return fmt.Errorf("encode secrets.yml: %w", err)
			}
			if err := atomicConfigWrite(secretsPath, data); err != nil {
				return fmt.Errorf("write secrets.yml: %w", err)
			}
		}
	}
	serverData, err := marshalYAML(parts["server.yml"])
	if err != nil {
		return fmt.Errorf("encode server.yml: %w", err)
	}
	if err := atomicConfigWrite(path, append([]byte(serverConfigHeader+"\n"), serverData...)); err != nil {
		return fmt.Errorf("write server.yml: %w", err)
	}
	return nil
}

func hasConfiguredSecret(value interface{}) bool {
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, child := range typed {
			if hasConfiguredSecret(child) {
				return true
			}
		}
	case string:
		return strings.TrimSpace(typed) != ""
	case int, int64, float64:
		return fmt.Sprint(typed) != "0"
	case bool:
		return typed
	case []interface{}:
		for _, child := range typed {
			if hasConfiguredSecret(child) {
				return true
			}
		}
	}
	return false
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
			if closeErr := ln.Close(); closeErr == nil {
				return port
			}
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

// isRunningInContainer detects if running in a container (tini as PID 1)
func isRunningInContainer() bool {
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

// IsDevelopmentMode returns true if the configured mode is development.
func (c *AppConfig) IsDevelopmentMode() bool {
	mode := strings.ToLower(c.Server.Mode)
	return mode == "development" || mode == "dev" || mode == "devel"
}

// IsDebugMode returns true only when the distinct debug mode is selected.
func (c *AppConfig) IsDebugMode() bool {
	return strings.EqualFold(strings.TrimSpace(c.Server.Mode), "debug")
}

// IsProductionMode returns true if running in production mode
func (c *AppConfig) IsProductionMode() bool {
	return !c.IsDevelopmentMode() && !c.IsDebugMode()
}

// current holds the process-wide active AppConfig, set once at startup via
// SetCurrent and mutated in place by the config watcher on hot-reload (see
// ConfigWatcher.reload). It exists so packages outside the request path
// that own the *AppConfig (e.g. src/server/handler) can answer "are we in
// production mode?" without threading a config pointer through every call
// site — needed by the Output Sanitization Pipeline (AI.md PART 11 stage 5:
// "In production mode, drop fields tagged dev_only").
var current atomic.Pointer[AppConfig]

// SetCurrent registers c as the process-wide active AppConfig. Call once
// after the initial config load; subsequent hot-reloads mutate the same
// pointer in place, so SetCurrent does not need to be called again.
func SetCurrent(c *AppConfig) {
	current.Store(c)
}

// IsProductionMode reports whether the process-wide active AppConfig (set
// via SetCurrent) is in production mode. Returns true (fail-safe: strip
// dev-only fields) if no config has been registered yet.
func IsProductionMode() bool {
	c := current.Load()
	if c == nil {
		return true
	}
	return c.IsProductionMode()
}

// NormalizeMode normalizes the mode string to its canonical application mode.
func NormalizeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "dev", "devel", "development":
		return "development"
	case "prod", "production", "":
		return "production"
	case "debug":
		return "debug"
	default:
		return "production"
	}
}

// AI.md PART 8: URL/FQDN Detection

// devOnlyTLDs are public-suffix labels allowed only in development mode per AI.md
// PART 8. Keys are the bare suffix as returned by publicsuffix.PublicSuffix.
var devOnlyTLDs = map[string]bool{
	"localhost": true, "test": true, "example": true, "invalid": true,
	"local": true, "lan": true, "internal": true, "home": true,
	"localdomain": true, "home.arpa": true, "intranet": true,
	"corp": true, "private": true,
}

// IsValidHost validates a host per AI.md PART 8 (Go reference implementation).
// In production, only real ICANN eTLD+1 domains are allowed (no IPs, no localhost,
// no dev TLDs, no suffix-only hosts). In development, localhost, dev TLDs, and the
// dynamic ".{projectName}" TLD are additionally allowed. Overlay-network hosts
// (.onion/.i2p/.exit) are always valid since they are app-managed, not DOMAIN-set.
func IsValidHost(host string, devMode bool, projectName string) bool {
	lower := strings.ToLower(strings.TrimSpace(host))

	// Reject empty
	if lower == "" {
		return false
	}

	// Reject IP addresses always
	if net.ParseIP(lower) != nil {
		return false
	}

	// Handle localhost - only valid in dev mode
	if lower == "localhost" {
		return devMode
	}

	// Must contain at least one dot
	if !strings.Contains(lower, ".") {
		return false
	}

	// Overlay network TLDs - valid but app-managed (not set via DOMAIN)
	if strings.HasSuffix(lower, ".onion") ||
		strings.HasSuffix(lower, ".i2p") ||
		strings.HasSuffix(lower, ".exit") {
		return true
	}

	// Dynamic project-specific dev TLD (e.g. app.vidveil) - dev mode only
	if projectName != "" && strings.HasSuffix(lower, "."+strings.ToLower(projectName)) {
		return devMode
	}

	// Get the public suffix (TLD or eTLD like co.uk)
	suffix, icann := publicsuffix.PublicSuffix(lower)

	// Dev-only TLDs are valid in dev mode only
	if devOnlyTLDs[suffix] {
		return devMode
	}

	// In production, require a valid ICANN TLD
	if !devMode && !icann {
		return false
	}

	// Require at least eTLD+1 (e.g. "domain.co.uk", not bare "co.uk")
	etldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(lower)
	if err != nil {
		return false
	}
	return len(etldPlusOne) > 0
}

// IsValidSSLHost validates host for SSL/Let's Encrypt per AI.md
// SSL always requires production-valid host (devMode=false)
func IsValidSSLHost(host string) bool {
	return IsValidHost(host, false, paths.ProjectName)
}

// LiveReload per AI.md PART 8 NON-NEGOTIABLE
// Watches config file and reloads on changes

// ReloadCallback is called when configuration is reloaded
type ReloadCallback func(*AppConfig)

// ConfigWatcher watches for config file changes
type ConfigWatcher struct {
	configPath      string
	appConfig       *AppConfig
	callbacks       []ReloadCallback
	stopChan        chan struct{}
	lastFingerprint [32]byte
}

// NewWatcher creates a new config watcher
func NewWatcher(configPath string, appConfig *AppConfig) *ConfigWatcher {
	return &ConfigWatcher{
		configPath:      configPath,
		appConfig:       appConfig,
		callbacks:       make([]ReloadCallback, 0),
		stopChan:        make(chan struct{}),
		lastFingerprint: configFilesFingerprint(configPath),
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
			fingerprint := configFilesFingerprint(w.configPath)
			if fingerprint != w.lastFingerprint {
				w.lastFingerprint = fingerprint
				w.reload()
			}
		}
	}
}

// reload reloads the configuration and notifies callbacks
func (w *ConfigWatcher) reload() {
	data, dropins, overrides, err := loadMergedConfig(w.configPath)
	if err != nil {
		fmt.Printf("⚠️  Failed to read config for reload: %v\n", err)
		return
	}
	// Start from defaults and overlay the file; unknown or legacy keys are
	// ignored rather than fatal (AI.md PART 5 never declares them errors).
	// A genuine parse failure keeps the currently loaded config.
	newCfg := DefaultAppConfig()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(newCfg); err != nil {
		fmt.Printf("⚠️  Failed to parse config for reload: %v\n", err)
		return
	}

	// Invalid values are warned about and replaced with defaults, same as startup
	validateConfig(newCfg)
	applyRuntimeEnvironmentOverrides(newCfg)
	logConfigSources(dropins, overrides, newCfg.Server.Mode == "debug")

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

	*w.appConfig = *newCfg
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

func configFilesFingerprint(configPath string) [32]byte {
	var content bytes.Buffer
	paths := []string{configPath, filepath.Join(filepath.Dir(configPath), "secrets.yml")}
	dropDir := filepath.Join(filepath.Dir(configPath), "yaml.d")
	if info, statErr := os.Lstat(dropDir); statErr == nil {
		fmt.Fprintf(&content, "dir:%s:%o:%d\x00", dropDir, info.Mode(), info.ModTime().UnixNano())
	} else {
		content.WriteString("dir-missing\x00")
	}
	entries, err := os.ReadDir(dropDir)
	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
				continue
			}
			paths = append(paths, filepath.Join(dropDir, name))
		}
	}
	for _, path := range paths {
		if info, statErr := os.Lstat(path); statErr == nil {
			fmt.Fprintf(&content, "%s:%o:%d\x00", path, info.Mode(), info.ModTime().UnixNano())
			if info.Mode()&os.ModeSymlink != 0 {
				if targetInfo, targetErr := os.Stat(path); targetErr == nil {
					fmt.Fprintf(&content, "target:%o:%d\x00", targetInfo.Mode(), targetInfo.ModTime().UnixNano())
				}
			}
		}
		data, readErr := os.ReadFile(path)
		content.WriteString(path)
		content.WriteByte(0)
		if readErr != nil {
			content.WriteString(readErr.Error())
		} else {
			content.Write(data)
		}
		content.WriteByte(0)
	}
	return sha256.Sum256(content.Bytes())
}

func logConfigSources(dropins, overrides []string, debug bool) {
	for _, file := range dropins {
		log.Printf("INFO config drop-in loaded: %s", file)
	}
	if debug {
		for _, key := range overrides {
			log.Printf("DEBUG config override: %s", key)
		}
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
// ParsePorts splits the configured Server.Port into the HTTP and HTTPS ports
// per AI.md PART 15 "Port Configuration (Project-Wide, NON-NEGOTIABLE)".
//
// Rules:
//   - Two comma-separated ports: first = HTTP, second = HTTPS ("80,443").
//   - Single port: HTTP by default.
//   - Single port 443: HTTPS-only mode.
//   - CONFIG(ssl.enabled) overrides a single port from HTTP to HTTPS.
//
// An empty return value for either port means that protocol is not served.
func ParsePorts(portStr string, sslEnabled bool) (httpPort, httpsPort string) {
	parts := make([]string, 0, 2)
	for _, p := range strings.Split(portStr, ",") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}

	switch {
	case len(parts) == 0:
		return "", ""
	case len(parts) >= 2:
		return parts[0], parts[1]
	default:
		single := parts[0]
		if single == "443" || sslEnabled {
			return "", single
		}
		return single, ""
	}
}

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

// APIBasePath returns the versioned API base path per AI.md PART 14
// ("Always version ALL API routes: /api/{api_version}/..."), e.g. "/api/v1".
// Never hardcode "/api/v1" in route registration or path construction - use this.
func (c *AppConfig) APIBasePath() string {
	ver := c.Server.APIVersion
	if ver == "" {
		ver = "v1"
	}
	return "/api/" + ver
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

// sanitizeSEOVerification removes invalid provider codes and custom tags so
// templates can only receive values that passed the PART 16 format checks.
func sanitizeSEOVerification(v *SEOVerificationConfig) []string {
	if v == nil {
		return nil
	}
	bad := validateSEOVerification(*v)
	invalid := make(map[string]struct{}, len(bad))
	for _, field := range bad {
		invalid[field] = struct{}{}
	}
	for _, provider := range []struct {
		key   string
		value *string
	}{
		{"google", &v.Google}, {"bing", &v.Bing}, {"yandex", &v.Yandex},
		{"baidu", &v.Baidu}, {"pinterest", &v.Pinterest}, {"facebook", &v.Facebook},
	} {
		if _, exists := invalid["seo.verification."+provider.key]; exists {
			*provider.value = ""
		}
	}
	custom := make([]SEOCustomTag, 0, len(v.Custom))
	for index, tag := range v.Custom {
		prefix := fmt.Sprintf("seo.verification.custom[%d]", index)
		invalidTag := false
		for field := range invalid {
			if strings.HasPrefix(field, prefix+".") {
				invalidTag = true
				break
			}
		}
		if !invalidTag {
			custom = append(custom, tag)
		}
	}
	v.Custom = custom
	return bad
}

func seoVerifyPattern(pattern, value string) bool {
	matched, err := regexp.MatchString(pattern, value)
	return err == nil && matched
}

func validateSEOOptions(cfg *SEOConfig, defaults SEOConfig) {
	if cfg == nil {
		return
	}
	if len(cfg.TitleSeparator) > 8 {
		fmt.Fprintf(os.Stderr, "Warning: invalid seo.title_separator, using default\n")
		cfg.TitleSeparator = defaults.TitleSeparator
	}
	if cfg.Canonical != "" {
		parsed, err := url.Parse(cfg.Canonical)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			fmt.Fprintf(os.Stderr, "Warning: invalid seo.canonical, using request origin\n")
			cfg.Canonical = defaults.Canonical
		}
	}
	if len(cfg.MetaTags) > 20 {
		fmt.Fprintf(os.Stderr, "Warning: invalid seo.meta_tags, using default\n")
		cfg.MetaTags = defaults.MetaTags
	} else {
		for name := range cfg.MetaTags {
			if !seoMetaNamePattern.MatchString(name) || isReservedSEOMetaName(name) {
				fmt.Fprintf(os.Stderr, "Warning: invalid seo.meta_tags key, ignoring custom tags\n")
				cfg.MetaTags = defaults.MetaTags
				break
			}
		}
	}
	if cfg.OpenGraph.Type == "" || !seoOpenGraphTypePattern.MatchString(cfg.OpenGraph.Type) {
		fmt.Fprintf(os.Stderr, "Warning: invalid seo.opengraph.type, using default\n")
		cfg.OpenGraph.Type = defaults.OpenGraph.Type
	}
	if cfg.Twitter.Card != "summary" && cfg.Twitter.Card != "summary_large_image" {
		fmt.Fprintf(os.Stderr, "Warning: invalid seo.twitter.card, using default\n")
		cfg.Twitter.Card = defaults.Twitter.Card
	}
}

func isReservedSEOMetaName(name string) bool {
	switch name {
	case "description", "keywords", "author", "robots", "viewport":
		return true
	default:
		return false
	}
}
