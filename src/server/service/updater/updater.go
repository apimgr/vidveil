// SPDX-License-Identifier: MIT
// AI.md PART 22: Update Command
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	githubOrg  = "apimgr"
	githubRepo = "vidveil"
)

// Release represents a GitHub release
type Release struct {
	TagName    string  `json:"tag_name"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset represents a release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckForUpdate checks GitHub releases for a newer version per AI.md PART 22.
// branch must be "stable", "beta", or "daily".
// Returns nil release and nil error when already up to date.
// HTTP 404 from GitHub API means no updates available.
func CheckForUpdate(ctx context.Context, currentVersion, branch string) (*Release, error) {
	var apiURL string
	switch branch {
	case "beta", "daily":
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", githubOrg, githubRepo)
	default:
		// stable (default)
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubOrg, githubRepo)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	// 404 = no releases exist yet or already current
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	if branch == "stable" {
		var release Release
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return nil, fmt.Errorf("decode release: %w", err)
		}
		if release.TagName == currentVersion {
			return nil, nil
		}
		return &release, nil
	}

	// beta / daily: fetch all releases and pick the most recent match
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	for i := range releases {
		r := &releases[i]
		if matchesBranch(r, branch) && r.TagName != currentVersion {
			return r, nil
		}
	}
	return nil, nil
}

// DoUpdate downloads and installs the update per AI.md PART 22.
// It downloads the platform binary, verifies the SHA-256 checksum (if
// a .sha256 asset is present), replaces the running binary atomically,
// then restarts the service.
func DoUpdate(ctx context.Context, release *Release) error {
	assetName := getBinaryName()
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s",
			runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	// Download to a temp file
	tmpFile, err := os.CreateTemp("", "vidveil-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write download: %w", err)
	}
	tmpFile.Close()

	// Try to verify checksum from a companion .sha256 asset
	checksumAsset := assetName + ".sha256"
	for _, a := range release.Assets {
		if a.Name == checksumAsset {
			if err := verifyChecksum(ctx, tmpPath, a.BrowserDownloadURL); err != nil {
				return fmt.Errorf("checksum verification: %w", err)
			}
			break
		}
	}

	// Make executable on non-Windows
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}

	// Resolve current binary path (follow symlinks)
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	// Platform-specific binary replacement
	if err := replaceBinary(currentPath, tmpPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	// Restart service or re-exec
	return restartSelf()
}

// SetBranch stores the update branch in the config file.
// Valid values: stable, beta, daily.
func SetBranch(configPath, branch string) error {
	branch = strings.ToLower(strings.TrimSpace(branch))
	switch branch {
	case "stable", "beta", "daily":
	default:
		return fmt.Errorf("invalid branch %q: must be stable, beta, or daily", branch)
	}
	// Write a simple marker file; the server reads this on startup
	markerPath := configPath + ".update_branch"
	return os.WriteFile(markerPath, []byte(branch), 0640)
}

// getBinaryName returns the release asset name for the current platform
// per AI.md PART 22 binary naming rules.
func getBinaryName() string {
	name := "vidveil-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// matchesBranch returns true if the release matches the requested branch.
func matchesBranch(r *Release, branch string) bool {
	switch branch {
	case "beta":
		return strings.HasSuffix(r.TagName, "-beta")
	case "daily":
		// Daily builds use YYYYMMDDHHMMSS timestamp tags (14 digits, no dots)
		return len(r.TagName) == 14 && !strings.Contains(r.TagName, ".")
	default:
		return !r.Prerelease
	}
}

// verifyChecksum downloads the .sha256 file and compares it against the
// downloaded binary per AI.md PART 22.
func verifyChecksum(ctx context.Context, filePath, checksumURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Format is "<hex> <filename>" or just "<hex>"
	expectedHash := strings.TrimSpace(strings.Fields(string(body))[0])

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))

	if actual != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s got %s", expectedHash, actual)
	}
	return nil
}

// restartService restarts the system service after an update.
// Called after replaceBinary succeeds per AI.md PART 22.
func restartService() error {
	switch runtime.GOOS {
	case "linux":
		return restartLinuxService()
	case "darwin":
		return restartDarwinService()
	case "freebsd", "openbsd", "netbsd":
		return restartBSDService()
	case "windows":
		return restartWindowsService()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
