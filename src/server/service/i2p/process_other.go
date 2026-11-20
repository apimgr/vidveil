//go:build !linux && !windows

// SPDX-License-Identifier: MIT
package i2p

import "os/exec"

func configureI2PDCommand(*exec.Cmd) {}
