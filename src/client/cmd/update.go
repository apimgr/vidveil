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
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/apimgr/vidveil/src/client/paths"
)

// CLIUpdateBranchFile is the on-disk record of the user-selected update channel.
const CLIUpdateBranchFile = "update-branch"

// CLIUpdateValidBranches enumerates branches per AI.md PART 23.
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
  daily             Daily builds (YYYYMMDDHHMM)
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

	if info.ChecksumURL != "" {
		fmt.Println("Verifying SHA-256 checksum...")
		if err := verifyCLIChecksum(tmpPath, info.ChecksumURL); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
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
	configDir := paths.ConfigDir()
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
	branchFile := filepath.Join(paths.ConfigDir(), CLIUpdateBranchFile)
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

	binaryAssetName := cliReleaseBinaryName()
	checksumAssetName := binaryAssetName + ".sha256"
	for _, asset := range release.Assets {
		switch asset.Name {
		case binaryAssetName:
			info.DownloadURL = asset.BrowserDownloadURL
		case checksumAssetName:
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
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", paths.GitHubOrg(), paths.GitHubRepo())
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
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=50", paths.GitHubOrg(), paths.GitHubRepo())
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

func fetchLatestCLIBetaRelease() (*CLIGitHubRelease, error) {
	releases, err := fetchAllCLIReleases()
	if err != nil {
		return nil, err
	}
	for i := range releases {
		if strings.Contains(strings.ToLower(releases[i].TagName), "-beta") {
			return &releases[i], nil
		}
	}
	return nil, fmt.Errorf("no beta releases found")
}

func fetchLatestCLIDailyRelease() (*CLIGitHubRelease, error) {
	releases, err := fetchAllCLIReleases()
	if err != nil {
		return nil, err
	}
	dailyPattern := regexp.MustCompile(`^\d{12}$`)
	for i := range releases {
		if dailyPattern.MatchString(releases[i].TagName) {
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

	tmpDir := filepath.Join(os.TempDir(), paths.GitHubOrg())
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

func verifyCLIChecksum(binaryPath, checksumURL string) error {
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
	expected := strings.ToLower(strings.TrimSpace(strings.Fields(string(body))[0]))

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
