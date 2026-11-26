// SPDX-License-Identifier: MIT
// Package i2p manages the I2P eepsite provider described in AI.md PART 31.2.
package i2p

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
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
	state       string
	address     string
	process     *exec.Cmd
	processDone chan struct{}
	sam         net.Conn
	forward     net.Conn
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
	s.state = "starting"
	runCtx, cancel := context.WithCancel(ctx)
	s.runCancel = cancel
	s.parentCtx = ctx
	s.mu.Unlock()
	failed := func(err error) error {
		cancel()
		s.mu.Lock()
		if s.state != "no_provider" {
			s.state = "error"
		}
		s.runCancel = nil
		s.parentCtx = nil
		s.starting = false
		s.mu.Unlock()
		return err
	}
	provider, binary, err := s.resolveProvider()
	if err != nil {
		s.setState("no_provider")
		return failed(err)
	}
	s.mu.Lock()
	s.provider = provider
	s.mu.Unlock()
	if err := s.ensureDirs(); err != nil {
		return failed(err)
	}
	listener, port, err := newBackendListener()
	if err != nil {
		return failed(err)
	}
	if err := s.startProviderResolved(runCtx, provider, binary, port); err != nil {
		_ = listener.Close()
		s.setState("error")
		return failed(err)
	}
	s.mu.Lock()
	s.listener = listener
	s.starting = false
	s.state = "running"
	s.mu.Unlock()
	return nil
}

func (s *Service) startProvider(ctx context.Context, port int) error {
	provider, binary, err := s.resolveProvider()
	if err != nil {
		s.setState("no_provider")
		return err
	}
	if err := s.startProviderResolved(ctx, provider, binary, port); err != nil {
		s.setState("error")
		return err
	}
	return nil
}

func (s *Service) startProviderResolved(ctx context.Context, provider Provider, binary string, port int) error {
	s.setState("starting")
	if s.logger != nil {
		s.logger.Info("I2P provider selected", map[string]interface{}{"provider": provider})
	}
	samAddress := s.cfg.SAMAddress
	var cmd *exec.Cmd
	if provider == ProviderI2PD {
		var err error
		samAddress, cmd, err = s.startI2PD(ctx, binary)
		if err != nil {
			return err
		}
	}
	addr, control, forward, err := s.startSAM(ctx, samAddress, port)
	if err != nil {
		if cmd != nil {
			s.stopStartedProcess(cmd)
		}
		return err
	}
	s.mu.Lock()
	s.provider, s.address, s.sam, s.forward, s.process, s.state = provider, addr, control, forward, cmd, "running"
	s.mu.Unlock()
	go s.monitorSAMConnection(control)
	go s.monitorSAMConnection(forward)
	return nil
}

func (s *Service) monitorSAMConnection(conn net.Conn) {
	var probe [1]byte
	_, _ = conn.Read(probe[:])
	s.mu.Lock()
	if s.sam == conn || s.forward == conn {
		s.state = "error"
	}
	s.mu.Unlock()
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
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		if err = samHello(conn); err == nil {
			_, _ = io.WriteString(conn, "QUIT\n")
			_ = conn.Close()
			return ProviderSAM, "", nil
		}
		_ = conn.Close()
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
			accepting, err := newAcceptingListener(listener)
			if err != nil {
				_ = listener.Close()
				return nil, 0, err
			}
			return accepting, port, nil
		}
	}
	return nil, 0, fmt.Errorf("unable to allocate I2P backend port")
}

type acceptingListener struct {
	listener net.Listener
	accepted chan net.Conn
	errors   chan error
	done     chan struct{}
	pumpDone chan struct{}
	close    sync.Once
}

func newAcceptingListener(listener net.Listener) (*acceptingListener, error) {
	wrapped := &acceptingListener{
		listener: listener,
		accepted: make(chan net.Conn, 32),
		errors:   make(chan error, 1),
		done:     make(chan struct{}),
		pumpDone: make(chan struct{}),
	}
	go wrapped.acceptLoop()
	probe, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		_ = wrapped.Close()
		return nil, fmt.Errorf("prime I2P backend accept loop: %w", err)
	}
	select {
	case accepted := <-wrapped.accepted:
		_ = accepted.Close()
		_ = probe.Close()
		return wrapped, nil
	case err := <-wrapped.errors:
		_ = probe.Close()
		_ = wrapped.Close()
		return nil, fmt.Errorf("start I2P backend accept loop: %w", err)
	case <-time.After(time.Second):
		_ = probe.Close()
		_ = wrapped.Close()
		return nil, errors.New("I2P backend accept loop did not accept its readiness connection")
	}
}

func (l *acceptingListener) acceptLoop() {
	defer close(l.pumpDone)
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			select {
			case l.errors <- err:
			case <-l.done:
			}
			return
		}
		select {
		case l.accepted <- conn:
		case <-l.done:
			_ = conn.Close()
			return
		}
	}
}

func (l *acceptingListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.accepted:
		return conn, nil
	case err := <-l.errors:
		return nil, err
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *acceptingListener) Close() error {
	var err error
	l.close.Do(func() {
		close(l.done)
		err = l.listener.Close()
		<-l.pumpDone
		for {
			select {
			case conn := <-l.accepted:
				_ = conn.Close()
			default:
				return
			}
		}
	})
	return err
}

func (l *acceptingListener) Addr() net.Addr { return l.listener.Addr() }

func (s *Service) startSAM(ctx context.Context, address string, port int) (string, net.Conn, net.Conn, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "", nil, nil, fmt.Errorf("connect to I2P SAM: %w", err)
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
		return "", nil, nil, fmt.Errorf("set I2P SAM deadline: %w", err)
	}
	r := bufio.NewReader(conn)
	if err := samHelloWithReader(conn, r); err != nil {
		_ = conn.Close()
		return "", nil, nil, err
	}
	keyPath := filepath.Join(s.dataDir, "i2p", "site", "site-keys.dat")
	private, destination, err := s.loadOrCreateSAMKeys(conn, r, keyPath, s.cfg.SignatureType)
	if err != nil {
		_ = conn.Close()
		return "", nil, nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(s.bootstrapTimeout())); err != nil {
		_ = conn.Close()
		return "", nil, nil, err
	}
	s.setState("building")
	sessionID := fmt.Sprintf("site-%d-%x", os.Getpid(), randomSessionSuffix())
	command := func(id string) string {
		return fmt.Sprintf("SESSION CREATE STYLE=STREAM ID=%s DESTINATION=%s i2cp.leaseSetEncType=4 inbound.length=%d outbound.length=%d inbound.quantity=%d outbound.quantity=%d\n", id, private, s.cfg.InboundLength, s.cfg.OutboundLength, s.cfg.InboundQuantity, s.cfg.OutboundQuantity)
	}
	if _, err := io.WriteString(conn, command(sessionID)); err != nil {
		_ = conn.Close()
		return "", nil, nil, fmt.Errorf("write SESSION CREATE: %w", err)
	}
	_, sessionErr := readSAMReply(r, "SESSION STATUS")
	if sessionErr != nil && strings.Contains(sessionErr.Error(), "DUPLICATED_ID") {
		sessionID = fmt.Sprintf("site-%d-%x", os.Getpid(), randomSessionSuffix())
		if _, writeErr := io.WriteString(conn, command(sessionID)); writeErr != nil {
			_ = conn.Close()
			return "", nil, nil, fmt.Errorf("retry SESSION CREATE: %w", writeErr)
		}
		_, sessionErr = readSAMReply(r, "SESSION STATUS")
	}
	if sessionErr != nil {
		_ = conn.Close()
		return "", nil, nil, fmt.Errorf("SESSION CREATE: %w", sessionErr)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return "", nil, nil, err
	}
	forward, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		_ = conn.Close()
		return "", nil, nil, fmt.Errorf("connect I2P SAM forward socket: %w", err)
	}
	forwardReader := bufio.NewReader(forward)
	if err := forward.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		_ = conn.Close()
		_ = forward.Close()
		return "", nil, nil, err
	}
	if err := samHelloWithReader(forward, forwardReader); err != nil {
		_ = conn.Close()
		_ = forward.Close()
		return "", nil, nil, err
	}
	if _, err := fmt.Fprintf(forward, "STREAM FORWARD ID=%s PORT=%d HOST=127.0.0.1 SILENT=true\n", sessionID, port); err != nil {
		_ = conn.Close()
		_ = forward.Close()
		return "", nil, nil, fmt.Errorf("write STREAM FORWARD: %w", err)
	}
	if _, err := readSAMReply(forwardReader, "STREAM STATUS"); err != nil {
		_ = conn.Close()
		_ = forward.Close()
		return "", nil, nil, fmt.Errorf("STREAM FORWARD: %w", err)
	}
	if err := forward.SetDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		_ = forward.Close()
		return "", nil, nil, fmt.Errorf("clear I2P SAM deadline: %w", err)
	}
	return b32Address(destination), conn, forward, nil
}

func randomSessionSuffix() uint32 {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return uint32(time.Now().UnixNano())
	}
	return binary.BigEndian.Uint32(suffix[:])
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
		private := strings.TrimSpace(string(data))
		if destination, err := destinationFromPrivate(private); err == nil {
			return private, destination, nil
		}
		return "", nil, fmt.Errorf("I2P destination key file %s is unparsable", keyPath)
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
	private := reply["PRIV"]
	if private == "" {
		return "", nil, errors.New("SAM destination response omitted private destination")
	}
	destination, err := destinationFromPrivate(private)
	if err != nil {
		return "", nil, fmt.Errorf("decode SAM private destination: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(keyPath), ".site-keys-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temporary I2P destination key: %w", err)
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return "", nil, err
	}
	_, writeErr := fmt.Fprintln(tmp, private)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return "", nil, err
	}
	if err := os.Rename(name, keyPath); err != nil {
		return "", nil, fmt.Errorf("persist I2P destination key: %w", err)
	}
	return private, destination, nil
}

func decodeSAMBase64(value string) ([]byte, error) {
	value = strings.NewReplacer("-", "+", "~", "/").Replace(value)
	return decodePaddedBase64(value)
}

func decodePaddedBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

func destinationFromPrivate(private string) ([]byte, error) {
	decoded, err := decodeSAMBase64(private)
	if err != nil {
		return nil, err
	}
	if len(decoded) < 387 {
		return nil, errors.New("destination key is shorter than its certificate header")
	}
	certLen := int(binary.BigEndian.Uint16(decoded[385:387]))
	destinationLen := 387 + certLen
	if destinationLen > len(decoded) {
		return nil, errors.New("destination key certificate exceeds key length")
	}
	return append([]byte(nil), decoded[:destinationLen]...), nil
}

func samHello(conn net.Conn) error {
	return samHelloWithReader(conn, bufio.NewReader(conn))
}

func samHelloWithReader(conn net.Conn, reader *bufio.Reader) error {
	if _, err := io.WriteString(conn, "HELLO VERSION MIN=3.1 MAX=3.3\n"); err != nil {
		return fmt.Errorf("write SAM HELLO: %w", err)
	}
	reply, err := readSAMReply(reader, "HELLO REPLY")
	if err != nil {
		return err
	}
	if reply["RESULT"] != "OK" {
		return errors.New("SAM HELLO rejected; SAM 3.1 or newer is required")
	}
	return nil
}

func (s *Service) bootstrapTimeout() time.Duration {
	duration, err := time.ParseDuration(s.cfg.BootstrapTimeout)
	if err != nil || duration < 30*time.Second || duration > 10*time.Minute {
		return 5 * time.Minute
	}
	return duration
}

func readSAMReply(reader *bufio.Reader, prefix string) (map[string]string, error) {
	lineBytes, err := reader.ReadSlice('\n')
	if err != nil {
		return nil, fmt.Errorf("read SAM reply: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(lineBytes)))
	if len(fields) < 2 || fields[0]+" "+fields[1] != prefix {
		return nil, fmt.Errorf("unexpected SAM reply to %s", prefix)
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

func (s *Service) startI2PD(ctx context.Context, binary string) (string, *exec.Cmd, error) {
	configDir := filepath.Join(s.configDir, "i2p")
	dataDir := filepath.Join(s.dataDir, "i2p")
	tunnelsPath := filepath.Join(configDir, "tunnels.conf")
	tunnelsDir := filepath.Join(configDir, "tunnels.d")
	if err := os.MkdirAll(tunnelsDir, 0700); err != nil {
		return "", nil, fmt.Errorf("create empty I2P tunnels directory: %w", err)
	}
	if err := os.Chmod(tunnelsDir, 0700); err != nil {
		return "", nil, fmt.Errorf("secure I2P tunnels directory: %w", err)
	}
	if err := os.WriteFile(tunnelsPath, nil, 0600); err != nil {
		return "", nil, fmt.Errorf("write I2P tunnels configuration: %w", err)
	}
	if err := os.Chmod(tunnelsPath, 0600); err != nil {
		return "", nil, fmt.Errorf("secure I2P tunnels configuration: %w", err)
	}
	portListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("allocate private I2P SAM port: %w", err)
	}
	samAddress := portListener.Addr().String()
	_, portText, err := net.SplitHostPort(samAddress)
	_ = portListener.Close()
	if err != nil {
		return "", nil, fmt.Errorf("parse private I2P SAM port: %w", err)
	}
	conf := fmt.Sprintf("daemon = false\nipv4 = true\nipv6 = false\nnotransit = true\nfloodfill = false\nlog = file\nlogfile = %s\nloglevel = warn\n[http]\nenabled = false\n[httpproxy]\nenabled = false\n[socksproxy]\nenabled = false\n[sam]\nenabled = true\naddress = 127.0.0.1\nport = %s\n[bob]\nenabled = false\n[i2cp]\nenabled = false\n[i2pcontrol]\nenabled = false\n[upnp]\nenabled = false\n", filepath.Join(s.logDir, "i2pd.log"), portText)
	confPath := filepath.Join(configDir, "i2pd.conf")
	pidPath := filepath.Join(dataDir, "i2pd.pid")
	if err := reapStaleI2PD(binary, confPath, pidPath); err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(confPath, []byte(conf), 0600); err != nil {
		return "", nil, fmt.Errorf("write I2P router configuration: %w", err)
	}
	args := []string{"--conf", confPath, "--tunconf", tunnelsPath, "--tunnelsdir", tunnelsDir, "--datadir", dataDir, "--pidfile", pidPath}
	cmd := exec.Command(binary, args...)
	configureI2PDCommand(cmd)
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start i2pd: %w", err)
	}
	done := make(chan struct{})
	s.mu.Lock()
	s.process, s.processDone = cmd, done
	s.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	until := time.NewTimer(60 * time.Second)
	defer until.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.stopStartedProcess(cmd)
			return "", nil, ctx.Err()
		case <-until.C:
			s.stopStartedProcess(cmd)
			return "", nil, errors.New("i2pd SAM startup timed out")
		case <-done:
			return "", nil, errors.New("i2pd process exited during startup; check i2pd.log and data directory permissions")
		case <-ticker.C:
			probe, dialErr := net.DialTimeout("tcp", samAddress, time.Second)
			if dialErr == nil {
				_ = probe.SetDeadline(time.Now().Add(time.Second))
				if helloErr := samHello(probe); helloErr == nil {
					_, _ = io.WriteString(probe, "QUIT\n")
					_ = probe.Close()
					return samAddress, cmd, nil
				}
				_ = probe.Close()
			}
		}
	}
}

func (s *Service) stopStartedProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	s.mu.RLock()
	done := s.processDone
	s.mu.RUnlock()
	if err := terminateI2PDProcess(cmd); err != nil && !errors.Is(err, os.ErrProcessDone) {
		_ = cmd.Process.Kill()
	}
	if done != nil {
		stopProcess(cmd, done)
	}
	s.mu.Lock()
	if s.process == cmd {
		s.process = nil
		s.processDone = nil
	}
	s.mu.Unlock()
}

func stopProcess(cmd *exec.Cmd, done <-chan struct{}) {
	select {
	case <-done:
		return
	case <-time.After(10 * time.Second):
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	<-done
}

// Listener returns the plain loopback backend listener while the service runs.
func (s *Service) Listener() net.Listener { s.mu.RLock(); defer s.mu.RUnlock(); return s.listener }

// Enabled reports the configured opt-in state.
func (s *Service) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Enabled && s.provider != ProviderNone
}

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
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()
	switch state {
	case "running":
		if s.IsRunning() {
			return "healthy"
		}
		return "unavailable"
	case "starting", "building":
		return "starting"
	case "no_provider":
		return "disabled"
	case "error":
		return "unavailable"
	}
	return "unavailable"
}

// State reports the detailed I2P startup and provider state.
func (s *Service) State() string {
	if !s.cfg.Enabled {
		return "disabled"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state == "" {
		return "no_provider"
	}
	return s.state
}

func (s *Service) setState(state string) {
	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
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
	if s.state == "error" {
		return errors.New("I2P SAM session was lost")
	}
	if s.provider == ProviderI2PD {
		if s.processDone == nil {
			return errors.New("i2pd process state is unavailable")
		}
		select {
		case <-s.processDone:
			s.state = "error"
			return errors.New("i2pd process exited")
		default:
		}
	}
	if (s.provider != ProviderSAM && s.provider != ProviderI2PD) || s.sam == nil || s.forward == nil {
		s.state = "error"
		return errors.New("I2P provider is unavailable")
	}
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	address := s.cfg.SAMAddress
	if s.provider == ProviderI2PD {
		address = s.sam.RemoteAddr().String()
	}
	probe, err := (&net.Dialer{Timeout: time.Until(deadline)}).DialContext(ctx, "tcp", address)
	if err != nil {
		s.state = "error"
		return fmt.Errorf("connect I2P SAM health probe: %w", err)
	}
	defer func() { _ = probe.Close() }()
	if err := probe.SetDeadline(deadline); err != nil {
		return err
	}
	if err := samHello(probe); err != nil {
		s.state = "error"
		return fmt.Errorf("I2P SAM health probe failed: %w", err)
	}
	_, _ = io.WriteString(probe, "QUIT\n")
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
	forward, conn, process, processDone, cancel := s.forward, s.sam, s.process, s.processDone, s.runCancel
	s.forward, s.sam, s.process, s.processDone = nil, nil, nil, nil
	s.runCancel = nil
	s.address = ""
	s.provider = ProviderNone
	if s.cfg.Enabled {
		s.state = "no_provider"
	} else {
		s.state = "disabled"
	}
	s.mu.Unlock()
	var closeErr error
	if forward != nil {
		closeErr = errors.Join(closeErr, forward.Close())
	}
	if conn != nil {
		closeErr = errors.Join(closeErr, conn.Close())
	}
	if cancel != nil {
		cancel()
	}
	if process != nil && process.Process != nil {
		if err := terminateI2PDProcess(process); err != nil && !errors.Is(err, os.ErrProcessDone) {
			closeErr = errors.Join(closeErr, err)
		}
		if processDone != nil {
			select {
			case <-processDone:
			case <-time.After(10 * time.Second):
				if err := process.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					closeErr = errors.Join(closeErr, err)
				}
				<-processDone
			}
		}
	}
	return closeErr
}
