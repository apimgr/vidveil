// SPDX-License-Identifier: MIT
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseRuntimeDoesNotCreateConfigurationPaths(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	dataDir := filepath.Join(root, "data")
	for _, key := range []string{"CONFIG_DIR", "DATA_DIR", "CACHE_DIR", "LOG_DIR", "BACKUP_DIR", "BASEURL", "LANG"} {
		t.Setenv(key, "")
	}
	if _, err := ParseRuntime([]string{"--config", configDir, "--data", dataDir}); err != nil {
		t.Fatalf("ParseRuntime: %v", err)
	}
	for _, path := range []string{configDir, dataDir} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("ParseRuntime created %q or returned an unexpected stat error: %v", path, err)
		}
	}
}

func TestParseRuntimeValidatesFilesystemDirectoryOverrides(t *testing.T) {
	for _, key := range []string{"CONFIG_DIR", "DATA_DIR", "CACHE_DIR", "LOG_DIR", "BACKUP_DIR"} {
		t.Setenv(key, "")
	}
	valid := filepath.Join(t.TempDir(), "absolute", "directory")
	parsed, err := ParseRuntime([]string{"--data", valid})
	if err != nil {
		t.Fatalf("ParseRuntime rejected absolute directory: %v", err)
	}
	if parsed.Runtime.DataDir != valid {
		t.Errorf("DataDir = %q, want %q", parsed.Runtime.DataDir, valid)
	}
	if _, err := ParseRuntime([]string{"--data", filepath.Join("..", "outside")}); err == nil {
		t.Fatal("ParseRuntime accepted a directory override with a parent segment")
	}
}

func TestSplitRuntimeCommandSkipsFlagValues(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		global  []string
		command []string
	}{
		{
			name:   "command-like config value",
			args:   []string{"--config", "tor"},
			global: []string{"--config", "tor"},
		},
		{
			name:    "command-like address value before command",
			args:    []string{"--address", "--status", "--status"},
			global:  []string{"--address", "--status"},
			command: []string{"--status"},
		},
		{
			name:    "inline flag value before command",
			args:    []string{"--config=tor", "--status"},
			global:  []string{"--config=tor"},
			command: []string{"--status"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			global, command := splitRuntimeCommand(test.args)
			if !reflect.DeepEqual(global, test.global) {
				t.Errorf("global args = %#v, want %#v", global, test.global)
			}
			if !reflect.DeepEqual(command, test.command) {
				t.Errorf("command args = %#v, want %#v", command, test.command)
			}
		})
	}
}
