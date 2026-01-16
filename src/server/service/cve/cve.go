// SPDX-License-Identifier: MIT
// PART 22: Security & Logging - CVE/Security Database Service
package cve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// CVEService manages CVE (Common Vulnerabilities and Exposures) database per PART 22
type CVEService struct {
	appConfig *config.AppConfig
	dataDir   string
	mu        sync.RWMutex
	cveData   map[string]CVEItem // CVE-ID -> CVE details
}

// CVEItem represents a CVE entry
type CVEItem struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Published   time.Time `json:"published"`
	Modified    time.Time `json:"modified"`
	CVSS        float64   `json:"cvss"`
	Severity    string    `json:"severity"`
	References  []string  `json:"references"`
	CPEs        []string  `json:"cpes"` // Common Platform Enumeration
}

// NVDResponse represents the NVD JSON feed structure
type NVDResponse struct {
	CVEItems []struct {
		CVE struct {
			DataMeta struct {
				ID string `json:"ID"`
			} `json:"CVE_data_meta"`
			Description struct {
				DescriptionData []struct {
					Value string `json:"value"`
				} `json:"description_data"`
			} `json:"description"`
		} `json:"cve"`
		PublishedDate    string `json:"publishedDate"`
		LastModifiedDate string `json:"lastModifiedDate"`
		Impact           struct {
			BaseMetricV3 struct {
				CVSSV3 struct {
					BaseScore float64 `json:"baseScore"`
				} `json:"cvssV3"`
			} `json:"baseMetricV3"`
		} `json:"impact"`
		Configurations struct {
			Nodes []struct {
				CPEMatch []struct {
					CPE23URI string `json:"cpe23Uri"`
				} `json:"cpe_match"`
			} `json:"nodes"`
		} `json:"configurations"`
	} `json:"CVE_Items"`
}

// NewCVEService creates a new CVE service
func NewCVEService(appConfig *config.AppConfig) *CVEService {
	// Get data directory per PART 4 (OS-Specific Paths)
	paths := config.GetAppPaths("", "")
	dataDir := filepath.Join(paths.Config, "security", "cve")

	return &CVEService{
		appConfig: appConfig,
		dataDir:   dataDir,
		cveData: make(map[string]CVEItem),
	}
}

// Initialize creates directory structure per PART 22
func (s *CVEService) Initialize() error {
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create CVE directory: %w", err)
	}
	return nil
}

// Update downloads and updates CVE database per PART 27
func (s *CVEService) Update(ctx context.Context) error {
	if !s.appConfig.Server.Security.CVE.Enabled {
		return nil
	}

	source := s.appConfig.Server.Security.CVE.Source
	if source == "" {
		// Default NVD source per PART 22 specification
		source = "https://nvd.nist.gov/feeds/json/cve/1.1/nvdcve-1.1-recent.json.gz"
	}

	// Download CVE feed
	if err := s.downloadCVEFeed(ctx, source); err != nil {
		return fmt.Errorf("failed to download CVE feed: %w", err)
	}

	// Write timestamp file per PART 22 specification
	timestampFile := filepath.Join(s.dataDir, ".last_updated")
	return os.WriteFile(timestampFile, []byte(time.Now().Format(time.RFC3339)), 0644)
}

// downloadCVEFeed downloads and parses CVE data from NVD
func (s *CVEService) downloadCVEFeed(ctx context.Context, source string) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 120 * time.Second, // CVE feeds can be large
	}

	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Save to file per PART 22 directory structure
	filename := filepath.Join(s.dataDir, "nvd.json")
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy response to file
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Parse and load into memory
	return s.loadCVEData(filename)
}

// loadCVEData loads CVE data from JSON file
func (s *CVEService) loadCVEData(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open CVE data: %w", err)
	}
	defer file.Close()

	var nvdResp NVDResponse
	if err := json.NewDecoder(file).Decode(&nvdResp); err != nil {
		return fmt.Errorf("failed to parse CVE data: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing data
	s.cveData = make(map[string]CVEItem)

	// Parse CVE items
	for _, item := range nvdResp.CVEItems {
		cveID := item.CVE.DataMeta.ID
		
		// Get description
		description := ""
		if len(item.CVE.Description.DescriptionData) > 0 {
			description = item.CVE.Description.DescriptionData[0].Value
		}

		// Get CVSS score
		cvss := item.Impact.BaseMetricV3.CVSSV3.BaseScore

		// Get severity
		severity := s.calculateSeverity(cvss)

		// Get CPEs
		cpes := make([]string, 0)
		for _, node := range item.Configurations.Nodes {
			for _, match := range node.CPEMatch {
				cpes = append(cpes, match.CPE23URI)
			}
		}

		// Filter by CPE if enabled per PART 22 specification
		if s.appConfig.Server.Security.CVE.FilterByCPE && len(cpes) == 0 {
			continue
		}

		// Parse dates
		published, _ := time.Parse(time.RFC3339, item.PublishedDate)
		modified, _ := time.Parse(time.RFC3339, item.LastModifiedDate)

		s.cveData[cveID] = CVEItem{
			ID:          cveID,
			Description: description,
			Published:   published,
			Modified:    modified,
			CVSS:        cvss,
			Severity:    severity,
			CPEs:        cpes,
		}
	}

	return nil
}

// calculateSeverity returns severity level based on CVSS score
func (s *CVEService) calculateSeverity(cvss float64) string {
	switch {
	case cvss >= 9.0:
		return "CRITICAL"
	case cvss >= 7.0:
		return "HIGH"
	case cvss >= 4.0:
		return "MEDIUM"
	case cvss > 0:
		return "LOW"
	default:
		return "NONE"
	}
}

// GetCVE retrieves a specific CVE by ID
func (s *CVEService) GetCVE(cveID string) (CVEItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cve, exists := s.cveData[strings.ToUpper(cveID)]
	return cve, exists
}

// SearchByCPE searches CVEs affecting a specific CPE
func (s *CVEService) SearchByCPE(cpe string) []CVEItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]CVEItem, 0)
	cpeLower := strings.ToLower(cpe)

	for _, cve := range s.cveData {
		for _, cveCPE := range cve.CPEs {
			if strings.Contains(strings.ToLower(cveCPE), cpeLower) {
				results = append(results, cve)
				break
			}
		}
	}

	return results
}

// GetStats returns CVE database statistics
func (s *CVEService) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Count by severity
	severityCounts := make(map[string]int)
	for _, cve := range s.cveData {
		severityCounts[cve.Severity]++
	}

	return map[string]interface{}{
		"total_cves": len(s.cveData),
		"by_severity": severityCounts,
		"data_dir":   s.dataDir,
	}
}

// LastUpdate returns the last update timestamp
func (s *CVEService) LastUpdate() time.Time {
	timestampFile := filepath.Join(s.dataDir, ".last_updated")
	data, err := os.ReadFile(timestampFile)
	if err != nil {
		return time.Time{}
	}

	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}
	}

	return t
}
