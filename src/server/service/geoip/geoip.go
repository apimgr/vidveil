// SPDX-License-Identifier: MIT
// AI.md PART 19: GeoIP
package geoip

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

	lastUpdate         time.Time
	updateFailed       bool
	checksumUnverified bool
}

// asnRecord for ASN database queries
type asnRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// countryRecord for country database queries
type countryRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
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
	result.CountryCode = record.Country.ISOCode
	if result.CountryCode == "" {
		result.CountryCode = record.CountryCode
	}
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
		log.Printf("geoip: one or more database downloads failed; continuing with available databases: %v", err)
	}

	// Open databases
	return s.openDatabases()
}

// downloadIfMissing downloads databases that don't exist
func (s *GeoIPService) downloadIfMissing() error {
	dbs := s.appConfig.Server.GeoIP.Databases

	var updateErr error
	didUpdate := false
	tryDownload := func(name, url, path string) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			updateErr = errors.Join(updateErr, fmt.Errorf("stat %s database: %w", name, err))
			return
		}
		if err := s.downloadFile(url, path); err != nil {
			updateErr = errors.Join(updateErr, fmt.Errorf("download %s database: %w", name, err))
		} else {
			didUpdate = true
		}
	}
	if dbs.ASN {
		asnPath := filepath.Join(s.dataDir, "iptoasn-asn.mmdb")
		tryDownload("ASN", ASNURL, asnPath)
	}

	if dbs.Country {
		countryPath := filepath.Join(s.dataDir, "iptoasn-country.mmdb")
		tryDownload("country", CountryURL, countryPath)
	}

	if dbs.City {
		cityPath := filepath.Join(s.dataDir, "dbip-city-ipv4.mmdb")
		tryDownload("city IPv4", CityURL, cityPath)

		cityPathV6 := filepath.Join(s.dataDir, "dbip-city-ipv6.mmdb")
		tryDownload("city IPv6", CityURLv6, cityPathV6)
	}
	if updateErr == nil && didUpdate {
		if err := writeLastUpdated(s.dataDir, time.Now().UTC()); err != nil {
			return fmt.Errorf("write GeoIP update timestamp: %w", err)
		}
	}
	return updateErr
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
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		lastErr = s.downloadFileAttempt(url, checksumURL, path, client)
		if lastErr == nil {
			return nil
		}
		if !strings.Contains(lastErr.Error(), "checksum mismatch") || attempt == 1 {
			break
		}
		time.Sleep(time.Second)
	}
	if !strings.Contains(lastErr.Error(), "checksum mismatch") {
		return lastErr
	}
	livePath := strings.TrimSuffix(path, ".next")
	if _, err := os.Stat(livePath); err == nil {
		return lastErr
	} else if !os.IsNotExist(err) {
		return errors.Join(lastErr, fmt.Errorf("stat existing GeoIP database: %w", err))
	}
	if err := installUnverifiedMMDB(url, path, client); err != nil {
		return errors.Join(lastErr, err)
	}
	s.mu.Lock()
	s.checksumUnverified = true
	s.mu.Unlock()
	log.Printf("geoip: installed valid but checksum-unverified %s database; health is degraded and update will retry", filepath.Base(path))
	return nil
}

func (s *GeoIPService) downloadFileAttempt(url, checksumURL, path string, client *http.Client) error {
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

func installUnverifiedMMDB(url, path string, client *http.Client) error {
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
		_ = resp.Body.Close()
		return fmt.Errorf("unverified database download failed with status %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".geoip-unverified-*.tmp")
	if err != nil {
		_ = resp.Body.Close()
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func(err error) error {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return errors.Join(err, fmt.Errorf("remove staged unverified database: %w", removeErr))
		}
		return err
	}
	_, copyErr := io.Copy(tmp, resp.Body)
	bodyCloseErr := resp.Body.Close()
	if copyErr != nil || bodyCloseErr != nil {
		_ = tmp.Close()
		return cleanup(errors.Join(copyErr, bodyCloseErr))
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		return cleanup(err)
	}
	db, err := maxminddb.Open(tmpPath)
	if err != nil {
		return cleanup(fmt.Errorf("unverified download is not a valid MMDB: %w", err))
	}
	metadata := db.Metadata
	closeErr := db.Close()
	if closeErr != nil {
		return cleanup(fmt.Errorf("close unverified MMDB: %w", closeErr))
	}
	if metadata.NodeCount == 0 || metadata.RecordSize == 0 || metadata.IPVersion != 4 && metadata.IPVersion != 6 || metadata.DatabaseType == "" {
		return cleanup(fmt.Errorf("unverified download has invalid MMDB metadata"))
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return cleanup(err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return cleanup(err)
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

func writeLastUpdated(dir string, updated time.Time) error {
	tmp, err := os.CreateTemp(dir, ".last_updated-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := fmt.Fprintln(tmp, updated.UTC().Format(time.RFC3339)); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, filepath.Join(dir, ".last_updated")); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func lastUpdated(dir string) (time.Time, error) {
	content, err := os.ReadFile(filepath.Join(dir, ".last_updated"))
	if err != nil {
		return time.Time{}, err
	}
	updated, err := time.Parse(time.RFC3339, strings.TrimSpace(string(content)))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse GeoIP last-updated timestamp: %w", err)
	}
	return updated, nil
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
	var openErr error
	open := func(label, path string, target **maxminddb.Reader) {
		if _, err := os.Stat(path); err != nil {
			if !os.IsNotExist(err) {
				openErr = errors.Join(openErr, fmt.Errorf("stat %s database: %w", label, err))
			}
			return
		}
		db, err := maxminddb.Open(path)
		if err != nil {
			openErr = errors.Join(openErr, fmt.Errorf("open %s database: %w", label, err))
			log.Printf("geoip: %s database unavailable: %v", label, err)
			return
		}
		*target = db
	}
	if dbs.ASN {
		asnPath := filepath.Join(s.dataDir, "iptoasn-asn.mmdb")
		open("ASN", asnPath, &s.asnDB)
	}

	if dbs.Country {
		countryPath := filepath.Join(s.dataDir, "iptoasn-country.mmdb")
		open("country", countryPath, &s.countryDB)
	}

	if dbs.City {
		cityPath := filepath.Join(s.dataDir, "dbip-city-ipv4.mmdb")
		open("city IPv4", cityPath, &s.cityDB)

		cityPathV6 := filepath.Join(s.dataDir, "dbip-city-ipv6.mmdb")
		open("city IPv6", cityPathV6, &s.cityDBv6)
	}

	if updated, err := lastUpdated(s.dataDir); err == nil {
		s.lastUpdate = updated
	}
	// A corrupt or unavailable dataset is isolated; other open databases remain usable.
	if openErr != nil {
		log.Printf("geoip: one or more database files could not be opened: %v", openErr)
	}
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
	s.mu.Lock()
	previouslyUnverified := s.checksumUnverified
	s.checksumUnverified = false
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.updateFailed = retErr != nil
		if retErr != nil && previouslyUnverified {
			s.checksumUnverified = true
		}
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
	var updateErr error
	validStages := make(map[int]bool, len(updates))
	for i, item := range updates {
		stagePath := item.path + ".next"
		staged = append(staged, stagePath)
		if err := s.downloadFile(item.url, stagePath); err != nil {
			updateErr = errors.Join(updateErr, fmt.Errorf("failed to update %s: %w", filepath.Base(item.path), err))
			continue
		}
		validStages[i] = true
	}
	s.mu.RLock()
	stagedUnverified := s.checksumUnverified
	s.mu.RUnlock()
	if len(validStages) == 0 {
		return cleanup(updateErr)
	}
	if err := s.Close(); err != nil {
		return cleanup(errors.Join(updateErr, fmt.Errorf("close current GeoIP databases before update: %w", err)))
	}
	for i, item := range updates {
		if !validStages[i] {
			continue
		}
		if err := os.Rename(staged[i], item.path); err != nil {
			updateErr = errors.Join(updateErr, fmt.Errorf("replace %s: %w", filepath.Base(item.path), err))
			continue
		}
		staged[i] = ""
	}
	if err := s.openDatabases(); err != nil {
		updateErr = errors.Join(updateErr, err)
	}
	if updateErr == nil {
		if err := writeLastUpdated(s.dataDir, time.Now().UTC()); err != nil {
			updateErr = fmt.Errorf("write GeoIP update timestamp: %w", err)
		}
	}
	s.mu.Lock()
	if updateErr == nil {
		s.checksumUnverified = stagedUnverified
		s.lastUpdate = time.Now().UTC()
	} else if previouslyUnverified {
		s.checksumUnverified = true
	}
	s.mu.Unlock()
	return cleanup(updateErr)
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
			checksumUnverified := s.checksumUnverified
			s.mu.RUnlock()
			if !loaded {
				return health.StatusError
			}
			if _, err := os.Stat(path); err != nil {
				return health.StatusError
			}
			updated, timestampErr := lastUpdated(s.dataDir)
			if updateFailed || checksumUnverified || timestampErr != nil || time.Since(updated) > 21*24*time.Hour {
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
