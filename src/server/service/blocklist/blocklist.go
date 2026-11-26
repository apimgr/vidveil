// SPDX-License-Identifier: MIT
// AI.md PART 11: Security & Logging - IP/Domain Blocklist Service
package blocklist

import (
	"bufio"
	"context"
	"crypto/sha256"
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
	"github.com/apimgr/vidveil/src/server/service/health"
)

// BlocklistService manages IP and domain blocklists per PART 11
type BlocklistService struct {
	appConfig *config.AppConfig
	dataDir   string
	mu        sync.RWMutex
	// ipBlocks contains IP addresses to block
	ipBlocks map[string]bool
	// subnets contains CIDR blocks to check
	subnets []*net.IPNet
	// domains contains domains to block
	domains      map[string]bool
	loadedAt     map[string]time.Time
	updateFailed bool
}

// NewBlocklistService creates a new blocklist service
func NewBlocklistService(appConfig *config.AppConfig) *BlocklistService {
	// Get data directory per PART 4 (OS-Specific Paths)
	paths := config.GetAppPaths("", "")
	dataDir := filepath.Join(paths.Data, "security", "blocklists")

	return &BlocklistService{
		appConfig: appConfig,
		dataDir:   dataDir,
		ipBlocks:  make(map[string]bool),
		subnets:   make([]*net.IPNet, 0),
		domains:   make(map[string]bool),
		loadedAt:  make(map[string]time.Time),
	}
}

// Initialize creates directory structure per PART 11
func (s *BlocklistService) Initialize() error {
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create blocklist directory: %w", err)
	}
	if s.appConfig == nil || !s.appConfig.Server.Security.Blocklists.Enabled {
		return nil
	}
	for _, source := range s.appConfig.Server.Security.Blocklists.Sources {
		if !source.Enabled {
			continue
		}
		filename := filepath.Join(s.dataDir, source.Name+".txt")
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("inspect blocklist source %q: %w", source.Name, err)
		}
		if err := s.loadBlocklist(filename, source.Type); err != nil {
			return fmt.Errorf("load blocklist source %q: %w", source.Name, err)
		}
	}
	return nil
}

// RegisterHealthChecks registers one cached check for every enabled source.
func (s *BlocklistService) RegisterHealthChecks(registry *health.Registry) {
	if registry == nil || s.appConfig == nil || !s.appConfig.Server.Security.Blocklists.Enabled {
		return
	}
	sources := s.appConfig.Server.Security.Blocklists.Sources
	keys := blocklistHealthKeys(sources)
	for index, source := range sources {
		if !source.Enabled {
			continue
		}
		key := keys[index]
		if key == "" {
			continue
		}
		name := source.Name
		_ = registry.Register(key, false, func() string {
			s.mu.RLock()
			lastUpdate := s.loadedAt[name]
			updateFailed := s.updateFailed
			s.mu.RUnlock()
			if lastUpdate.IsZero() {
				return health.StatusError
			}
			if updateFailed || time.Since(lastUpdate) > 72*time.Hour {
				return health.StatusDegraded
			}
			return health.StatusOK
		})
	}
}

func blocklistHealthKeys(sources []config.BlocklistSource) []string {
	keys := make([]string, len(sources))
	baseCounts := make(map[string]int)
	for _, source := range sources {
		if source.Enabled {
			if id := healthCheckID(source.Name); id != "" {
				baseCounts[id]++
			}
		}
	}
	used := make(map[string]int)
	for index, source := range sources {
		if !source.Enabled {
			continue
		}
		id := healthCheckID(source.Name)
		if id == "" {
			continue
		}
		if baseCounts[id] > 1 {
			digest := sha256.Sum256([]byte(source.Name))
			id = fmt.Sprintf("%s_%x", id, digest[:4])
		}
		used[id]++
		if used[id] > 1 {
			id = fmt.Sprintf("%s_%d", id, used[id])
		}
		keys[index] = "blocklist_" + id
	}
	return keys
}

func healthCheckID(value string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// Update downloads and updates all enabled blocklists per PART 11
func (s *BlocklistService) Update(ctx context.Context) (retErr error) {
	defer func() {
		s.mu.Lock()
		s.updateFailed = retErr != nil
		s.mu.Unlock()
	}()
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

	// Write timestamp file per PART 11 specification
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Save to file per PART 11 directory structure
	filename := filepath.Join(s.dataDir, source.Name+".txt")
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Copy response to file
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close blocklist file: %w", err)
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
	defer func() { _ = file.Close() }()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadedAt == nil {
		s.loadedAt = make(map[string]time.Time)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch blockType {
		case "ip":
			s.parseIPLine(line)
		case "domain":
			s.parseDomainLine(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	s.loadedAt[strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))] = info.ModTime()
	return nil
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
