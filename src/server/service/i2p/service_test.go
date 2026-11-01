// SPDX-License-Identifier: MIT
package i2p

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestSAMHealthCheckRequiresPong(t *testing.T) {
	local, remote := net.Pipe()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close SAM test listener: %v", err)
		}
	})
	svc := &Service{cfg: config.I2PConfig{Enabled: true}, listener: listener, provider: ProviderSAM, address: "stable.b32.i2p", sam: local}
	go func() {
		defer func() { _ = remote.Close() }()
		probe, err := bufio.NewReader(remote).ReadString('\n')
		if err == nil && probe == "PING\n" {
			_, _ = fmt.Fprint(remote, "PONG\n")
		}
	}()
	if err := svc.CheckHealth(context.Background()); err != nil {
		t.Fatalf("CheckHealth with a live SAM session: %v", err)
	}
	_ = svc.Close()
}
