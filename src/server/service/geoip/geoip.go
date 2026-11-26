// SPDX-License-Identifier: MIT
// AI.md PART 19: GeoIP
package geoip

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/health"
)

// Database URLs per AI.md PART 19.
const (
	ASNURL     = "https://github.com/sapics/ip-location-db/releases/download/latest/iptoasn-asn.mmdb"
	CountryURL = "https://github.com/sapics/ip-location-db/releases/download/latest/iptoasn-country.mmdb"
	CityURL    = "https://github.com/sapics/ip-location-db/releases/download/latest/dbip-city-ipv4.mmdb"
	CityURLv6  = "https://github.com/sapics/ip-location-db/releases/download/latest/dbip-city-ipv6.mmdb"
)

// countryDBWarnOnce keeps the missing-country-database warning to a single log
// line instead of one per request.
var countryDBWarnOnce sync.Once

// GeoIPResult holds GeoIP lookup results
type GeoIPResult struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	City        string  `json:"city,omitempty"`
	Region      string  `json:"region,omitempty"`
	Postal      string  `json:"postal,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	ASN         uint    `json:"asn,omitempty"`
	ASNOrg      string  `json:"asn_org,omitempty"`
}

// GeoIPService provides GeoIP lookup functionality
type GeoIPService struct {
	mu        sync.RWMutex
	appConfig *config.AppConfig
	dataDir   string

	asnDB     *maxminddb.Reader
	countryDB *maxminddb.Reader
	// cityDB serves IPv4 lookups and cityDBv6 serves IPv6 lookups: per AI.md
	// PART 19 the DB-IP city databases ship as separate v4/v6 files, so IPv6
	// clients need a separate reader.
	cityDB   *maxminddb.Reader
	cityDBv6 *maxminddb.Reader

	lastUpdate   time.Time
	updateFailed bool
}

// asnRecord for ASN database queries
type asnRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// countryRecord for country database queries
type countryRecord struct {
	CountryCode string `maxminddb:"country_code"`
}

// cityRecord for city database queries
type cityRecord struct {
	City        string  `maxminddb:"city"`
	CountryCode string  `maxminddb:"country_code"`
	State1      string  `maxminddb:"state1"`
	State2      string  `maxminddb:"state2"`
	Postcode    string  `maxminddb:"postcode"`
	Latitude    float64 `maxminddb:"latitude"`
	Longitude   float64 `maxminddb:"longitude"`
	Timezone    string  `maxminddb:"timezone"`
}

func applyCountryRecord(result *GeoIPResult, record countryRecord) {
	result.CountryCode = record.CountryCode
}

func applyCityRecord(result *GeoIPResult, record cityRecord) {
	result.CountryCode = record.CountryCode
	result.City = record.City
	result.Region = record.State1
	if result.Region == "" {
		result.Region = record.State2
	}
	result.Postal = record.Postcode
	result.Latitude = record.Latitude
	result.Longitude = record.Longitude
	result.Timezone = record.Timezone
}

// NewGeoIPService creates a new GeoIP service
// Per AI.md PART 19: GeoIP databases go in {data_dir}/security/geoip/.
func NewGeoIPService(appConfig *config.AppConfig) *GeoIPService {
	paths := config.GetAppPaths("", "")
	dataDir := strings.ReplaceAll(appConfig.Server.GeoIP.Dir, "{data_dir}", paths.Data)
	if dataDir == "" {
		dataDir = filepath.Join(paths.Data, "security", "geoip")
	}

	return &GeoIPService{
		appConfig: appConfig,
		dataDir:   dataDir,
	}
}

// Initialize downloads databases if needed and opens them
func (s *GeoIPService) Initialize() error {
	if !s.appConfig.Server.GeoIP.Enabled {
		return nil
	}

	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create geoip directory: %w", err)
	}

	// Download databases if not present
	if err := s.downloadIfMissing(); err != nil {
		return err
	}

	// Open databases
	return s.openDatabases()
}

// downloadIfMissing downloads databases that don't exist
func (s *GeoIPService) downloadIfMissing() error {
	dbs := s.appConfig.Server.GeoIP.Databases

	if dbs.ASN {
		asnPath := filepath.Join(s.dataDir, "iptoasn-asn.mmdb")
		if _, err := os.Stat(asnPath); os.IsNotExist(err) {
			if err := s.downloadFile(ASNURL, asnPath); err != nil {
				return fmt.Errorf("failed to download ASN database: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("stat ASN database: %w", err)
		}
	}

	if dbs.Country {
		countryPath := filepath.Join(s.dataDir, "iptoasn-country.mmdb")
		if _, err := os.Stat(countryPath); os.IsNotExist(err) {
			if err := s.downloadFile(CountryURL, countryPath); err != nil {
				return fmt.Errorf("failed to download country database: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("stat country database: %w", err)
		}
	}

	if dbs.City {
		cityPath := filepath.Join(s.dataDir, "dbip-city-ipv4.mmdb")
		if _, err := os.Stat(cityPath); os.IsNotExist(err) {
			if err := s.downloadFile(CityURL, cityPath); err != nil {
				return fmt.Errorf("failed to download city database: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("stat city database: %w", err)
		}

		cityPathV6 := filepath.Join(s.dataDir, "dbip-city-ipv6.mmdb")
		if _, err := os.Stat(cityPathV6); os.IsNotExist(err) {
			if err := s.downloadFile(CityURLv6, cityPathV6); err != nil {
				return fmt.Errorf("failed to download city IPv6 database: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("stat city IPv6 database: %w", err)
		}
	}

	return nil
}

// downloadFile downloads a file from URL to path
// Verifies the upstream SHA-256 manifest before atomically replacing the file.
func (s *GeoIPService) downloadFile(url, path string) error {
	checksumURL, err := checksumURL(url)
	if err != nil {
		return err
	}
	return s.downloadFileWithChecksum(url, checksumURL, path, &http.Client{Timeout: 60 * time.Second})
}

func (s *GeoIPService) downloadFileWithChecksum(url, checksumURL, path string, client *http.Client) error {
	filename := filepath.Base(strings.Split(url, "?")[0])
	digest, err := fetchChecksum(client, checksumURL, filename)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "vidveil GeoIP database updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			return fmt.Errorf("download failed with status %d (close response: %w)", resp.StatusCode, closeErr)
		}
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return fmt.Errorf("create GeoIP database directory: %w (close response: %v)", err, closeErr)
		}
		return err
	}
	out, err := os.CreateTemp(filepath.Dir(path), ".geoip-*.tmp")
	if err != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			return fmt.Errorf("create temporary GeoIP database: %w (close response: %v)", err, closeErr)
		}
		return err
	}
	tmpPath := out.Name()
	cleanup := func(primary error) error {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			if primary != nil {
				return fmt.Errorf("%w (remove temporary GeoIP file: %v)", primary, removeErr)
			}
			return fmt.Errorf("remove temporary GeoIP file: %w", removeErr)
		}
		return primary
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, hasher), resp.Body)
	bodyCloseErr := resp.Body.Close()
	fileCloseErr := out.Close()
	if copyErr != nil {
		return cleanup(copyErr)
	}
	if bodyCloseErr != nil {
		return cleanup(fmt.Errorf("close GeoIP response: %w", bodyCloseErr))
	}
	if fileCloseErr != nil {
		return cleanup(fmt.Errorf("close downloaded GeoIP database: %w", fileCloseErr))
	}
	if !equalDigest(hasher.Sum(nil), digest) {
		return cleanup(fmt.Errorf("SHA-256 checksum mismatch for %s", filepath.Base(path)))
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return cleanup(fmt.Errorf("set GeoIP database permissions: %w", err))
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return cleanup(fmt.Errorf("replace GeoIP database: %w", err))
	}
	return nil
}

func checksumURL(databaseURL string) (string, error) {
	u, err := http.NewRequest("GET", databaseURL, nil)
	if err != nil {
		return "", err
	}
	base := filepath.Base(u.URL.Path)
	if base == "." || base == "/" || base == "" {
		return "", fmt.Errorf("invalid GeoIP database URL %q", databaseURL)
	}
	return "https://github.com/sapics/ip-location-db/releases/download/checksum/" + base + ".sha256", nil
}

func fetchChecksum(client *http.Client, url, filename string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(resp.Body, 4097))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close GeoIP checksum response: %w", closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksum download failed with status %d", resp.StatusCode)
	}
	if len(content) > 4096 {
		return nil, fmt.Errorf("GeoIP checksum response is too large")
	}
	fields := strings.Fields(string(content))
	if len(fields) == 0 || (len(fields) > 1 && strings.TrimPrefix(fields[1], "*") != filename) {
		return nil, fmt.Errorf("GeoIP checksum manifest does not name %s", filename)
	}
	if len(fields[0]) != sha256.Size*2 {
		return nil, fmt.Errorf("invalid SHA-256 checksum for %s", filename)
	}
	digest, err := hex.DecodeString(fields[0])
	if err != nil {
		return nil, fmt.Errorf("invalid SHA-256 checksum for %s: %w", filename, err)
	}
	return digest, nil
}

func equalDigest(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// openDatabases opens all configured databases
func (s *GeoIPService) openDatabases() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dbs := s.appConfig.Server.GeoIP.Databases
	if dbs.ASN {
		asnPath := filepath.Join(s.dataDir, "iptoasn-asn.mmdb")
		if _, err := os.Stat(asnPath); err == nil {
			db, err := maxminddb.Open(asnPath)
			if err != nil {
				return fmt.Errorf("failed to open ASN database: %w", err)
			}
			s.asnDB = db
		}
	}

	if dbs.Country {
		countryPath := filepath.Join(s.dataDir, "iptoasn-country.mmdb")
		if _, err := os.Stat(countryPath); err == nil {
			db, err := maxminddb.Open(countryPath)
			if err != nil {
				return fmt.Errorf("failed to open country database: %w", err)
			}
			s.countryDB = db
		}
	}

	if dbs.City {
		cityPath := filepath.Join(s.dataDir, "dbip-city-ipv4.mmdb")
		if _, err := os.Stat(cityPath); err == nil {
			db, err := maxminddb.Open(cityPath)
			if err != nil {
				return fmt.Errorf("failed to open city database: %w", err)
			}
			s.cityDB = db
		}

		cityPathV6 := filepath.Join(s.dataDir, "dbip-city-ipv6.mmdb")
		if _, err := os.Stat(cityPathV6); err == nil {
			db, err := maxminddb.Open(cityPathV6)
			if err != nil {
				return fmt.Errorf("failed to open city IPv6 database: %w", err)
			}
			s.cityDBv6 = db
		}
	}

	s.lastUpdate = time.Now()
	return nil
}

// Lookup performs a GeoIP lookup for an IP address
func (s *GeoIPService) Lookup(ipStr string) *GeoIPResult {
	if !s.appConfig.Server.GeoIP.Enabled {
		return &GeoIPResult{IP: ipStr}
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return &GeoIPResult{IP: ipStr}
	}

	result := &GeoIPResult{IP: ipStr}
	if isPrivateIP(ip) {
		return result
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// ASN lookup
	if s.asnDB != nil {
		var record asnRecord
		if err := s.asnDB.Lookup(ip, &record); err == nil {
			result.ASN = record.AutonomousSystemNumber
			result.ASNOrg = record.AutonomousSystemOrganization
		}
	}

	// Country lookup (prefer the family-appropriate city DB if available).
	// IPv6 clients query cityDBv6; IPv4 clients query cityDB. When the
	// family-specific reader is missing, cityReader stays nil so the lookup
	// drops to the family-agnostic country-only DB below rather than probing a
	// database that cannot contain this address family.
	var cityReader *maxminddb.Reader
	if ip.To4() != nil {
		cityReader = s.cityDB
	} else {
		cityReader = s.cityDBv6
	}
	if cityReader != nil {
		var record cityRecord
		if err := cityReader.Lookup(ip, &record); err == nil {
			applyCityRecord(result, record)
		}
	}
	if result.CountryCode == "" && s.countryDB != nil {
		var record countryRecord
		if err := s.countryDB.Lookup(ip, &record); err == nil {
			applyCountryRecord(result, record)
		}
	}

	return result
}

// isPrivateIP reports whether ip is an RFC 1918 / RFC 4193 / loopback / link-local address.
// Private and internal IPs are never subject to country blocking per AI.md PART 19.
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	privateRanges := []net.IPNet{
		{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)},
		{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)},
		{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)},
		{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},
		{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)},
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// IsBlocked checks if an IP is from a blocked country per AI.md PART 19.
// CountryMode controls behavior: "none" disables blocking, "allow" = allowlist-only,
// "deny" = blocklist. AllowCountries takes precedence when both are set (allowlist mode).
// RFC 1918 private and loopback IPs are never blocked.
func (s *GeoIPService) IsBlocked(ipStr string) bool {
	if !s.appConfig.Server.GeoIP.Enabled {
		return false
	}

	// country_mode: "none" means no country blocking
	mode := s.appConfig.Server.GeoIP.CountryMode
	if mode == "none" || mode == "" {
		// legacy: if CountryMode unset, fall back to list presence
		if len(s.appConfig.Server.GeoIP.AllowCountries) == 0 &&
			len(s.appConfig.Server.GeoIP.DenyCountries) == 0 {
			return false
		}
	}

	allowList := s.appConfig.Server.GeoIP.AllowCountries
	denyList := s.appConfig.Server.GeoIP.DenyCountries

	// Nothing configured — nothing blocked
	if len(allowList) == 0 && len(denyList) == 0 {
		return false
	}

	// Per AI.md PART 19: private/internal IPs are never country-blocked
	ip := net.ParseIP(ipStr)
	if isPrivateIP(ip) {
		return false
	}

	// Per AI.md PART 19: country blocking requires a country-capable database.
	// If none is loaded for this address family, the country is unresolvable —
	// same as a resolvable database returning no record for this IP.
	countryCode := ""
	if s.countryLookupAvailable(ip) {
		countryCode = s.Lookup(ipStr).CountryCode
	} else {
		countryDBWarnOnce.Do(func() {
			log.Printf("geoip: country database unavailable, country blocking skipped")
		})
	}

	// A missing database or failed lookup must never block a request. This
	// fail-open rule applies to both country-list modes; a resolved country
	// outside an allowlist is still blocked below.
	if len(allowList) > 0 {
		if countryCode == "" {
			return false
		}
		for _, code := range allowList {
			if code == countryCode {
				return false
			}
		}
		return true
	}

	// Denylist mode is default-allow: an unresolvable/unknown country is never
	// explicitly denied, so GeoIP fails open per the risk-signal rule.
	if countryCode == "" {
		return false
	}
	for _, code := range denyList {
		if code == countryCode {
			return true
		}
	}
	return false
}

// countryLookupAvailable reports whether a database able to resolve a country
// code for this address family is currently open. The family-specific city
// database also carries country codes, so either one satisfies the check.
func (s *GeoIPService) countryLookupAvailable(ip net.IP) bool {
	if ip == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.countryDB != nil {
		return true
	}
	if ip.To4() != nil {
		return s.cityDB != nil
	}
	return s.cityDBv6 != nil
}

// RestrictionResult holds the result of a content restriction check
type RestrictionResult struct {
	// Restricted indicates if the user is from a restricted region
	Restricted bool
	// Mode is the restriction mode: "off", "warn", "soft_block", "hard_block"
	Mode string
	// Reason describes why access is restricted (country or region name)
	Reason string
	// Message is the warning/block message to display
	Message string
	// GeoIP holds the full GeoIP lookup result
	GeoIP *GeoIPResult
}

// CheckContentRestriction checks if an IP is from a content-restricted region
// Returns restriction result with mode and message
// If bypassTor is true and IP cannot be geolocated, restriction is bypassed
func (s *GeoIPService) CheckContentRestriction(ipStr string, isTorUser bool) *RestrictionResult {
	cfg := s.appConfig.Server.GeoIP.ContentRestriction

	// Default result - not restricted
	result := &RestrictionResult{
		Restricted: false,
		Mode:       cfg.Mode,
		Message:    cfg.WarningMessage,
	}

	// Mode "off" means no restriction checking
	if cfg.Mode == "off" || cfg.Mode == "" {
		return result
	}

	// Bypass for Tor users if configured
	if isTorUser && cfg.BypassTor {
		return result
	}

	// GeoIP must be enabled
	if !s.appConfig.Server.GeoIP.Enabled {
		return result
	}

	// No restrictions configured
	if len(cfg.RestrictedCountries) == 0 && len(cfg.RestrictedRegions) == 0 {
		return result
	}

	// Perform GeoIP lookup
	geoResult := s.Lookup(ipStr)
	result.GeoIP = geoResult

	// Cannot geolocate - bypass (likely VPN/Tor)
	if geoResult.CountryCode == "" {
		return result
	}

	// Check country restrictions
	for _, country := range cfg.RestrictedCountries {
		if country == geoResult.CountryCode {
			result.Restricted = true
			result.Reason = geoResult.Country
			if result.Reason == "" {
				result.Reason = geoResult.CountryCode
			}
			return result
		}
	}

	// Check region restrictions (format: "COUNTRY:Region Name")
	if geoResult.Region != "" {
		regionKey := geoResult.CountryCode + ":" + geoResult.Region
		for _, restricted := range cfg.RestrictedRegions {
			if restricted == regionKey {
				result.Restricted = true
				result.Reason = geoResult.Region + ", " + geoResult.Country
				return result
			}
		}
	}

	return result
}

// GetRestrictionMode returns the current content restriction mode
func (s *GeoIPService) GetRestrictionMode() string {
	return s.appConfig.Server.GeoIP.ContentRestriction.Mode
}

// GetRestrictionConfig returns the content restriction configuration
func (s *GeoIPService) GetRestrictionConfig() config.ContentRestrictionConfig {
	return s.appConfig.Server.GeoIP.ContentRestriction
}

// Update downloads fresh databases
func (s *GeoIPService) Update() (retErr error) {
	defer func() {
		s.mu.Lock()
		s.updateFailed = retErr != nil
		s.mu.Unlock()
	}()
	if !s.appConfig.Server.GeoIP.Enabled {
		return nil
	}

	dbs := s.appConfig.Server.GeoIP.Databases
	type update struct{ url, path string }
	var updates []update
	if dbs.ASN {
		updates = append(updates, update{ASNURL, filepath.Join(s.dataDir, "iptoasn-asn.mmdb")})
	}
	if dbs.Country {
		updates = append(updates, update{CountryURL, filepath.Join(s.dataDir, "iptoasn-country.mmdb")})
	}
	if dbs.City {
		updates = append(updates, update{CityURL, filepath.Join(s.dataDir, "dbip-city-ipv4.mmdb")}, update{CityURLv6, filepath.Join(s.dataDir, "dbip-city-ipv6.mmdb")})
	}
	staged := make([]string, 0, len(updates))
	cleanup := func(primary error) error {
		for _, path := range staged {
			if path == "" {
				continue
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				if primary == nil {
					primary = fmt.Errorf("remove staged GeoIP database %s: %w", path, err)
				} else {
					primary = fmt.Errorf("%w (remove staged GeoIP database %s: %v)", primary, path, err)
				}
			}
		}
		return primary
	}
	for _, item := range updates {
		stagePath := item.path + ".next"
		staged = append(staged, stagePath)
		if err := s.downloadFile(item.url, stagePath); err != nil {
			return cleanup(fmt.Errorf("failed to update %s: %w", filepath.Base(item.path), err))
		}
	}
	if err := s.Close(); err != nil {
		return cleanup(fmt.Errorf("close current GeoIP databases before update: %w", err))
	}
	for i, item := range updates {
		if err := os.Rename(staged[i], item.path); err != nil {
			return cleanup(fmt.Errorf("replace %s: %w", filepath.Base(item.path), err))
		}
		staged[i] = ""
	}
	if err := s.openDatabases(); err != nil {
		return cleanup(err)
	}
	return cleanup(nil)
}

// LastUpdate returns when databases were last updated
func (s *GeoIPService) LastUpdate() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUpdate
}

// RegisterHealthChecks registers each configured GeoIP database as an
// optional, public-safe cached health check per AI.md PART 13.
func (s *GeoIPService) RegisterHealthChecks(registry *health.Registry) {
	if registry == nil || s.appConfig == nil || !s.appConfig.Server.GeoIP.Enabled {
		return
	}
	databases := s.appConfig.Server.GeoIP.Databases
	for _, database := range []struct {
		key     string
		name    string
		path    string
		enabled bool
	}{
		{key: "geoip_asn", name: "asn", path: "iptoasn-asn.mmdb", enabled: databases.ASN},
		{key: "geoip_country", name: "country", path: "iptoasn-country.mmdb", enabled: databases.Country},
		{key: "geoip_city_ipv4", name: "city_ipv4", path: "dbip-city-ipv4.mmdb", enabled: databases.City},
		{key: "geoip_city_ipv6", name: "city_ipv6", path: "dbip-city-ipv6.mmdb", enabled: databases.City},
	} {
		if !database.enabled {
			continue
		}
		name := database.name
		path := filepath.Join(s.dataDir, database.path)
		_ = registry.Register(database.key, false, func() string {
			s.mu.RLock()
			loaded := false
			switch name {
			case "asn":
				loaded = s.asnDB != nil
			case "country":
				loaded = s.countryDB != nil
			case "city":
				loaded = s.cityDB != nil && s.cityDBv6 != nil
			case "city_ipv4":
				loaded = s.cityDB != nil
			case "city_ipv6":
				loaded = s.cityDBv6 != nil
			}
			updateFailed := s.updateFailed
			s.mu.RUnlock()
			if !loaded {
				return health.StatusError
			}
			info, err := os.Stat(path)
			if err != nil {
				return health.StatusError
			}
			if updateFailed || time.Since(info.ModTime()) > 7*24*time.Hour {
				return health.StatusDegraded
			}
			return health.StatusOK
		})
	}
}

// IsEnabled returns whether GeoIP is enabled
func (s *GeoIPService) IsEnabled() bool {
	return s.appConfig.Server.GeoIP.Enabled
}

// Close closes all database readers and reports any close failures.
func (s *GeoIPService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var closeErr error
	if s.asnDB != nil {
		if err := s.asnDB.Close(); err != nil {
			closeErr = fmt.Errorf("close ASN database: %w", err)
		}
		s.asnDB = nil
	}
	if s.countryDB != nil {
		if err := s.countryDB.Close(); err != nil {
			closeErr = appendCloseError(closeErr, "country", err)
		}
		s.countryDB = nil
	}
	if s.cityDB != nil {
		if err := s.cityDB.Close(); err != nil {
			closeErr = appendCloseError(closeErr, "city", err)
		}
		s.cityDB = nil
	}
	if s.cityDBv6 != nil {
		if err := s.cityDBv6.Close(); err != nil {
			closeErr = appendCloseError(closeErr, "city IPv6", err)
		}
		s.cityDBv6 = nil
	}
	return closeErr
}

func appendCloseError(current error, name string, err error) error {
	if current == nil {
		return fmt.Errorf("close %s database: %w", name, err)
	}
	return fmt.Errorf("%w; close %s database: %v", current, name, err)
}
