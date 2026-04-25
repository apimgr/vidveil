// SPDX-License-Identifier: MIT
package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigFilePath(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	yamlConfigPath := filepath.Join(ConfigDir(), "existing.yaml")

	if err := os.MkdirAll(filepath.Dir(yamlConfigPath), 0700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(yamlConfigPath, []byte("server:\n  address: https://x.scour.li\n"), 0600); err != nil {
		t.Fatalf("writing existing config: %v", err)
	}

	testCases := []struct {
		name       string
		configFlag string
		wantPath   string
	}{
		{
			name:       "default config file",
			configFlag: "",
			wantPath:   filepath.Join(ConfigDir(), "cli.yml"),
		},
		{
			name:       "relative bare name defaults to yml",
			configFlag: "test",
			wantPath:   filepath.Join(ConfigDir(), "test.yml"),
		},
		{
			name:       "relative existing yaml is detected",
			configFlag: "existing",
			wantPath:   filepath.Join(ConfigDir(), "existing.yaml"),
		},
		{
			name:       "relative explicit yaml extension is preserved",
			configFlag: "dev.yml",
			wantPath:   filepath.Join(ConfigDir(), "dev.yml"),
		},
		{
			name:       "absolute path is preserved",
			configFlag: "/tmp/custom-config.yaml",
			wantPath:   "/tmp/custom-config.yaml",
		},
		{
			name:       "tilde path expands to home directory",
			configFlag: "~/custom/cli",
			wantPath:   filepath.Join(homeDir, "custom", "cli.yml"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotPath := ResolveConfigFilePath(testCase.configFlag)
			if gotPath != testCase.wantPath {
				t.Fatalf("resolved path = %q, want %q", gotPath, testCase.wantPath)
			}
		})
	}
}

func TestTokenFileUsesConfigDirectory(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	wantTokenPath := filepath.Join(ConfigDir(), "token")
	if gotTokenPath := TokenFile(); gotTokenPath != wantTokenPath {
		t.Fatalf("token path = %q, want %q", gotTokenPath, wantTokenPath)
	}
}

func TestEnsureClientDirsUsesCurrentUserOwnership(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(homeDir, "AppData", "Local"))

	if err := EnsureClientDirs(); err != nil {
		t.Fatalf("ensuring client dirs: %v", err)
	}

	dirs := []string{
		ConfigDir(),
		DataDir(),
		CacheDir(),
		LogDir(),
	}
	for _, dirPath := range dirs {
		if err := EnsurePathOwnership(dirPath); err != nil {
			t.Fatalf("verifying ownership for %q: %v", dirPath, err)
		}
	}
}
