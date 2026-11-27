// SPDX-License-Identifier: MIT
//go:build linux || darwin

package cmd

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func cliAutostartPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "linux" {
		return filepath.Join(home, ".config", "autostart", "vidveil-cli.desktop"), nil
	}
	return filepath.Join(home, "Library", "LaunchAgents", "apimgr.vidveil-cli.plist"), nil
}

func platformCLIAutostartEnabled() bool {
	path, err := cliAutostartPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func setPlatformCLIAutostartEnabled(enabled bool) error {
	path, err := cliAutostartPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve CLI executable: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	var contents string
	if strings.HasSuffix(path, ".desktop") {
		contents = "[Desktop Entry]\nType=Application\nName=VidVeil CLI\nExec=" + desktopExecArgument(executable) + "\nX-GNOME-Autostart-enabled=true\n"
	} else {
		var escaped strings.Builder
		if err := xml.EscapeText(&escaped, []byte(executable)); err != nil {
			return err
		}
		contents = fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict><key>Label</key><string>apimgr.vidveil-cli</string><key>ProgramArguments</key><array><string>%s</string></array><key>RunAtLoad</key><true/></dict></plist>\n", escaped.String())
	}
	return os.WriteFile(path, []byte(contents), 0600)
}

func desktopExecArgument(value string) string {
	value = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$").Replace(value)
	return `"` + value + `"`
}
