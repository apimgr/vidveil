// SPDX-License-Identifier: MIT
// AI.md PART 10: GeoIP Support
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

// Database URLs per AI.md PART 10
const (
	ASNURL     = "https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb"
	CountryURL = "https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb"
	CityURL    = "https://cdn.jsdelivr.net/npm/@ip-location-db/dbip-city-mmdb/dbip-city.mmdb"
)

// Result holds GeoIP lookup results
type Result struct {
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

// Service provides GeoIP lookup functionality
type Service struct {
	mu      sync.RWMutex
	cfg     *config.Config
	dataDir string

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

// New creates a new GeoIP service
func New(cfg *config.Config) *Service {
	dataDir := cfg.Server.GeoIP.Dir
	if dataDir == "" {
		paths := config.GetPaths("", "")
		dataDir = filepath.Join(paths.Data, "geoip")
	}

	return &Service{
		cfg:     cfg,
		dataDir: dataDir,
	}
}

// Initialize downloads databases if needed and opens them
func (s *Service) Initialize() error {
	if !s.cfg.Server.GeoIP.Enabled {
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
func (s *Service) downloadIfMissing() error {
	dbs := s.cfg.Server.GeoIP.Databases

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
func (s *Service) downloadFile(url, path string) error {
	resp, err := http.Get(url)
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
func (s *Service) openDatabases() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dbs := s.cfg.Server.GeoIP.Databases

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
func (s *Service) Lookup(ipStr string) *Result {
	if !s.cfg.Server.GeoIP.Enabled {
		return &Result{IP: ipStr}
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return &Result{IP: ipStr}
	}

	result := &Result{IP: ipStr}

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
func (s *Service) IsBlocked(ipStr string) bool {
	if !s.cfg.Server.GeoIP.Enabled {
		return false
	}

	denyList := s.cfg.Server.GeoIP.DenyCountries
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

// Update downloads fresh databases
func (s *Service) Update() error {
	if !s.cfg.Server.GeoIP.Enabled {
		return nil
	}

	// Close existing databases
	s.Close()

	dbs := s.cfg.Server.GeoIP.Databases

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
func (s *Service) LastUpdate() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUpdate
}

// IsEnabled returns whether GeoIP is enabled
func (s *Service) IsEnabled() bool {
	return s.cfg.Server.GeoIP.Enabled
}

// Close closes all database readers
func (s *Service) Close() {
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
