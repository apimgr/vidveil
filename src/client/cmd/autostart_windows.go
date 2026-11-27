// SPDX-License-Identifier: MIT
//go:build windows

package cmd

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const cliAutostartRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func platformCLIAutostartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, cliAutostartRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue("vidveil-cli")
	return err == nil
}

func setPlatformCLIAutostartEnabled(enabled bool) error {
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, cliAutostartRegistryKey, registry.SET_VALUE)
		if err != nil {
			if err == registry.ErrNotExist {
				return nil
			}
			return err
		}
		defer key.Close()
		if err := key.DeleteValue("vidveil-cli"); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve CLI executable: %w", err)
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, cliAutostartRegistryKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("vidveil-cli", `"`+strings.ReplaceAll(executable, `"`, `\"`)+`"`)
}
