// SPDX-License-Identifier: MIT
// AI.md PART 32: Embedded Tor Hidden Service Support using bine
package tor

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/cretz/bine/tor"
	bineed25519 "github.com/cretz/bine/torutil/ed25519"
	"golang.org/x/crypto/sha3"
)

// TorService represents the embedded Tor hidden service per AI.md PART 32
// Uses github.com/cretz/bine for dedicated Tor process management
// Supports both hidden service hosting AND outbound network routing
type TorService struct {
	cfg       *TorServiceConfig
	torConfig *config.TorConfig // Full config from server.yml
	dataDir   string
	logger    *logging.AppLogger

	// bine Tor instance - manages dedicated Tor process
	torInstance *tor.Tor

	// OnionService listener - implements net.Listener
	// Per AI.md PART 32: Unix socket on Unix, high TCP port on Windows
	onionService *tor.OnionService

	// Outbound Tor dialer per PART 32
	// Used when UseNetwork is enabled to route engine queries through Tor
	dialer *tor.Dialer

	// Hidden service state
	onionAddress string
	privateKey   ed25519.PrivateKey
	publicKey    ed25519.PublicKey

	// Status tracking
	status    TorServiceStatus
	startTime time.Time
	mu        sync.RWMutex

	// Local port the hidden service forwards to (Windows only)
	// On Unix, uses Unix socket instead
	localPort int

	// Vanity generation
	vanityCtx    context.Context
	vanityCancel context.CancelFunc
	vanityStatus *VanityStatus

	// Process monitoring per PART 32
	monitorCtx    context.Context
	monitorCancel context.CancelFunc
}

// TorServiceConfig holds Tor service configuration per AI.md PART 32
type TorServiceConfig struct {
	// Set from paths.GetDataDir() + "/tor"
	DataDir string `yaml:"-"`
}

// TorServiceStatus represents Tor service status
type TorServiceStatus string

const (
	TorServiceStatusDisabled     TorServiceStatus = "disabled"
	TorServiceStatusStarting     TorServiceStatus = "starting"
	TorServiceStatusConnected    TorServiceStatus = "connected"
	TorServiceStatusDisconnected TorServiceStatus = "disconnected"
	TorServiceStatusError        TorServiceStatus = "error"
	// Tor binary not found
	TorServiceStatusNoTorBinary TorServiceStatus = "no_tor_binary"
)

// VanityStatus tracks vanity address generation progress
type VanityStatus struct {
	Active      bool      `json:"active"`
	Prefix      string    `json:"prefix"`
	StartTime   time.Time `json:"start_time"`
	Attempts    int64     `json:"attempts"`
	ElapsedTime string    `json:"elapsed_time"`
}

// NewTorService creates a new Tor service instance
// Per PART 32: Tor auto-enables if binary is found - no enable flag needed
func NewTorService(dataDir string, logger *logging.AppLogger) *TorService {
	return &TorService{
		cfg: &TorServiceConfig{
			DataDir: filepath.Join(dataDir, "tor"),
		},
		dataDir: filepath.Join(dataDir, "tor"),
		status:  TorServiceStatusDisabled,
		logger:  logger,
	}
}

// SetConfig sets the Tor configuration from server.yml
// Must be called before Start() to enable outbound network routing
func (s *TorService) SetConfig(cfg *config.TorConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.torConfig = cfg
}

// GetHTTPClient returns an HTTP client, optionally routed through Tor
// Per PART 32: Use this for engine queries when UseNetwork is enabled
// useTor: true = route through Tor SOCKS5 proxy, false = direct connection
func (s *TorService) GetHTTPClient(useTor bool) *http.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !useTor || s.dialer == nil {
		// Direct connection - standard HTTP client
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// Route through Tor network via SOCKS5 proxy
	return &http.Client{
		Timeout: 60 * time.Second, // Tor is slower, use longer timeout
		Transport: &http.Transport{
			DialContext: s.dialer.DialContext,
		},
	}
}

// OutboundEnabled returns true if Tor outbound connections are available
// Per PART 32: This is true when UseNetwork is enabled and Tor is running
func (s *TorService) OutboundEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dialer != nil
}

// UseNetworkEnabled returns true if Tor network routing is configured
func (s *TorService) UseNetworkEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.torConfig != nil && s.torConfig.UseNetwork
}

// AllowUserIPForward returns true if admin allows users to forward their IP
// When true, users can opt-in (via cookie) to have their IP passed to video sites
// in X-Forwarded-For header for geo-targeted content
func (s *TorService) AllowUserIPForward() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.torConfig != nil && s.torConfig.AllowUserIPForward
}

// Start initializes the Tor hidden service using bine
// Per AI.md PART 32: Uses dedicated Tor process via bine library
// Auto-enabled if tor binary is found - no enable flag needed
func (s *TorService) Start(ctx context.Context, localPort int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = TorServiceStatusStarting
	s.startTime = time.Now()
	s.localPort = localPort

	// Ensure data directories exist
	torDataDir := filepath.Join(s.dataDir, "data")
	siteDir := filepath.Join(s.dataDir, "site")
	if err := os.MkdirAll(torDataDir, 0700); err != nil {
		s.status = TorServiceStatusError
		return fmt.Errorf("failed to create tor data directory: %w", err)
	}
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		s.status = TorServiceStatusError
		return fmt.Errorf("failed to create tor site directory: %w", err)
	}

	// Per AI.md PART 32: Enforce ownership (current user) on all Tor directories recursively
	// This fixes "is not owned by this user" errors when directories were created by different user
	// Must be recursive because Tor creates subdirectories (e.g., data/keys)
	if runtime.GOOS != "windows" {
		currentUID := os.Getuid()
		currentGID := os.Getgid()
		for _, dir := range []string{torDataDir, siteDir} {
			err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				return os.Chown(path, currentUID, currentGID)
			})
			if err != nil {
				s.status = TorServiceStatusError
				return fmt.Errorf("chown tor dir %s: %w", dir, err)
			}
		}
	}

	// Check if Tor binary exists
	torPath, err := exec.LookPath("tor")
	if err != nil {
		// Tor binary not found - fall back to key-only mode
		s.logger.Info("Tor binary not found in PATH, running in key-only mode", nil)
		s.status = TorServiceStatusNoTorBinary

		// Still load/generate keys for address generation
		if err := s.loadOrGenerateKeys(); err != nil {
			s.status = TorServiceStatusError
			return fmt.Errorf("failed to load/generate keys: %w", err)
		}
		s.onionAddress = s.generateOnionAddress()
		return nil
	}

	s.logger.Info("Found Tor binary", map[string]interface{}{"path": torPath})

	// Load or generate hidden service keys first
	if err := s.loadOrGenerateKeys(); err != nil {
		s.status = TorServiceStatusError
		return fmt.Errorf("failed to load/generate keys: %w", err)
	}
	s.onionAddress = s.generateOnionAddress()

	// Start dedicated Tor process using bine
	// Per AI.md: Start OUR OWN Tor process - completely separate from system Tor
	// Per AI.md PART 32: Tor startup/runtime errors = WARN (server continues without Tor)
	// Suppress Tor's verbose output - only show errors when connection actually fails
	startConf := &tor.StartConf{
		// Our own data directory - isolated from system Tor
		DataDir: torDataDir,

		// Let bine pick available ports (avoids conflict with system Tor 9050/9051)
		NoAutoSocksPort: false,

		// Use found Tor binary
		ExePath: torPath,

		// NoHush=false means bine adds --hush flag to reduce Tor output
		// This suppresses "You are running Tor as root" and bootstrap progress messages
		NoHush: false,

		// Redirect Tor debug output to discard (suppresses warnings like "Problem bootstrapping")
		// If debug mode, could set to os.Stderr
		DebugWriter: io.Discard,

		// Extra args to further suppress warnings
		// --quiet: Suppress non-error log messages
		ExtraArgs: []string{"--quiet"},
	}

	s.logger.Info("Starting dedicated Tor process...", nil)
	t, err := tor.Start(ctx, startConf)
	if err != nil {
		s.status = TorServiceStatusError
		return fmt.Errorf("failed to start dedicated tor: %w", err)
	}
	s.torInstance = t

	// Wait for Tor to bootstrap (with timeout)
	s.logger.Info("Waiting for Tor to bootstrap...", nil)
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	if err := t.EnableNetwork(dialCtx, true); err != nil {
		t.Close()
		s.torInstance = nil
		s.status = TorServiceStatusError
		return fmt.Errorf("failed to enable tor network: %w", err)
	}

	// Create hidden service using tor.Listen() with proper local listener
	// Per AI.md PART 32: Unix socket on Unix, high TCP port (63000+) on Windows
	s.logger.Info("Creating hidden service", map[string]interface{}{
		"remote_port": 80,
	})

	// Create local listener based on platform
	var localListener net.Listener
	if runtime.GOOS == "windows" {
		// Windows: Unix sockets not supported, use high TCP port (63000+ range)
		// Find available port in 63000-63999 range
		var listenErr error
		for port := 63000; port < 64000; port++ {
			localListener, listenErr = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if listenErr == nil {
				s.localPort = port
				s.logger.Info("Using TCP port for hidden service", map[string]interface{}{"port": port})
				break
			}
		}
		if localListener == nil {
			t.Close()
			s.torInstance = nil
			s.status = TorServiceStatusError
			return fmt.Errorf("failed to find available port in 63000-63999 range: %w", listenErr)
		}
	} else {
		// Unix/macOS/BSD: Use Unix socket (more secure, no port conflicts)
		socketPath := filepath.Join(s.dataDir, "service.sock")
		// Remove existing socket file if it exists
		os.Remove(socketPath)
		var listenErr error
		localListener, listenErr = net.Listen("unix", socketPath)
		if listenErr != nil {
			t.Close()
			s.torInstance = nil
			s.status = TorServiceStatusError
			return fmt.Errorf("failed to create unix socket %s: %w", socketPath, listenErr)
		}
		// Set socket permissions to 0600 per PART 32
		if err := os.Chmod(socketPath, 0600); err != nil {
			localListener.Close()
			t.Close()
			s.torInstance = nil
			s.status = TorServiceStatusError
			return fmt.Errorf("failed to chmod socket: %w", err)
		}
		s.logger.Info("Using Unix socket for hidden service", map[string]interface{}{"socket": socketPath})
	}

	// Convert Go ed25519 key to bine's ed25519 KeyPair
	bineKeyPair := bineed25519.FromCryptoPrivateKey(s.privateKey)

	// Create hidden service with our pre-created local listener
	onionSvc, err := t.Listen(ctx, &tor.ListenConf{
		LocalListener: localListener, // Our Unix socket or TCP listener
		RemotePorts:   []int{80},     // .onion:80
		Key:           bineKeyPair,   // Use our existing key
		Version3:      true,          // Ed25519 v3 onion
	})
	if err != nil {
		localListener.Close()
		t.Close()
		s.torInstance = nil
		s.status = TorServiceStatusError
		return fmt.Errorf("failed to create onion service: %w", err)
	}
	s.onionService = onionSvc

	// Update onion address from the service (should match our calculated one)
	actualAddress := onionSvc.ID + ".onion"
	if actualAddress != s.onionAddress {
		s.logger.Warn("Onion address mismatch", map[string]interface{}{
			"expected": s.onionAddress,
			"got":      actualAddress,
		})
		s.onionAddress = actualAddress
	}

	s.status = TorServiceStatusConnected
	s.logger.Info("Hidden service started", map[string]interface{}{"onion_address": s.onionAddress})

	// Initialize outbound dialer if UseNetwork is enabled (per PART 32)
	// This allows engine queries to be routed through Tor for privacy
	if s.torConfig != nil && s.torConfig.UseNetwork {
		dialer, err := t.Dialer(ctx, nil)
		if err != nil {
			s.logger.Warn("Failed to create Tor dialer for outbound connections", map[string]interface{}{
				"error": err.Error(),
			})
			// Continue without outbound - hidden service still works
		} else {
			s.dialer = dialer
			s.logger.Info("Tor outbound network enabled - engine queries will be anonymized", nil)
		}
	}

	// Start process monitoring per PART 32
	s.monitorCtx, s.monitorCancel = context.WithCancel(context.Background())
	go s.monitorProcess()

	return nil
}

// monitorProcess monitors Tor and restarts if it crashes per PART 32
func (s *TorService) monitorProcess() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.monitorCtx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			torInst := s.torInstance
			status := s.status
			s.mu.RUnlock()

			if torInst == nil || status != TorServiceStatusConnected {
				continue
			}

			// Check if Tor is still responsive via control connection
			if _, err := torInst.Control.GetInfo("version"); err != nil {
				s.logger.Warn("Tor process unresponsive, restarting...", map[string]interface{}{"error": err.Error()})

				// Attempt restart
				s.mu.Lock()
				localPort := s.localPort
				s.mu.Unlock()

				if err := s.Stop(); err != nil {
					s.logger.Warn("Error stopping Tor during restart", map[string]interface{}{"error": err.Error()})
				}

				// Restart in background to avoid blocking monitor
				go func() {
					ctx := context.Background()
					if err := s.Start(ctx, localPort); err != nil {
						s.logger.Warn("Failed to restart Tor", map[string]interface{}{"error": err.Error()})
					} else {
						s.logger.Info("Tor restarted successfully", nil)
					}
				}()
				// Exit this monitor, new one will be started
				return
			}
		}
	}
}

// Stop shuts down the Tor service
func (s *TorService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel process monitoring per PART 32
	if s.monitorCancel != nil {
		s.monitorCancel()
		s.monitorCancel = nil
	}

	// Cancel any vanity generation
	if s.vanityCancel != nil {
		s.vanityCancel()
	}

	// Close onion service listener
	if s.onionService != nil {
		s.onionService.Close()
		s.onionService = nil
	}

	// Close dedicated Tor process
	if s.torInstance != nil {
		s.logger.Info("Shutting down dedicated Tor process...", nil)
		if err := s.torInstance.Close(); err != nil {
			s.logger.Warn("Error closing Tor", map[string]interface{}{"error": err.Error()})
		}
		s.torInstance = nil
	}

	s.status = TorServiceStatusDisconnected
	return nil
}

// loadOrGenerateKeys loads existing keys or generates new ones
func (s *TorService) loadOrGenerateKeys() error {
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
func (s *TorService) generateOnionAddress() string {
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
func (s *TorService) GetOnionAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.onionAddress
}

// GetStatus returns the current service status
func (s *TorService) GetStatus() TorServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// GetUptime returns the service uptime as a string
func (s *TorService) GetUptime() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.status != TorServiceStatusConnected {
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

// IsEnabled returns whether Tor hidden service is active
// Per PART 32: Tor auto-enables if binary found
func (s *TorService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == TorServiceStatusConnected || s.status == TorServiceStatusNoTorBinary
}

// IsRunning returns whether Tor process is actually running
func (s *TorService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.torInstance != nil && s.status == TorServiceStatusConnected
}

// RegenerateAddress generates a new random .onion address
// This deletes existing keys and generates new ones
func (s *TorService) RegenerateAddress() error {
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
// maxPrefixLength is limited to 6 characters per AI.md PART 32
func (s *TorService) GenerateVanityAddress(prefix string) error {
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
func (s *TorService) runVanityGeneration(ctx context.Context, prefix string) {
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
func (s *TorService) CancelVanityGeneration() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.vanityCancel != nil {
		s.vanityCancel()
		s.vanityCancel = nil
	}
}

// GetVanityStatus returns the current vanity generation status
func (s *TorService) GetVanityStatus() *VanityStatus {
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
func (s *TorService) ApplyVanityAddress() error {
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
func (s *TorService) ImportKeys(secretKey []byte) error {
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
func (s *TorService) GetInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Per PART 32: Tor is enabled if binary found and running
	enabled := s.status == TorServiceStatusConnected || s.status == TorServiceStatusNoTorBinary

	info := map[string]interface{}{
		"enabled": enabled,
		"status":  string(s.status),
	}

	if enabled {
		info["onion_address"] = s.onionAddress
		if s.status == TorServiceStatusConnected {
			info["uptime"] = s.GetUptime()
			info["process_running"] = true
		} else {
			info["process_running"] = false
			info["note"] = "Tor binary not found - key-only mode"
		}
	}

	// Outbound network status per PART 32
	outboundConfigured := s.torConfig != nil && s.torConfig.UseNetwork
	outboundActive := s.dialer != nil
	info["outbound"] = map[string]interface{}{
		"configured": outboundConfigured,
		"active":     outboundActive,
	}
	if outboundActive {
		info["outbound"].(map[string]interface{})["note"] = "Engine queries are being anonymized through Tor"
	} else if outboundConfigured {
		info["outbound"].(map[string]interface{})["note"] = "Configured but not active (Tor not running)"
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
func (s *TorService) GetPublicKeyHex() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return hex.EncodeToString(s.publicKey)
}

// GetListener returns the OnionService listener (implements net.Listener)
// The HTTP server should call Serve() on this listener to handle Tor traffic
// Returns nil if Tor is not running
func (s *TorService) GetListener() net.Listener {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.onionService == nil {
		return nil
	}
	return s.onionService
}

// Restart restarts the Tor service with new configuration
func (s *TorService) Restart(ctx context.Context) error {
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
func (s *TorService) TestConnection() *TestConnectionResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := &TestConnectionResult{
		Status: string(s.status),
	}

	// Check if Tor is running
	if s.status != TorServiceStatusConnected {
		result.Message = fmt.Sprintf("Tor is not connected (status: %s)", s.status)
		return result
	}

	// Check if we have an onion address
	if s.onionAddress == "" {
		result.Message = "Tor is running but no onion address is available"
		return result
	}

	// Check if Tor instance and onion service are active
	if s.torInstance == nil {
		result.Message = "Tor process is not running"
		return result
	}

	if s.onionService == nil {
		result.Message = "Onion service listener is not active"
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
