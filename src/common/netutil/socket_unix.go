// SPDX-License-Identifier: MIT
//go:build !windows

package netutil

import (
	"syscall"
)

func setIPv6Only(enabled bool) func(string, string, syscall.RawConn) error {
	return func(_, _ string, raw syscall.RawConn) error {
		var socketErr error
		controlErr := raw.Control(func(fd uintptr) {
			value := 0
			if enabled {
				value = 1
			}
			socketErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, value)
		})
		if controlErr != nil {
			return controlErr
		}
		return socketErr
	}
}

func errAddressFamilyUnavailable() error { return syscall.EAFNOSUPPORT }
