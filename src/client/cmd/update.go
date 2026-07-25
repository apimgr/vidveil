// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI Auto-Update
package cmd

import (
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
	"syscall"
	"time"

	"github.com/apimgr/vidveil/src/client/path"
)

// CLIUpdateBranchFile is the on-disk record of the user-selected update channel.
const CLIUpdateBranchFile = "update-branch"

// CLIUpdateValidBranches enumerates branches per AI.md PART 22.
var CLIUpdateValidBranches = map[string]bool{
	"stable": true,
	"beta":   true,
	"daily":  true,
}

// CLIGitHubRelease mirrors the fields from the GitHub releases API used for self-update.
type CLIGitHubRelease struct {
	TagName     string           `json:"tag_name"`
	HTMLURL     string           `json:"html_url"`
	Body        string           `json:"body"`
	Prerelease  bool             `json:"prerelease"`
	PublishedAt time.Time        `json:"published_at"`
	Assets      []CLIGitHubAsset `json:"assets"`
}

// CLIGitHubAsset is one downloadable artifact attached to a release.
type CLIGitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CLIUpdateInfo summarizes a self-update check result.
type CLIUpdateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ReleaseURL      string
	DownloadURL     string
	ChecksumURL     string
	AssetName       string
}

// RunCLIUpdateCommand handles --update [check|yes|branch <name>|--help] per AI.md PART 32.
func RunCLIUpdateCommand(args []string) error {
	cmd := "yes"
	var arg string
	if len(args) > 0 {
		cmd = args[0]
		if len(args) > 1 {
			arg = args[1]
		}
	}

	switch cmd {
	case "--help", "help", "-h":
		PrintCLIUpdateHelp()
		return nil
	case "check":
		return runCLIUpdateCheck()
	case "yes", "":
		return runCLIUpdateApply()
	case "branch":
		return runCLIUpdateBranch(arg)
	default:
		fmt.Fprintf(os.Stderr, "unknown update command: %s\n", cmd)
		PrintCLIUpdateHelp()
		return fmt.Errorf("unknown update command: %s", cmd)
	}
}

// PrintCLIUpdateHelp prints --update help and exits successfully per AI.md PART 8.
func PrintCLIUpdateHelp() {
	fmt.Printf(`Update Commands:
  %s --update              Check and perform in-place update with re-exec
  %s --update yes          Same as --update (default)
  %s --update check        Check for updates without installing
  %s --update branch NAME  Set update branch (stable, beta, daily)

Update Branches:
  stable (default)  Release builds (v*, *.*.*)
  beta              Pre-release builds (*-beta)
  daily             Daily builds (YYYYMMDDHHMMSS)
`, BinaryName, BinaryName, BinaryName, BinaryName)
}

func runCLIUpdateCheck() error {
	fmt.Println("Checking for updates...")
	fmt.Printf("Current version: %s\n", Version)

	info, err := CheckCLIUpdate()
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			fmt.Println("Already up to date (no newer release found)")
			return nil
		}
		fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		return err
	}

	fmt.Printf("Latest version:  %s\n", info.LatestVersion)
	if info.UpdateAvailable {
		fmt.Println()
		fmt.Println("Update available.")
		fmt.Printf("   Release: %s\n", info.ReleaseURL)
		fmt.Printf("\n   Run '%s --update yes' to download and install\n", BinaryName)
	} else {
		fmt.Println("Already up to date")
	}
	return nil
}

func runCLIUpdateApply() error {
	fmt.Println("Checking for updates...")
	fmt.Printf("Current version: %s\n", Version)

	info, err := CheckCLIUpdate()
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			fmt.Println("Already up to date")
			return nil
		}
		fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		return err
	}

	fmt.Printf("Latest version:  %s\n", info.LatestVersion)
	if !info.UpdateAvailable {
		fmt.Println("Already up to date")
		return nil
	}
	if info.DownloadURL == "" {
		return fmt.Errorf("no download asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Println()
	fmt.Println("Downloading update...")
	tmpPath, err := downloadCLIBinary(info.DownloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tmpPath)

	if info.ChecksumURL == "" {
		return fmt.Errorf("release has no checksums.txt asset; refusing to install unverified binary")
	}
	fmt.Println("Verifying SHA-256 checksum...")
	if err := verifyCLIChecksum(tmpPath, info.ChecksumURL, info.AssetName); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	if err := replaceCLIBinary(execPath, tmpPath); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "permission") {
			fmt.Fprintf(os.Stderr, "you do not have permission to update %s; ask your admin or move the binary to a writable path\n", execPath)
			return err
		}
		return fmt.Errorf("installing update: %w", err)
	}

	fmt.Println("Update installed; re-executing...")
	if execErr := syscall.Exec(execPath, os.Args, os.Environ()); execErr != nil {
		fmt.Fprintf(os.Stderr, "re-exec failed; please rerun the command manually: %v\n", execErr)
		return execErr
	}
	return nil
}

func runCLIUpdateBranch(branch string) error {
	branch = strings.TrimSpace(strings.ToLower(branch))
	if !CLIUpdateValidBranches[branch] {
		return fmt.Errorf("invalid branch: %q (valid: stable, beta, daily)", branch)
	}
	if err := SetCLIUpdateBranch(branch); err != nil {
		return err
	}
	fmt.Printf("Update branch set to: %s\n", branch)
	return nil
}

// SetCLIUpdateBranch persists the selected branch to the CLI config dir.
func SetCLIUpdateBranch(branch string) error {
	configDir := path.ConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	branchFile := filepath.Join(configDir, CLIUpdateBranchFile)
	if err := os.WriteFile(branchFile, []byte(branch+"\n"), 0600); err != nil {
		return fmt.Errorf("writing branch file: %w", err)
	}
	return nil
}

// GetCLIUpdateBranch returns the persisted branch (defaulting to stable).
func GetCLIUpdateBranch() string {
	branchFile := filepath.Join(path.ConfigDir(), CLIUpdateBranchFile)
	data, err := os.ReadFile(branchFile)
	if err != nil {
		return "stable"
	}
	branch := strings.TrimSpace(string(data))
	if !CLIUpdateValidBranches[branch] {
		return "stable"
	}
	return branch
}

// CheckCLIUpdate fetches release metadata for the active branch and resolves the artifact URL.
func CheckCLIUpdate() (*CLIUpdateInfo, error) {
	branch := GetCLIUpdateBranch()

	var release *CLIGitHubRelease
	var err error
	switch branch {
	case "beta":
		release, err = fetchLatestCLIBetaRelease()
	case "daily":
		release, err = fetchLatestCLIDailyRelease()
	default:
		release, err = fetchLatestCLIStableRelease()
	}
	if err != nil {
		return nil, err
	}

	currentVersion := strings.TrimPrefix(Version, "v")
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	info := &CLIUpdateInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: latestVersion != currentVersion && compareCLIVersions(latestVersion, currentVersion) > 0,
		ReleaseURL:      release.HTMLURL,
	}

	// Per AI.md PART 22 checksums come from the release's checksums.txt asset
	// (lines "{sha256}  {filename}"), NOT a per-file .sha256 sidecar.
	binaryAssetName := cliReleaseBinaryName()
	info.AssetName = binaryAssetName
	for _, asset := range release.Assets {
		switch asset.Name {
		case binaryAssetName:
			info.DownloadURL = asset.BrowserDownloadURL
		case "checksums.txt":
			info.ChecksumURL = asset.BrowserDownloadURL
		}
	}
	return info, nil
}

func cliReleaseBinaryName() string {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return fmt.Sprintf("%s-%s-%s%s", BinaryName, runtime.GOOS, runtime.GOARCH, suffix)
}

func fetchLatestCLIStableRelease() (*CLIGitHubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", path.GitHubOrg(), path.GitHubRepo())
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	var release CLIGitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release: %w", err)
	}
	return &release, nil
}

func fetchAllCLIReleases() ([]CLIGitHubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=50", path.GitHubOrg(), path.GitHubRepo())
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	var releases []CLIGitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("parsing releases: %w", err)
	}
	return releases, nil
}

// matchesCLIBranch implements cumulative update channels per AI.md PART 22:
// stable matches every channel; beta = {stable, beta}; daily = {stable, beta,
// daily}. Daily builds are 14-digit timestamps (YYYYMMDDHHMMSS) with no dots.
func matchesCLIBranch(r CLIGitHubRelease, branch string) bool {
	if !r.Prerelease {
		return true
	}
	isBeta := strings.HasSuffix(r.TagName, "-beta")
	isDaily := len(r.TagName) == 14 && !strings.Contains(r.TagName, ".")
	switch branch {
	case "beta":
		return isBeta
	case "daily":
		return isBeta || isDaily
	default:
		return false
	}
}

// fetchLatestCLIBetaRelease returns the newest of {stable, beta}. Releases are
// returned newest-first, so the first match wins.
func fetchLatestCLIBetaRelease() (*CLIGitHubRelease, error) {
	releases, err := fetchAllCLIReleases()
	if err != nil {
		return nil, err
	}
	for i := range releases {
		if matchesCLIBranch(releases[i], "beta") {
			return &releases[i], nil
		}
	}
	return nil, fmt.Errorf("no beta releases found")
}

// fetchLatestCLIDailyRelease returns the newest of {stable, beta, daily}.
// Releases are returned newest-first, so the first match wins.
func fetchLatestCLIDailyRelease() (*CLIGitHubRelease, error) {
	releases, err := fetchAllCLIReleases()
	if err != nil {
		return nil, err
	}
	for i := range releases {
		if matchesCLIBranch(releases[i], "daily") {
			return &releases[i], nil
		}
	}
	return nil, fmt.Errorf("no daily releases found")
}

func downloadCLIBinary(downloadURL string) (string, error) {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("downloading binary: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download status %d", resp.StatusCode)
	}

	tmpDir := filepath.Join(os.TempDir(), path.GitHubOrg())
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(tmpDir, BinaryName+".update.*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("chmod temp file: %w", err)
	}
	return tmpPath, nil
}

func verifyCLIChecksum(binaryPath, checksumURL, assetName string) error {
	resp, err := http.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("downloading checksum: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksum download status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading checksum: %w", err)
	}
	// checksums.txt has one "{sha256}  {filename}" line per asset
	var expected string
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("no checksum entry for %s in checksums.txt", assetName)
	}

	binaryFile, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("opening downloaded binary: %w", err)
	}
	defer binaryFile.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, binaryFile); err != nil {
		return fmt.Errorf("hashing downloaded binary: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("sha-256 mismatch (expected %s, got %s)", expected, actual)
	}
	return nil
}

func replaceCLIBinary(execPath, newBinaryPath string) error {
	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("stating current binary: %w", err)
	}
	if runtime.GOOS == "windows" {
		oldPath := execPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			return fmt.Errorf("renaming current binary: %w", err)
		}
		if err := os.Rename(newBinaryPath, execPath); err != nil {
			_ = os.Rename(oldPath, execPath)
			return fmt.Errorf("moving new binary: %w", err)
		}
		return os.Chmod(execPath, info.Mode())
	}
	if err := os.Rename(newBinaryPath, execPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}
	return os.Chmod(execPath, info.Mode())
}

func compareCLIVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &aNum)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bNum)
		}
		if aNum > bNum {
			return 1
		}
		if aNum < bNum {
			return -1
		}
	}
	return 0
}
