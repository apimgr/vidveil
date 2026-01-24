// SPDX-License-Identifier: MIT
// AI.md PART 11: Security & Logging - IP/Domain Blocklist Service
package blocklist

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// BlocklistService manages IP and domain blocklists per PART 22
type BlocklistService struct {
	appConfig *config.AppConfig
	dataDir   string
	mu        sync.RWMutex
	// ipBlocks contains IP addresses to block
	ipBlocks map[string]bool
	// subnets contains CIDR blocks to check
	subnets []*net.IPNet
	// domains contains domains to block
	domains map[string]bool
}

// NewBlocklistService creates a new blocklist service
func NewBlocklistService(appConfig *config.AppConfig) *BlocklistService {
	// Get data directory per PART 4 (OS-Specific Paths)
	paths := config.GetAppPaths("", "")
	dataDir := filepath.Join(paths.Config, "security", "blocklists")

	return &BlocklistService{
		appConfig: appConfig,
		dataDir:   dataDir,
		ipBlocks:  make(map[string]bool),
		subnets:   make([]*net.IPNet, 0),
		domains:   make(map[string]bool),
	}
}

// Initialize creates directory structure per PART 22
func (s *BlocklistService) Initialize() error {
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create blocklist directory: %w", err)
	}
	return nil
}

// Update downloads and updates all enabled blocklists per PART 27
func (s *BlocklistService) Update(ctx context.Context) error {
	// Check if blocklists are enabled in config
	if !s.appConfig.Server.Security.Blocklists.Enabled || len(s.appConfig.Server.Security.Blocklists.Sources) == 0 {
		return nil
	}

	sources := s.appConfig.Server.Security.Blocklists.Sources
	var errors []string
	
	for _, source := range sources {
		if !source.Enabled {
			continue
		}

		if err := s.downloadAndParse(ctx, source); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", source.Name, err))
			continue
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("blocklist update errors: %s", strings.Join(errors, "; "))
	}

	// Write timestamp file per PART 22 specification
	timestampFile := filepath.Join(s.dataDir, ".last_updated")
	return os.WriteFile(timestampFile, []byte(time.Now().Format(time.RFC3339)), 0644)
}

// downloadAndParse downloads and parses a blocklist source
func (s *BlocklistService) downloadAndParse(ctx context.Context, source config.BlocklistSource) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
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
	filename := filepath.Join(s.dataDir, source.Name+".txt")
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
	return s.loadBlocklist(filename, source.Type)
}

// loadBlocklist loads a blocklist file into memory
func (s *BlocklistService) loadBlocklist(filename, blockType string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open blocklist: %w", err)
	}
	defer file.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if blockType == "ip" {
			s.parseIPLine(line)
		} else if blockType == "domain" {
			s.parseDomainLine(line)
		}
	}

	return scanner.Err()
}

// parseIPLine parses an IP address or CIDR block
func (s *BlocklistService) parseIPLine(line string) {
	// Remove inline comments
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	// Check if it's a CIDR block
	if strings.Contains(line, "/") {
		_, ipNet, err := net.ParseCIDR(line)
		if err == nil {
			s.subnets = append(s.subnets, ipNet)
		}
		return
	}

	// Single IP address
	if ip := net.ParseIP(line); ip != nil {
		s.ipBlocks[line] = true
	}
}

// parseDomainLine parses a domain name
func (s *BlocklistService) parseDomainLine(line string) {
	// Remove inline comments
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	// Remove protocol if present
	line = strings.TrimPrefix(line, "http://")
	line = strings.TrimPrefix(line, "https://")
	
	// Remove path if present
	if idx := strings.Index(line, "/"); idx >= 0 {
		line = line[:idx]
	}

	if line != "" {
		s.domains[strings.ToLower(line)] = true
	}
}

// IsBlocked checks if an IP address or domain is blocked
func (s *BlocklistService) IsBlocked(ipOrDomain string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if it's an IP address
	if ip := net.ParseIP(ipOrDomain); ip != nil {
		// Check exact IP match
		if s.ipBlocks[ipOrDomain] {
			return true
		}

		// Check CIDR blocks
		for _, subnet := range s.subnets {
			if subnet.Contains(ip) {
				return true
			}
		}
		return false
	}

	// Check domain (case-insensitive)
	domain := strings.ToLower(ipOrDomain)
	if s.domains[domain] {
		return true
	}

	// Check parent domains (e.g., sub.example.com matches example.com)
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[i:], ".")
		if s.domains[parent] {
			return true
		}
	}

	return false
}

// GetStats returns blocklist statistics
func (s *BlocklistService) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"ip_count":     len(s.ipBlocks),
		"subnet_count": len(s.subnets),
		"domain_count": len(s.domains),
		"data_dir":     s.dataDir,
	}
}

// LastUpdate returns the last update timestamp
func (s *BlocklistService) LastUpdate() time.Time {
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
