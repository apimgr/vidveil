// SPDX-License-Identifier: MIT
package i2p

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

func TestDisabledServiceDoesNotCreateRuntimeState(t *testing.T) {
	root := t.TempDir()
	svc := NewService(config.DefaultI2PConfig(), filepath.Join(root, "config"), filepath.Join(root, "data"), filepath.Join(root, "logs"), nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start on disabled service: %v", err)
	}
	if svc.Listener() != nil || svc.IsRunning() || svc.Address() != "" || svc.Provider() != string(ProviderNone) {
		t.Fatal("disabled I2P service unexpectedly acquired runtime state")
	}
	if _, err := os.Stat(filepath.Join(root, "config", "i2p")); !os.IsNotExist(err) {
		t.Fatalf("disabled service created configuration directory: %v", err)
	}
}

func TestNewBackendListenerIsLoopbackOnly(t *testing.T) {
	listener, port, err := newBackendListener()
	if err != nil {
		t.Fatalf("newBackendListener: %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close backend listener: %v", err)
		}
	})
	if port < 64000 || port > 64999 {
		t.Fatalf("backend port %d outside required range", port)
	}
	host, _, err := net.SplitHostPort(listener.Addr().String())
	if err != nil || host != "127.0.0.1" {
		t.Fatalf("backend listener address %q is not IPv4 loopback", listener.Addr())
	}
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("connect to accepting I2P listener: %v", err)
	}
	defer client.Close()
	accepted, err := listener.Accept()
	if err != nil {
		t.Fatalf("accept queued backend connection: %v", err)
	}
	_ = accepted.Close()
}

func TestB32AddressFormat(t *testing.T) {
	address := b32Address([]byte("stable destination"))
	if !strings.HasSuffix(address, ".b32.i2p") || len(strings.TrimSuffix(address, ".b32.i2p")) != 52 {
		t.Fatalf("unexpected I2P hostname %q", address)
	}
}

func TestReadSAMReplyParsesStatusAndRejectsErrors(t *testing.T) {
	values, err := readSAMReply(bufio.NewReader(strings.NewReader("SESSION STATUS RESULT=OK DESTINATION=abc\n")), "SESSION STATUS")
	if err != nil || values["RESULT"] != "OK" || values["DESTINATION"] != "abc" {
		t.Fatalf("readSAMReply success = (%v, %v)", values, err)
	}
	if _, err := readSAMReply(bufio.NewReader(strings.NewReader("SESSION STATUS RESULT=INVALID_ID\n")), "SESSION STATUS"); err == nil {
		t.Fatal("readSAMReply accepted a failed SAM result")
	}
	if _, err := readSAMReply(bufio.NewReader(strings.NewReader("HELLO REPLY RESULT=OK\n")), "SESSION STATUS"); err == nil {
		t.Fatal("readSAMReply accepted an unexpected reply command")
	}
}

func TestSAMHealthCheckUsesIndependentHelloProbe(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close SAM test listener: %v", err)
		}
	})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		if _, readErr := reader.ReadString('\n'); readErr == nil {
			_, _ = conn.Write([]byte("HELLO REPLY RESULT=OK VERSION=3.3\n"))
			_, _ = reader.ReadString('\n')
		}
	}()
	control, controlPeer := net.Pipe()
	forward, forwardPeer := net.Pipe()
	defer controlPeer.Close()
	defer forwardPeer.Close()
	svc := &Service{cfg: config.I2PConfig{Enabled: true, SAMAddress: listener.Addr().String()}, listener: listener, provider: ProviderSAM, address: "stable.b32.i2p", sam: control, forward: forward}
	if err := svc.CheckHealth(context.Background()); err != nil {
		t.Fatalf("CheckHealth with a live SAM session: %v", err)
	}
	_ = svc.Close()
}

func TestDestinationFromPrivateUsesCertificateLength(t *testing.T) {
	private := make([]byte, 600)
	binary.BigEndian.PutUint16(private[385:387], 4)
	encoded := base64.StdEncoding.EncodeToString(private)
	destination, err := destinationFromPrivate(encoded)
	if err != nil {
		t.Fatalf("destinationFromPrivate: %v", err)
	}
	if len(destination) != 391 {
		t.Fatalf("destination length = %d, want 391", len(destination))
	}
}

func TestMonitorSAMConnectionMarksLostSession(t *testing.T) {
	for _, closeForward := range []bool{false, true} {
		localControl, peerControl := net.Pipe()
		localForward, peerForward := net.Pipe()
		svc := &Service{cfg: config.I2PConfig{Enabled: true}, provider: ProviderSAM, state: "running", sam: localControl, forward: localForward}
		connToClose := peerControl
		if closeForward {
			connToClose = peerForward
		}
		monitored := localControl
		if closeForward {
			monitored = localForward
		}
		done := make(chan struct{})
		go func() {
			svc.monitorSAMConnection(monitored)
			close(done)
		}()
		_ = connToClose.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("SAM connection monitor did not detect closure")
		}
		if got := svc.State(); got != "error" {
			t.Errorf("closeForward=%t: state = %q, want error", closeForward, got)
		}
		_ = localControl.Close()
		_ = localForward.Close()
		_ = peerControl.Close()
		_ = peerForward.Close()
	}
}

func TestReapStaleI2PDRefusesUnrelatedProcess(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "i2pd.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	confPath := filepath.Join(t.TempDir(), "i2pd.conf")
	if err := reapStaleI2PD("not-the-current-process", confPath, pidPath); err == nil {
		t.Fatal("reapStaleI2PD did not protect an unrelated process")
	}
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("unrelated process pid file was removed: %v", err)
	}
}

func TestDecodeSAMBase64SupportsI2PAlphabet(t *testing.T) {
	decoded := make([]byte, 256)
	for i := range decoded {
		decoded[i] = byte(i)
	}
	encoded := strings.NewReplacer("+", "-", "/", "~").Replace(base64.StdEncoding.EncodeToString(decoded))
	actual, err := decodeSAMBase64(encoded)
	if err != nil {
		t.Fatalf("decodeSAMBase64: %v", err)
	}
	if string(actual) != string(decoded) {
		t.Fatal("I2P base64 round trip did not preserve bytes")
	}
}

func TestStartSAMUsesSeparateForwardSocketAndPersistsPrivateKey(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"config/i2p", "data/i2p/site"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	private := make([]byte, 600)
	binary.BigEndian.PutUint16(private[385:387], 4)
	privateKey := strings.NewReplacer("+", "-", "/", "~").Replace(base64.StdEncoding.EncodeToString(private))
	commands := make(chan string, 4)
	serverErr := make(chan error, 1)
	go func() {
		control, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer control.Close()
		reader := bufio.NewReader(control)
		if err := samTestHello(reader, control); err != nil {
			serverErr <- err
			return
		}
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			serverErr <- readErr
			return
		}
		commands <- strings.TrimSpace(line)
		_, _ = fmt.Fprintf(control, "DEST REPLY PRIV=%s\n", privateKey)
		line, readErr = reader.ReadString('\n')
		if readErr != nil {
			serverErr <- readErr
			return
		}
		commands <- strings.TrimSpace(line)
		_, _ = control.Write([]byte("SESSION STATUS RESULT=DUPLICATED_ID\n"))
		line, readErr = reader.ReadString('\n')
		if readErr != nil {
			serverErr <- readErr
			return
		}
		commands <- strings.TrimSpace(line)
		storedKey, keyErr := os.ReadFile(filepath.Join(root, "data/i2p/site/site-keys.dat"))
		if keyErr != nil || strings.TrimSpace(string(storedKey)) != privateKey {
			serverErr <- errors.New("private key was not persisted before SESSION CREATE")
			return
		}
		_, _ = control.Write([]byte("SESSION STATUS RESULT=OK\n"))

		forward, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer forward.Close()
		forwardReader := bufio.NewReader(forward)
		if err := samTestHello(forwardReader, forward); err != nil {
			serverErr <- err
			return
		}
		line, readErr = forwardReader.ReadString('\n')
		if readErr != nil {
			serverErr <- readErr
			return
		}
		commands <- strings.TrimSpace(line)
		_, _ = forward.Write([]byte("STREAM STATUS RESULT=OK\n"))
		serverErr <- nil
	}()

	cfg := config.DefaultI2PConfig()
	cfg.Enabled = true
	cfg.SAMAddress = listener.Addr().String()
	svc := NewService(cfg, filepath.Join(root, "config"), filepath.Join(root, "data"), filepath.Join(root, "logs"), nil)
	address, control, forward, err := svc.startSAM(context.Background(), cfg.SAMAddress, 64888)
	if err != nil {
		t.Fatalf("startSAM: %v", err)
	}
	defer control.Close()
	defer forward.Close()
	if err := <-serverErr; err != nil {
		t.Fatalf("fake SAM server: %v", err)
	}
	if !strings.HasSuffix(address, ".b32.i2p") {
		t.Fatalf("derived hostname %q has wrong suffix", address)
	}
	if command := <-commands; !strings.HasPrefix(command, "DEST GENERATE SIGNATURE_TYPE=7") {
		t.Fatalf("first SAM command = %q", command)
	}
	firstSessionCommand := <-commands
	if !strings.HasPrefix(firstSessionCommand, "SESSION CREATE STYLE=STREAM") {
		t.Fatalf("second SAM command = %q", firstSessionCommand)
	}
	retrySessionCommand := <-commands
	if !strings.HasPrefix(retrySessionCommand, "SESSION CREATE STYLE=STREAM") || firstSessionCommand == retrySessionCommand {
		t.Fatalf("retried SAM session command = %q, first=%q", retrySessionCommand, firstSessionCommand)
	}
	if command := <-commands; !strings.Contains(command, "SILENT=true") || !strings.Contains(command, "HOST=127.0.0.1") {
		t.Fatalf("forward command = %q", command)
	}
	keyPath := filepath.Join(root, "data/i2p/site/site-keys.dat")
	info, err := os.Stat(keyPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("key permissions = %v, %v; want 0600", info, err)
	}
}

func samTestHello(reader *bufio.Reader, conn net.Conn) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.Contains(line, "MIN=3.1 MAX=3.3") {
		return fmt.Errorf("unexpected HELLO command %q", strings.TrimSpace(line))
	}
	_, err = conn.Write([]byte("HELLO REPLY RESULT=OK VERSION=3.3\n"))
	return err
}
