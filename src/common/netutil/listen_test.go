// SPDX-License-Identifier: MIT
package netutil

import (
	"net"
	"strconv"
	"testing"
)

func TestListenTCPIPv4WildcardStaysIPv4(t *testing.T) {
	listener, err := ListenTCP("0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	if got := listener.Addr().(*net.TCPAddr).IP; got.To4() == nil {
		t.Fatalf("listener address = %v, want an IPv4 address", got)
	}
}

func TestListenTCPDefaultAddressIsDualStack(t *testing.T) {
	listener, err := ListenTCP("[::]:0")
	if err != nil {
		t.Skipf("dual-stack IPv6 listener unavailable: %v", err)
	}
	defer func() { _ = listener.Close() }()
	port := listener.Addr().(*net.TCPAddr).Port

	for _, address := range []string{net.JoinHostPort("::1", strconv.Itoa(port)), net.JoinHostPort("127.0.0.1", strconv.Itoa(port))} {
		conn, dialErr := net.Dial("tcp", address)
		if dialErr != nil {
			t.Fatalf("dial dual-stack listener at %s: %v", address, dialErr)
		}
		_ = conn.Close()
	}
}
