// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 32: Embedded Tor Hidden Service Support using bine
package tor

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cretz/bine/tor"
	"golang.org/x/crypto/sha3"
)

// Service represents the embedded Tor hidden service per TEMPLATE.md PART 32
// Uses github.com/cretz/bine for dedicated Tor process management
type Service struct {
	cfg     *Config
	dataDir string

	// bine Tor instance - manages dedicated Tor process
	torInstance *tor.Tor
	onionSvc    net.Listener

	// Hidden service state
	onionAddress string
	privateKey   ed25519.PrivateKey
	publicKey    ed25519.PublicKey

	// Status tracking
	status    Status
	startTime time.Time
	mu        sync.RWMutex

	// Local port the hidden service forwards to
	localPort int

	// Vanity generation
	vanityCtx    context.Context
	vanityCancel context.CancelFunc
	vanityStatus *VanityStatus
}

// Config holds Tor service configuration per TEMPLATE.md PART 32
type Config struct {
	Enabled bool   `yaml:"enabled"` // Default: true (enabled by default per PART 32)
	DataDir string `yaml:"-"`       // Set from paths.GetDataDir() + "/tor"
}

// Status represents Tor service status
type Status string

const (
	StatusDisabled     Status = "disabled"
	StatusStarting     Status = "starting"
	StatusConnected    Status = "connected"
	StatusDisconnected Status = "disconnected"
	StatusError        Status = "error"
	StatusNoTorBinary  Status = "no_tor_binary" // Tor binary not found
)

// VanityStatus tracks vanity address generation progress
type VanityStatus struct {
	Active      bool      `json:"active"`
	Prefix      string    `json:"prefix"`
	StartTime   time.Time `json:"start_time"`
	Attempts    int64     `json:"attempts"`
	ElapsedTime string    `json:"elapsed_time"`
}

// New creates a new Tor service instance
func New(dataDir string, enabled bool) *Service {
	return &Service{
		cfg: &Config{
			Enabled: enabled,
			DataDir: filepath.Join(dataDir, "tor"),
		},
		dataDir: filepath.Join(dataDir, "tor"),
		status:  StatusDisabled,
	}
}

// Start initializes the Tor hidden service using bine
// Per TEMPLATE.md PART 32: Uses dedicated Tor process via bine library
func (s *Service) Start(ctx context.Context, localPort int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.Enabled {
		s.status = StatusDisabled
		return nil
	}

	s.status = StatusStarting
	s.startTime = time.Now()
	s.localPort = localPort

	// Ensure data directories exist
	torDataDir := filepath.Join(s.dataDir, "data")
	siteDir := filepath.Join(s.dataDir, "site")
	if err := os.MkdirAll(torDataDir, 0700); err != nil {
		s.status = StatusError
		return fmt.Errorf("failed to create tor data directory: %w", err)
	}
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		s.status = StatusError
		return fmt.Errorf("failed to create tor site directory: %w", err)
	}

	// Check if Tor binary exists
	torPath, err := exec.LookPath("tor")
	if err != nil {
		// Tor binary not found - fall back to key-only mode
		log.Printf("[tor] Tor binary not found in PATH, running in key-only mode")
		s.status = StatusNoTorBinary

		// Still load/generate keys for address generation
		if err := s.loadOrGenerateKeys(); err != nil {
			s.status = StatusError
			return fmt.Errorf("failed to load/generate keys: %w", err)
		}
		s.onionAddress = s.generateOnionAddress()
		return nil
	}

	log.Printf("[tor] Found Tor binary at: %s", torPath)

	// Load or generate hidden service keys first
	if err := s.loadOrGenerateKeys(); err != nil {
		s.status = StatusError
		return fmt.Errorf("failed to load/generate keys: %w", err)
	}
	s.onionAddress = s.generateOnionAddress()

	// Start dedicated Tor process using bine
	// Per TEMPLATE.md: Start OUR OWN Tor process - completely separate from system Tor
	startConf := &tor.StartConf{
		// Our own data directory - isolated from system Tor
		DataDir: torDataDir,

		// Let bine pick available ports (avoids conflict with system Tor 9050/9051)
		NoAutoSocksPort: false,

		// Use found Tor binary
		ExePath: torPath,

		// Optional: Debug output for development
		// DebugWriter: os.Stderr,
	}

	log.Printf("[tor] Starting dedicated Tor process...")
	t, err := tor.Start(ctx, startConf)
	if err != nil {
		s.status = StatusError
		return fmt.Errorf("failed to start dedicated tor: %w", err)
	}
	s.torInstance = t

	// Wait for Tor to bootstrap (with timeout)
	log.Printf("[tor] Waiting for Tor to bootstrap...")
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	if err := t.EnableNetwork(dialCtx, true); err != nil {
		t.Close()
		s.torInstance = nil
		s.status = StatusError
		return fmt.Errorf("failed to enable tor network: %w", err)
	}

	// Create hidden service on port 80 forwarding to localPort
	log.Printf("[tor] Creating hidden service on port 80 -> localhost:%d", localPort)
	onionSvc, err := t.Listen(ctx, &tor.ListenConf{
		RemotePorts: []int{80},
		LocalPort:   localPort,
		Key:         s.privateKey,
	})
	if err != nil {
		t.Close()
		s.torInstance = nil
		s.status = StatusError
		return fmt.Errorf("failed to create onion service: %w", err)
	}
	s.onionSvc = onionSvc

	// The onion address was already calculated from our keys
	// No need to update from listener - we already have the correct address
	s.status = StatusConnected
	log.Printf("[tor] Hidden service started: %s", s.onionAddress)

	return nil
}

// Stop shuts down the Tor service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel any vanity generation
	if s.vanityCancel != nil {
		s.vanityCancel()
	}

	// Close onion service listener
	if s.onionSvc != nil {
		s.onionSvc.Close()
		s.onionSvc = nil
	}

	// Close dedicated Tor process
	if s.torInstance != nil {
		log.Printf("[tor] Shutting down dedicated Tor process...")
		if err := s.torInstance.Close(); err != nil {
			log.Printf("[tor] Error closing Tor: %v", err)
		}
		s.torInstance = nil
	}

	s.status = StatusDisconnected
	return nil
}

// loadOrGenerateKeys loads existing keys or generates new ones
func (s *Service) loadOrGenerateKeys() error {
	siteDir := filepath.Join(s.dataDir, "site")
	secretKeyPath := filepath.Join(siteDir, "hs_ed25519_secret_key")
	publicKeyPath := filepath.Join(siteDir, "hs_ed25519_public_key")

	// Try to load existing keys
	if _, err := os.Stat(secretKeyPath); err == nil {
		secretData, err := os.ReadFile(secretKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read secret key: %w", err)
		}

		// Tor stores keys with a header "== ed25519v1-secret: type0 ==" (32 bytes) + expanded key
		if len(secretData) >= 64 {
			// Extract the key part (skip header if present)
			var seed []byte
			if len(secretData) == 64 {
				seed = secretData[:32]
			} else if len(secretData) >= 96 {
				// Standard Tor format with header
				seed = secretData[32:64]
			} else {
				return fmt.Errorf("invalid secret key format")
			}

			s.privateKey = ed25519.NewKeyFromSeed(seed)
			s.publicKey = s.privateKey.Public().(ed25519.PublicKey)
			return nil
		}
	}

	// Generate new keys
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate keys: %w", err)
	}

	s.privateKey = priv
	s.publicKey = pub

	// Save keys in Tor format
	// Secret key: "== ed25519v1-secret: type0 ==" header + expanded key
	header := []byte("== ed25519v1-secret: type0 ==\x00\x00\x00")
	secretData := append(header, priv.Seed()...)
	secretData = append(secretData, priv[32:]...)

	if err := os.WriteFile(secretKeyPath, secretData, 0600); err != nil {
		return fmt.Errorf("failed to write secret key: %w", err)
	}

	// Public key: "== ed25519v1-public: type0 ==" header + public key
	pubHeader := []byte("== ed25519v1-public: type0 ==\x00\x00\x00")
	pubData := append(pubHeader, pub...)
	if err := os.WriteFile(publicKeyPath, pubData, 0600); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	// Write hostname file
	hostname := s.generateOnionAddress() + "\n"
	hostnamePath := filepath.Join(siteDir, "hostname")
	if err := os.WriteFile(hostnamePath, []byte(hostname), 0600); err != nil {
		return fmt.Errorf("failed to write hostname: %w", err)
	}

	return nil
}

// generateOnionAddress generates .onion address from public key
// This implements the Tor v3 onion address format
func (s *Service) generateOnionAddress() string {
	// Tor v3 address = base32(pubkey || checksum || version)
	// checksum = SHA3-256(".onion checksum" || pubkey || version)[:2]
	// version = 0x03

	version := byte(0x03)

	// Calculate checksum
	checksumInput := append([]byte(".onion checksum"), s.publicKey...)
	checksumInput = append(checksumInput, version)

	hasher := sha3.New256()
	hasher.Write(checksumInput)
	checksum := hasher.Sum(nil)[:2]

	// Build address bytes
	addressBytes := append([]byte{}, s.publicKey...)
	addressBytes = append(addressBytes, checksum...)
	addressBytes = append(addressBytes, version)

	// Base32 encode (lowercase, no padding)
	address := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(addressBytes))

	return address + ".onion"
}

// GetOnionAddress returns the current .onion address
func (s *Service) GetOnionAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.onionAddress
}

// GetStatus returns the current service status
func (s *Service) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// GetUptime returns the service uptime as a string
func (s *Service) GetUptime() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.status != StatusConnected {
		return "0s"
	}

	uptime := time.Since(s.startTime)
	hours := int(uptime.Hours())
	minutes := int(uptime.Minutes()) % 60
	seconds := int(uptime.Seconds()) % 60

	if hours >= 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}

// IsEnabled returns whether Tor is enabled
func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Enabled
}

// IsRunning returns whether Tor process is actually running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.torInstance != nil && s.status == StatusConnected
}

// RegenerateAddress generates a new random .onion address
// This deletes existing keys and generates new ones
func (s *Service) RegenerateAddress() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	siteDir := filepath.Join(s.dataDir, "site")

	// Delete existing keys
	os.Remove(filepath.Join(siteDir, "hs_ed25519_secret_key"))
	os.Remove(filepath.Join(siteDir, "hs_ed25519_public_key"))
	os.Remove(filepath.Join(siteDir, "hostname"))

	// Generate new keys
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate new keys: %w", err)
	}

	s.privateKey = priv
	s.publicKey = pub

	// Save new keys
	header := []byte("== ed25519v1-secret: type0 ==\x00\x00\x00")
	secretData := append(header, priv.Seed()...)
	secretData = append(secretData, priv[32:]...)

	if err := os.WriteFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), secretData, 0600); err != nil {
		return fmt.Errorf("failed to write new secret key: %w", err)
	}

	pubHeader := []byte("== ed25519v1-public: type0 ==\x00\x00\x00")
	pubData := append(pubHeader, pub...)
	if err := os.WriteFile(filepath.Join(siteDir, "hs_ed25519_public_key"), pubData, 0600); err != nil {
		return fmt.Errorf("failed to write new public key: %w", err)
	}

	// Update onion address
	s.onionAddress = s.generateOnionAddress()

	// Write new hostname
	hostname := s.onionAddress + "\n"
	if err := os.WriteFile(filepath.Join(siteDir, "hostname"), []byte(hostname), 0600); err != nil {
		return fmt.Errorf("failed to write new hostname: %w", err)
	}

	return nil
}

// GenerateVanityAddress starts background generation of a vanity address
// maxPrefixLength is limited to 6 characters per TEMPLATE.md PART 32
func (s *Service) GenerateVanityAddress(prefix string) error {
	prefix = strings.ToLower(prefix)

	// Validate prefix
	if len(prefix) > 6 {
		return fmt.Errorf("prefix too long (max 6 characters for built-in generation)")
	}

	// Check for valid base32 characters only
	for _, c := range prefix {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz234567", c) {
			return fmt.Errorf("invalid character '%c' in prefix (must be a-z or 2-7)", c)
		}
	}

	s.mu.Lock()

	// Cancel any existing generation
	if s.vanityCancel != nil {
		s.vanityCancel()
	}

	s.vanityCtx, s.vanityCancel = context.WithCancel(context.Background())
	s.vanityStatus = &VanityStatus{
		Active:    true,
		Prefix:    prefix,
		StartTime: time.Now(),
		Attempts:  0,
	}

	ctx := s.vanityCtx
	s.mu.Unlock()

	// Start background generation
	go s.runVanityGeneration(ctx, prefix)

	return nil
}

// runVanityGeneration runs the vanity address generation in background
func (s *Service) runVanityGeneration(ctx context.Context, prefix string) {
	var attempts int64
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			if s.vanityStatus != nil {
				s.vanityStatus.Active = false
			}
			s.mu.Unlock()
			return
		default:
			// Generate random key pair
			pub, priv, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				continue
			}

			// Calculate address
			version := byte(0x03)
			checksumInput := append([]byte(".onion checksum"), pub...)
			checksumInput = append(checksumInput, version)
			hasher := sha3.New256()
			hasher.Write(checksumInput)
			checksum := hasher.Sum(nil)[:2]

			addressBytes := append([]byte{}, pub...)
			addressBytes = append(addressBytes, checksum...)
			addressBytes = append(addressBytes, version)

			address := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(addressBytes))

			attempts++

			// Update status periodically
			if attempts%10000 == 0 {
				s.mu.Lock()
				if s.vanityStatus != nil {
					s.vanityStatus.Attempts = attempts
					s.vanityStatus.ElapsedTime = time.Since(s.vanityStatus.StartTime).Round(time.Second).String()
				}
				s.mu.Unlock()
			}

			// Check if address matches prefix
			if strings.HasPrefix(address, prefix) {
				// Found a match! Save it to pending directory
				s.mu.Lock()

				pendingDir := filepath.Join(s.dataDir, "vanity_pending")
				os.MkdirAll(pendingDir, 0700)

				// Save keys to pending directory
				header := []byte("== ed25519v1-secret: type0 ==\x00\x00\x00")
				secretData := append(header, priv.Seed()...)
				secretData = append(secretData, priv[32:]...)
				os.WriteFile(filepath.Join(pendingDir, "hs_ed25519_secret_key"), secretData, 0600)

				pubHeader := []byte("== ed25519v1-public: type0 ==\x00\x00\x00")
				pubData := append(pubHeader, pub...)
				os.WriteFile(filepath.Join(pendingDir, "hs_ed25519_public_key"), pubData, 0600)

				hostname := address + ".onion\n"
				os.WriteFile(filepath.Join(pendingDir, "hostname"), []byte(hostname), 0600)

				if s.vanityStatus != nil {
					s.vanityStatus.Active = false
					s.vanityStatus.Attempts = attempts
					s.vanityStatus.ElapsedTime = time.Since(s.vanityStatus.StartTime).Round(time.Second).String()
				}

				s.mu.Unlock()
				return
			}
		}
	}
}

// CancelVanityGeneration cancels any in-progress vanity generation
func (s *Service) CancelVanityGeneration() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.vanityCancel != nil {
		s.vanityCancel()
		s.vanityCancel = nil
	}
}

// GetVanityStatus returns the current vanity generation status
func (s *Service) GetVanityStatus() *VanityStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.vanityStatus == nil {
		return nil
	}

	// Return a copy
	return &VanityStatus{
		Active:      s.vanityStatus.Active,
		Prefix:      s.vanityStatus.Prefix,
		StartTime:   s.vanityStatus.StartTime,
		Attempts:    s.vanityStatus.Attempts,
		ElapsedTime: s.vanityStatus.ElapsedTime,
	}
}

// ApplyVanityAddress applies the pending vanity address
func (s *Service) ApplyVanityAddress() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pendingDir := filepath.Join(s.dataDir, "vanity_pending")
	siteDir := filepath.Join(s.dataDir, "site")

	// Check if pending keys exist
	if _, err := os.Stat(filepath.Join(pendingDir, "hs_ed25519_secret_key")); os.IsNotExist(err) {
		return fmt.Errorf("no pending vanity address found")
	}

	// Backup current keys
	backupDir := filepath.Join(s.dataDir, "backup_"+time.Now().Format("20060102150405"))
	os.MkdirAll(backupDir, 0700)
	copyFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), filepath.Join(backupDir, "hs_ed25519_secret_key"))
	copyFile(filepath.Join(siteDir, "hs_ed25519_public_key"), filepath.Join(backupDir, "hs_ed25519_public_key"))
	copyFile(filepath.Join(siteDir, "hostname"), filepath.Join(backupDir, "hostname"))

	// Move pending keys to site
	os.Rename(filepath.Join(pendingDir, "hs_ed25519_secret_key"), filepath.Join(siteDir, "hs_ed25519_secret_key"))
	os.Rename(filepath.Join(pendingDir, "hs_ed25519_public_key"), filepath.Join(siteDir, "hs_ed25519_public_key"))
	os.Rename(filepath.Join(pendingDir, "hostname"), filepath.Join(siteDir, "hostname"))

	// Remove pending directory
	os.RemoveAll(pendingDir)

	// Reload keys
	return s.loadOrGenerateKeys()
}

// ImportKeys imports externally generated keys
func (s *Service) ImportKeys(secretKey []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	siteDir := filepath.Join(s.dataDir, "site")

	// Validate key format
	if len(secretKey) < 64 {
		return fmt.Errorf("invalid key format (too short)")
	}

	// Backup current keys
	backupDir := filepath.Join(s.dataDir, "backup_"+time.Now().Format("20060102150405"))
	os.MkdirAll(backupDir, 0700)
	copyFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), filepath.Join(backupDir, "hs_ed25519_secret_key"))
	copyFile(filepath.Join(siteDir, "hs_ed25519_public_key"), filepath.Join(backupDir, "hs_ed25519_public_key"))
	copyFile(filepath.Join(siteDir, "hostname"), filepath.Join(backupDir, "hostname"))

	// Write new secret key
	if err := os.WriteFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), secretKey, 0600); err != nil {
		return fmt.Errorf("failed to write secret key: %w", err)
	}

	// Reload keys
	return s.loadOrGenerateKeys()
}

// GetInfo returns current Tor service info for API/status
func (s *Service) GetInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := map[string]interface{}{
		"enabled": s.cfg.Enabled,
		"status":  string(s.status),
	}

	if s.cfg.Enabled && (s.status == StatusConnected || s.status == StatusNoTorBinary) {
		info["onion_address"] = s.onionAddress
		if s.status == StatusConnected {
			info["uptime"] = s.GetUptime()
			info["process_running"] = true
		} else {
			info["process_running"] = false
			info["note"] = "Tor binary not found - key-only mode"
		}
	}

	if s.vanityStatus != nil && s.vanityStatus.Active {
		info["vanity_generation"] = map[string]interface{}{
			"active":       true,
			"prefix":       s.vanityStatus.Prefix,
			"attempts":     s.vanityStatus.Attempts,
			"elapsed_time": s.vanityStatus.ElapsedTime,
		}
	}

	return info
}

// GetPublicKeyHex returns the public key as hex string
func (s *Service) GetPublicKeyHex() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return hex.EncodeToString(s.publicKey)
}

// Restart restarts the Tor service with new configuration
func (s *Service) Restart(ctx context.Context) error {
	localPort := s.localPort
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Start(ctx, localPort)
}

// TestConnectionResult holds the result of a Tor connection test
type TestConnectionResult struct {
	Connected    bool   `json:"connected"`
	OnionAddress string `json:"onion_address,omitempty"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

// TestConnection tests if the Tor hidden service is working
func (s *Service) TestConnection() *TestConnectionResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := &TestConnectionResult{
		Status: string(s.status),
	}

	// Check if Tor is enabled
	if !s.cfg.Enabled {
		result.Message = "Tor is disabled in configuration"
		return result
	}

	// Check if Tor is running
	if s.status != StatusConnected {
		result.Message = fmt.Sprintf("Tor is not connected (status: %s)", s.status)
		return result
	}

	// Check if we have an onion address
	if s.onionAddress == "" {
		result.Message = "Tor is running but no onion address is available"
		return result
	}

	// Check if the onion service listener is active
	if s.onionSvc == nil {
		result.Message = "Tor onion service listener is not active"
		return result
	}

	// All checks passed
	result.Connected = true
	result.OnionAddress = s.onionAddress
	result.Message = "Tor hidden service is running and accessible"
	return result
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
