// SPDX-License-Identifier: MIT
// Package i2p manages the opt-in I2P eepsite provider described in AI.md PART 31.2.
package i2p

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/logging"
)

// Provider identifies the active I2P provider.
type Provider string

const (
	ProviderNone Provider = "none"
	ProviderI2PD Provider = "i2pd"
	ProviderSAM  Provider = "sam"
)

// Service owns a dedicated eepsite backend and either an i2pd child process
// or an external SAM session. It is disabled unless explicitly configured.
type Service struct {
	configDir string
	dataDir   string
	logDir    string
	cfg       config.I2PConfig
	logger    *logging.AppLogger

	mu          sync.RWMutex
	starting    bool
	listener    net.Listener
	provider    Provider
	address     string
	process     *exec.Cmd
	processDone chan struct{}
	sam         net.Conn
	runCancel   context.CancelFunc
	parentCtx   context.Context
}

// NewService creates an I2P manager without contacting a provider or opening a port.
func NewService(cfg config.I2PConfig, configDir, dataDir, logDir string, logger *logging.AppLogger) *Service {
	return &Service{cfg: cfg, configDir: configDir, dataDir: dataDir, logDir: logDir, logger: logger, provider: ProviderNone}
}

// Start creates the eepsite only when enabled and a provider is available.
func (s *Service) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}
	s.mu.Lock()
	if s.listener != nil || s.starting {
		s.mu.Unlock()
		return nil
	}
	s.starting = true
	runCtx, cancel := context.WithCancel(ctx)
	s.runCancel = cancel
	s.parentCtx = ctx
	s.mu.Unlock()
	failed := func(err error) error {
		cancel()
		s.mu.Lock()
		s.runCancel = nil
		s.parentCtx = nil
		s.starting = false
		s.mu.Unlock()
		return err
	}
	if err := s.ensureDirs(); err != nil {
		return failed(err)
	}
	listener, port, err := newBackendListener()
	if err != nil {
		return failed(err)
	}
	if err := s.startProvider(runCtx, port); err != nil {
		_ = listener.Close()
		return failed(err)
	}
	s.mu.Lock()
	s.listener = listener
	s.starting = false
	s.mu.Unlock()
	return nil
}

func (s *Service) startProvider(ctx context.Context, port int) error {
	provider, binary, err := s.resolveProvider()
	if err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Info("I2P provider selected", map[string]interface{}{"provider": provider})
	}
	if provider == ProviderSAM {
		addr, conn, err := s.startSAM(ctx, port)
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.provider, s.address, s.sam = provider, addr, conn
		s.mu.Unlock()
	} else {
		addr, cmd, err := s.startI2PD(ctx, binary, port)
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.provider, s.address, s.process = provider, addr, cmd
		s.mu.Unlock()
	}
	return nil
}

func (s *Service) resolveProvider() (Provider, string, error) {
	if s.cfg.Binary != "" {
		if st, err := os.Stat(s.cfg.Binary); err == nil && !st.IsDir() {
			return ProviderI2PD, s.cfg.Binary, nil
		}
	}
	for _, candidate := range []string{"/usr/bin/i2pd", "/usr/sbin/i2pd", "/usr/local/bin/i2pd", "/opt/homebrew/bin/i2pd"} {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return ProviderI2PD, candidate, nil
		}
	}
	if binary, err := exec.LookPath("i2pd"); err == nil {
		return ProviderI2PD, binary, nil
	}
	conn, err := net.DialTimeout("tcp", s.cfg.SAMAddress, 3*time.Second)
	if err == nil {
		_ = conn.Close()
		return ProviderSAM, "", nil
	}
	return ProviderNone, "", fmt.Errorf("no I2P provider found (i2pd unavailable and SAM %s unreachable)", s.cfg.SAMAddress)
}

func (s *Service) ensureDirs() error {
	for _, dir := range []string{
		filepath.Join(s.configDir, "i2p"),
		filepath.Join(s.dataDir, "i2p"),
		filepath.Join(s.dataDir, "i2p", "site"),
	} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create I2P directory: %w", err)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			return fmt.Errorf("set I2P directory permissions: %w", err)
		}
	}
	return nil
}

func newBackendListener() (net.Listener, int, error) {
	span := 1000
	for attempt := 0; attempt < 50; attempt++ {
		var random [4]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, 0, fmt.Errorf("select I2P backend port: %w", err)
		}
		port := 64000 + int(uint32(random[0])<<24|uint32(random[1])<<16|uint32(random[2])<<8|uint32(random[3]))%span
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			return listener, port, nil
		}
	}
	return nil, 0, fmt.Errorf("unable to allocate I2P backend port")
}

func (s *Service) startSAM(ctx context.Context, port int) (string, net.Conn, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.cfg.SAMAddress)
	if err != nil {
		return "", nil, fmt.Errorf("connect to I2P SAM: %w", err)
	}
	finished := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-finished:
		}
	}()
	defer close(finished)
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		_ = conn.Close()
		return "", nil, fmt.Errorf("set I2P SAM deadline: %w", err)
	}
	r := bufio.NewReader(conn)
	if _, err := io.WriteString(conn, "HELLO VERSION MIN=3.0 MAX=3.3\n"); err != nil {
		_ = conn.Close()
		return "", nil, err
	}
	if _, err := readSAMReply(r, "HELLO REPLY"); err != nil {
		_ = conn.Close()
		return "", nil, err
	}
	keyPath := filepath.Join(s.dataDir, "i2p", "site", "site-keys.dat")
	private, public, err := s.loadOrCreateSAMKeys(conn, r, keyPath, s.cfg.SignatureType)
	if err != nil {
		_ = conn.Close()
		return "", nil, err
	}
	if _, err := fmt.Fprintf(conn, "SESSION CREATE STYLE=STREAM ID=vidveil DESTINATION=%s inbound.length=%d outbound.length=%d inbound.quantity=%d outbound.quantity=%d\n", private, s.cfg.InboundLength, s.cfg.OutboundLength, s.cfg.InboundQuantity, s.cfg.OutboundQuantity); err != nil {
		_ = conn.Close()
		return "", nil, err
	}
	if _, err := readSAMReply(r, "SESSION STATUS"); err != nil {
		_ = conn.Close()
		return "", nil, err
	}
	if _, err := fmt.Fprintf(conn, "STREAM FORWARD ID=vidveil PORT=%d HOST=127.0.0.1\n", port); err != nil {
		_ = conn.Close()
		return "", nil, err
	}
	if _, err := readSAMReply(r, "STREAM STATUS"); err != nil {
		_ = conn.Close()
		return "", nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return "", nil, fmt.Errorf("clear I2P SAM deadline: %w", err)
	}
	return b32Address(public), conn, nil
}

func (s *Service) loadOrCreateSAMKeys(conn net.Conn, reader *bufio.Reader, keyPath string, signatureType int) (string, []byte, error) {
	info, statErr := os.Lstat(keyPath)
	if statErr == nil {
		if !info.Mode().IsRegular() {
			return "", nil, errors.New("I2P destination key is not a regular file")
		}
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return "", nil, fmt.Errorf("read I2P destination keys: %w", err)
		}
		if err := os.Chmod(keyPath, 0600); err != nil {
			return "", nil, fmt.Errorf("secure I2P destination key permissions: %w", err)
		}
		parts := strings.Fields(string(data))
		if len(parts) == 2 {
			public, err := decodeSAMBase64(parts[1])
			if err == nil {
				return parts[0], public, nil
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", nil, fmt.Errorf("inspect I2P destination keys: %w", statErr)
	}
	if _, err := fmt.Fprintf(conn, "DEST GENERATE SIGNATURE_TYPE=%d\n", signatureType); err != nil {
		return "", nil, err
	}
	reply, err := readSAMReply(reader, "DEST REPLY")
	if err != nil {
		return "", nil, err
	}
	private, public := reply["PRIV"], reply["PUB"]
	if private == "" || public == "" {
		return "", nil, errors.New("SAM destination response omitted public or private destination")
	}
	publicBytes, err := decodeSAMBase64(public)
	if err != nil {
		return "", nil, fmt.Errorf("decode SAM public destination: %w", err)
	}
	file, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", nil, fmt.Errorf("create I2P destination key: %w", err)
	}
	_, writeErr := fmt.Fprintln(file, private, public)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(keyPath)
		return "", nil, errors.Join(writeErr, closeErr)
	}
	return private, publicBytes, nil
}

func decodeSAMBase64(value string) ([]byte, error) {
	value = strings.NewReplacer("-", "+", "~", "/").Replace(value)
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return base64.RawURLEncoding.DecodeString(value)
}

func readSAMReply(reader *bufio.Reader, prefix string) (map[string]string, error) {
	lineBytes, err := reader.ReadSlice('\n')
	if err != nil {
		return nil, fmt.Errorf("read SAM reply: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(lineBytes)))
	if len(fields) < 2 || fields[0]+" "+fields[1] != prefix {
		return nil, fmt.Errorf("unexpected SAM reply %q", strings.TrimSpace(string(lineBytes)))
	}
	values := make(map[string]string)
	for _, field := range fields[2:] {
		key, value, ok := strings.Cut(field, "=")
		if ok {
			values[key] = value
		}
	}
	if status := values["RESULT"]; status != "" && status != "OK" {
		return nil, fmt.Errorf("SAM returned %s", status)
	}
	return values, nil
}

func b32Address(destination []byte) string {
	digest := sha256.Sum256(destination)
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:])) + ".b32.i2p"
}

func (s *Service) startI2PD(ctx context.Context, binary string, port int) (string, *exec.Cmd, error) {
	configDir := filepath.Join(s.configDir, "i2p")
	dataDir := filepath.Join(s.dataDir, "i2p")
	keysPath := filepath.Join(dataDir, "site", "site-keys.dat")
	tunnelsPath := filepath.Join(configDir, "tunnels.conf")
	tunnels := fmt.Sprintf("[site]\ntype = server\nhost = 127.0.0.1\nport = %d\nkeys = %q\ninbound.length = %d\noutbound.length = %d\ninbound.quantity = %d\noutbound.quantity = %d\nsignaturetype = %d\n", port, keysPath, s.cfg.InboundLength, s.cfg.OutboundLength, s.cfg.InboundQuantity, s.cfg.OutboundQuantity, s.cfg.SignatureType)
	if err := os.WriteFile(tunnelsPath, []byte(tunnels), 0600); err != nil {
		return "", nil, fmt.Errorf("write I2P tunnels configuration: %w", err)
	}
	if err := os.Chmod(tunnelsPath, 0600); err != nil {
		return "", nil, fmt.Errorf("secure I2P tunnels configuration: %w", err)
	}
	args := []string{"--datadir", dataDir, "--tunconf", tunnelsPath, "--log", "file"}
	if s.logDir != "" {
		args = append(args, "--logfile", filepath.Join(s.logDir, "i2pd.log"))
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start i2pd: %w", err)
	}
	pidPath := filepath.Join(dataDir, "i2pd.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0600); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", nil, fmt.Errorf("write i2pd PID file: %w", err)
	}
	done := make(chan struct{})
	s.mu.Lock()
	s.process, s.processDone = cmd, done
	s.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		_ = os.Remove(pidPath)
		close(done)
	}()
	deadline, err := time.ParseDuration(s.cfg.BootstrapTimeout)
	if err != nil {
		deadline = 5 * time.Minute
	}
	until := time.NewTimer(deadline)
	defer until.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Signal(os.Interrupt)
			<-done
			return "", nil, ctx.Err()
		case <-until.C:
			_ = cmd.Process.Signal(os.Interrupt)
			stopProcess(cmd, done)
			return "", nil, errors.New("i2pd destination startup timed out")
		case <-ticker.C:
			// i2pd publishes the derived address alongside the persisted key.
			for _, name := range []string{"hostname", "site-keys.dat.hostname"} {
				if data, readErr := os.ReadFile(filepath.Join(dataDir, "site", name)); readErr == nil {
					address := strings.TrimSpace(string(data))
					if strings.HasSuffix(address, ".b32.i2p") {
						keyInfo, statErr := os.Lstat(keysPath)
						if statErr != nil || !keyInfo.Mode().IsRegular() {
							_ = cmd.Process.Signal(os.Interrupt)
							stopProcess(cmd, done)
							return "", nil, errors.New("i2pd destination key is not a regular file")
						}
						if err := os.Chmod(keysPath, 0600); err != nil {
							_ = cmd.Process.Signal(os.Interrupt)
							stopProcess(cmd, done)
							return "", nil, fmt.Errorf("secure i2pd destination key permissions: %w", err)
						}
						return address, cmd, nil
					}
				}
			}
		}
	}
}

func stopProcess(cmd *exec.Cmd, done <-chan struct{}) {
	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	<-done
}

// Listener returns the plain loopback backend listener while the service runs.
func (s *Service) Listener() net.Listener { s.mu.RLock(); defer s.mu.RUnlock(); return s.listener }

// Enabled reports the configured opt-in state.
func (s *Service) Enabled() bool { return s.cfg.Enabled }

// IsRunning reports whether the eepsite has an active backend and provider.
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil || s.address == "" {
		return false
	}
	if s.provider == ProviderI2PD && s.processDone != nil {
		select {
		case <-s.processDone:
			return false
		default:
		}
	}
	return true
}

// Status reports disabled, healthy, or a non-sensitive provider state.
func (s *Service) Status() string {
	if !s.cfg.Enabled {
		return "disabled"
	}
	if s.IsRunning() {
		return "healthy"
	}
	return "error:no_provider"
}

// CheckHealth verifies that the active provider still owns its destination.
func (s *Service) CheckHealth(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil || s.address == "" {
		return errors.New("I2P eepsite is not running")
	}
	if s.provider == ProviderI2PD {
		if s.processDone == nil {
			return errors.New("i2pd process state is unavailable")
		}
		select {
		case <-s.processDone:
			return errors.New("i2pd process exited")
		default:
			return nil
		}
	}
	if s.provider != ProviderSAM || s.sam == nil {
		return errors.New("I2P provider is unavailable")
	}
	deadline := time.Now().Add(3 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := s.sam.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set SAM health deadline: %w", err)
	}
	defer func() { _ = s.sam.SetDeadline(time.Time{}) }()
	if _, err := io.WriteString(s.sam, "PING\n"); err != nil {
		return fmt.Errorf("ping SAM provider: %w", err)
	}
	line, err := bufio.NewReader(s.sam).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read SAM health response: %w", err)
	}
	if strings.TrimSpace(line) != "PONG" {
		return fmt.Errorf("unexpected SAM health response %q", strings.TrimSpace(line))
	}
	return nil
}

// Address returns the public-safe .b32.i2p hostname.
func (s *Service) Address() string { s.mu.RLock(); defer s.mu.RUnlock(); return s.address }

// Provider returns i2pd, sam, or none.
func (s *Service) Provider() string { s.mu.RLock(); defer s.mu.RUnlock(); return string(s.provider) }

// Restart replaces a failed provider and its backend using the current config.
func (s *Service) Restart(ctx context.Context) error {
	s.mu.RLock()
	listener := s.listener
	starting := s.starting
	s.mu.RUnlock()
	if listener == nil {
		if starting {
			return nil
		}
		return s.Start(ctx)
	}
	if err := s.stopProvider(); err != nil {
		return err
	}
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return fmt.Errorf("read I2P backend port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return fmt.Errorf("parse I2P backend port: %w", err)
	}
	s.mu.RLock()
	parentCtx := s.parentCtx
	s.mu.RUnlock()
	if parentCtx == nil {
		parentCtx = ctx
	}
	runCtx, cancel := context.WithCancel(parentCtx)
	s.mu.Lock()
	s.runCancel = cancel
	s.mu.Unlock()
	if err := s.startProvider(runCtx, port); err != nil {
		cancel()
		s.mu.Lock()
		s.runCancel = nil
		s.mu.Unlock()
		return err
	}
	return nil
}

// Close stops the managed provider and closes the backend listener.
func (s *Service) Close() error {
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()
	closeErr := s.stopProvider()
	if listener != nil {
		closeErr = errors.Join(closeErr, listener.Close())
	}
	return closeErr
}

func (s *Service) stopProvider() error {
	s.mu.Lock()
	conn, process, processDone, cancel := s.sam, s.process, s.processDone, s.runCancel
	s.sam, s.process, s.processDone = nil, nil, nil
	s.runCancel = nil
	s.address = ""
	s.provider = ProviderNone
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	var closeErr error
	if conn != nil {
		closeErr = errors.Join(closeErr, conn.Close())
	}
	if process != nil && process.Process != nil {
		if err := process.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			closeErr = errors.Join(closeErr, err)
		}
		if processDone != nil {
			select {
			case <-processDone:
			case <-time.After(5 * time.Second):
				if err := process.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					closeErr = errors.Join(closeErr, err)
				}
				<-processDone
			}
		}
	}
	return closeErr
}
