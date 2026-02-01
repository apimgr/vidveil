// SPDX-License-Identifier: MIT
// AI.md PART 20: GeoIP
package geoip

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"

	"github.com/apimgr/vidveil/src/config"
)

// Database URLs per AI.md PART 20 - using ip-location-db via jsDelivr
// Note: geolite2-city used instead of dbip-city (dbip-city returns 403 on jsDelivr)
const (
	ASNURL     = "https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb"
	CountryURL = "https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb"
	CityURL    = "https://cdn.jsdelivr.net/npm/@ip-location-db/geolite2-city-mmdb/geolite2-city-ipv4.mmdb"
)

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
	cityDB    *maxminddb.Reader

	lastUpdate time.Time
}

// asnRecord for ASN database queries
type asnRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// countryRecord for country database queries
type countryRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

// cityRecord for city database queries
type cityRecord struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Subdivisions []struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	Postal struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"postal"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
}

// NewGeoIPService creates a new GeoIP service
// Per AI.md PART 4: Security DBs go in {config}/security/geoip/
func NewGeoIPService(appConfig *config.AppConfig) *GeoIPService {
	dataDir := appConfig.Server.GeoIP.Dir
	if dataDir == "" {
		paths := config.GetAppPaths("", "")
		dataDir = filepath.Join(paths.Config, "security", "geoip")
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
		asnPath := filepath.Join(s.dataDir, "asn.mmdb")
		if _, err := os.Stat(asnPath); os.IsNotExist(err) {
			if err := s.downloadFile(ASNURL, asnPath); err != nil {
				return fmt.Errorf("failed to download ASN database: %w", err)
			}
		}
	}

	if dbs.Country {
		countryPath := filepath.Join(s.dataDir, "country.mmdb")
		if _, err := os.Stat(countryPath); os.IsNotExist(err) {
			if err := s.downloadFile(CountryURL, countryPath); err != nil {
				return fmt.Errorf("failed to download country database: %w", err)
			}
		}
	}

	if dbs.City {
		cityPath := filepath.Join(s.dataDir, "city.mmdb")
		if _, err := os.Stat(cityPath); os.IsNotExist(err) {
			if err := s.downloadFile(CityURL, cityPath); err != nil {
				return fmt.Errorf("failed to download city database: %w", err)
			}
		}
	}

	return nil
}

// downloadFile downloads a file from URL to path
// Uses User-Agent header as jsDelivr requires it for some files
func (s *GeoIPService) downloadFile(url, path string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpPath := path + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, path)
}

// openDatabases opens all configured databases
func (s *GeoIPService) openDatabases() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dbs := s.appConfig.Server.GeoIP.Databases

	if dbs.ASN {
		asnPath := filepath.Join(s.dataDir, "asn.mmdb")
		if _, err := os.Stat(asnPath); err == nil {
			db, err := maxminddb.Open(asnPath)
			if err != nil {
				return fmt.Errorf("failed to open ASN database: %w", err)
			}
			s.asnDB = db
		}
	}

	if dbs.Country {
		countryPath := filepath.Join(s.dataDir, "country.mmdb")
		if _, err := os.Stat(countryPath); err == nil {
			db, err := maxminddb.Open(countryPath)
			if err != nil {
				return fmt.Errorf("failed to open country database: %w", err)
			}
			s.countryDB = db
		}
	}

	if dbs.City {
		cityPath := filepath.Join(s.dataDir, "city.mmdb")
		if _, err := os.Stat(cityPath); err == nil {
			db, err := maxminddb.Open(cityPath)
			if err != nil {
				return fmt.Errorf("failed to open city database: %w", err)
			}
			s.cityDB = db
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

	// Country lookup (prefer city DB if available)
	if s.cityDB != nil {
		var record cityRecord
		if err := s.cityDB.Lookup(ip, &record); err == nil {
			result.CountryCode = record.Country.ISOCode
			if name, ok := record.Country.Names["en"]; ok {
				result.Country = name
			}
			if name, ok := record.City.Names["en"]; ok {
				result.City = name
			}
			if len(record.Subdivisions) > 0 {
				if name, ok := record.Subdivisions[0].Names["en"]; ok {
					result.Region = name
				}
			}
			result.Postal = record.Postal.Code
			result.Latitude = record.Location.Latitude
			result.Longitude = record.Location.Longitude
			result.Timezone = record.Location.TimeZone
		}
	} else if s.countryDB != nil {
		var record countryRecord
		if err := s.countryDB.Lookup(ip, &record); err == nil {
			result.CountryCode = record.Country.ISOCode
			if name, ok := record.Country.Names["en"]; ok {
				result.Country = name
			}
		}
	}

	return result
}

// IsBlocked checks if an IP is from a blocked country
func (s *GeoIPService) IsBlocked(ipStr string) bool {
	if !s.appConfig.Server.GeoIP.Enabled {
		return false
	}

	denyList := s.appConfig.Server.GeoIP.DenyCountries
	if len(denyList) == 0 {
		return false
	}

	result := s.Lookup(ipStr)
	if result.CountryCode == "" {
		return false
	}

	for _, code := range denyList {
		if code == result.CountryCode {
			return true
		}
	}
	return false
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
func (s *GeoIPService) Update() error {
	if !s.appConfig.Server.GeoIP.Enabled {
		return nil
	}

	// Close existing databases
	s.Close()

	dbs := s.appConfig.Server.GeoIP.Databases

	if dbs.ASN {
		asnPath := filepath.Join(s.dataDir, "asn.mmdb")
		if err := s.downloadFile(ASNURL, asnPath); err != nil {
			return fmt.Errorf("failed to update ASN database: %w", err)
		}
	}

	if dbs.Country {
		countryPath := filepath.Join(s.dataDir, "country.mmdb")
		if err := s.downloadFile(CountryURL, countryPath); err != nil {
			return fmt.Errorf("failed to update country database: %w", err)
		}
	}

	if dbs.City {
		cityPath := filepath.Join(s.dataDir, "city.mmdb")
		if err := s.downloadFile(CityURL, cityPath); err != nil {
			return fmt.Errorf("failed to update city database: %w", err)
		}
	}

	return s.openDatabases()
}

// LastUpdate returns when databases were last updated
func (s *GeoIPService) LastUpdate() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUpdate
}

// IsEnabled returns whether GeoIP is enabled
func (s *GeoIPService) IsEnabled() bool {
	return s.appConfig.Server.GeoIP.Enabled
}

// Close closes all database readers
func (s *GeoIPService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.asnDB != nil {
		s.asnDB.Close()
		s.asnDB = nil
	}
	if s.countryDB != nil {
		s.countryDB.Close()
		s.countryDB = nil
	}
	if s.cityDB != nil {
		s.cityDB.Close()
		s.cityDB = nil
	}
}
