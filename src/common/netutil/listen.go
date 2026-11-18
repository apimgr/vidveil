// SPDX-License-Identifier: MIT
package netutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

var warnIPv4Fallback sync.Once

// ListenTCP opens a TCP listener while explicitly configuring IPv6 dual-stack
// behavior for the unspecified IPv6 address.
func ListenTCP(address string) (net.Listener, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if host == "" || host == "0.0.0.0" {
		return net.Listen("tcp4", net.JoinHostPort(host, port))
	}
	if !strings.Contains(host, ":") {
		return net.Listen("tcp", address)
	}
	if host != "::" {
		cfg := net.ListenConfig{Control: setIPv6Only(true)}
		return cfg.Listen(context.Background(), "tcp6", address)
	}

	cfg := net.ListenConfig{Control: setIPv6Only(false)}
	listener, err := cfg.Listen(context.Background(), "tcp6", net.JoinHostPort("::", port))
	if err == nil {
		return listener, nil
	}
	if !isIPv6Unavailable(err) {
		return nil, err
	}
	warnIPv4Fallback.Do(func() {
		fmt.Fprintln(os.Stderr, "Warning: IPv6 is unavailable; listening on IPv4 0.0.0.0")
	})
	return net.Listen("tcp4", net.JoinHostPort("0.0.0.0", port))
}

func isIPv6Unavailable(err error) bool {
	message := strings.ToLower(err.Error())
	return errors.Is(err, errAddressFamilyUnavailable()) ||
		strings.Contains(message, "address family not supported") ||
		strings.Contains(message, "protocol not supported")
}
