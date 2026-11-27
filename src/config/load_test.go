// SPDX-License-Identifier: MIT
package config

import (
	"reflect"
	"testing"
)

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
