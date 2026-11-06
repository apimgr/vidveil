// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI Auto-Update
package cmd

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/client/api"
	paths "github.com/apimgr/vidveil/src/client/paths"
	"github.com/apimgr/vidveil/src/config"
)

// CLIUpdateValidBranches enumerates branches per AI.md PART 22.
var CLIUpdateValidBranches = map[string]bool{
	"stable":      true,
	"beta":        true,
	"devel":       true,
	"daily":       true,
	"rolling":     true,
	"dev":         true,
	"development": true,
}

const cliUpdateRestartedEnv = "VIDVEIL_CLI_UPDATE_RESTARTED"

const cliUpdateHTTPTimeout = 30 * time.Second

func cliUpdateContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, cliUpdateHTTPTimeout)
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
	Checksum        string
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
		printCLIUpdateHelp()
		return nil
	case "check":
		return runCLIUpdateCheck()
	case "yes", "":
		return runCLIUpdateApply()
	case "branch":
		return runCLIUpdateBranch(arg)
	default:
		fmt.Fprintf(os.Stderr, "unknown update command: %s\n", cmd)
		printCLIUpdateHelp()
		return fmt.Errorf("unknown update command: %s", cmd)
	}
}

// printCLIUpdateHelp prints --update help and exits successfully per AI.md PART 8.
func printCLIUpdateHelp() {
	fmt.Printf(`Update Commands:
  %s --update              Check and perform in-place update with re-exec
  %s --update yes          Same as --update (default)
  %s --update check        Check for updates without installing
  %s --update branch NAME  Set update branch (stable, beta, devel)

Update Branches:
  stable (default)  Release builds (v*, *.*.*)
  beta              Pre-release builds (*-beta)
  devel             Rolling development builds
  daily, rolling, dev, development  Aliases for devel
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
	log.Printf("event=cli.update_started version=%s", info.LatestVersion)
	token := ""
	if cliConfig != nil {
		token = cliConfig.Server.Token
	}
	if envToken := getCLIAuthTokenFromEnv(); envToken != "" {
		token = envToken
	}
	tmpPath, err := downloadCLIBinary(info.DownloadURL, token)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer func() {
		if err := removeCLIUpdateTemp(tmpPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: clean up update temporary file: %v\n", err)
		}
	}()

	fmt.Println("Verifying SHA-256 checksum...")
	if err := verifyCLIBinaryChecksum(tmpPath, info.Checksum); err != nil {
		log.Printf("event=cli.update_checksum_invalid")
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	if err := replaceCLIBinary(execPath, tmpPath); err != nil {
		if isCLIBinaryPermissionError(err) {
			fmt.Fprintf(os.Stderr, "you do not have permission to update %s; ask your admin or move the binary to a writable path\n", execPath)
			return err
		}
		return fmt.Errorf("installing update: %w", err)
	}

	fmt.Println("Update installed; re-executing...")
	if err := restartCLI(execPath); err != nil {
		fmt.Fprintf(os.Stderr, "re-exec failed; please rerun the command manually: %v\n", err)
		return err
	}
	return nil
}

func isCLIBinaryPermissionError(err error) bool {
	return os.IsPermission(err) || errors.Is(err, fs.ErrPermission)
}

// logCLIUpdateCompletionIfRestarted records completion in the newly started
// process so Unix exec replacement and Windows child restart share one event.
func logCLIUpdateCompletionIfRestarted() {
	if os.Getenv(cliUpdateRestartedEnv) != "1" {
		return
	}
	_ = os.Unsetenv(cliUpdateRestartedEnv)
	log.Printf("event=cli.update_completed version=%s", Version)
}

func runCLIUpdateBranch(branch string) error {
	canonical, valid := config.NormalizeUpdateChannel(branch)
	if strings.TrimSpace(branch) == "" || !valid {
		return fmt.Errorf("invalid branch: %q (valid: stable, beta, devel)", branch)
	}
	if err := SetCLIUpdateBranch(canonical); err != nil {
		return err
	}
	fmt.Printf("Update branch set to: %s\n", canonical)
	return nil
}

// SetCLIUpdateBranch persists the selected channel in cli.yml.
func SetCLIUpdateBranch(branch string) error {
	canonical, valid := config.NormalizeUpdateChannel(branch)
	if !valid {
		return fmt.Errorf("invalid update channel: %q", branch)
	}
	if cliConfig == nil {
		if err := LoadCLIConfigFromFile(); err != nil {
			return fmt.Errorf("loading CLI config: %w", err)
		}
	}
	cliConfig.Update.Channel = canonical
	if err := writeCLIConfigFile(*cliConfig, getCLIConfigFilePath()); err != nil {
		return fmt.Errorf("saving update channel: %w", err)
	}
	return nil
}

// GetCLIUpdateBranch returns the configured update channel (defaulting to stable).
func GetCLIUpdateBranch() string {
	if cliConfig == nil {
		if err := LoadCLIConfigFromFile(); err != nil {
			return "stable"
		}
	}
	branch := strings.TrimSpace(cliConfig.Update.Channel)
	canonical, valid := config.NormalizeUpdateChannel(branch)
	if !valid {
		return "stable"
	}
	return canonical
}

// CheckCLIUpdate reads release metadata from the configured server's autodiscover API.
func CheckCLIUpdate() (*CLIUpdateInfo, error) {
	branch := GetCLIUpdateBranch()
	client := apiClient
	if client == nil {
		serverAddress := ""
		if cliConfig != nil {
			serverAddress = cliConfig.Server.Address
		}
		if serverAddress == "" {
			serverAddress = getCLIServerAddressFromEnv()
		}
		if serverAddress == "" {
			return nil, fmt.Errorf("server is not configured; set --server or cli.yml server.primary")
		}
		token := ""
		if cliConfig != nil {
			token = cliConfig.Server.Token
		}
		if envToken := getCLIAuthTokenFromEnv(); envToken != "" {
			token = envToken
		}
		client = api.NewAPIClient(serverAddress, token, 30, "v1")
	}
	discovery, err := client.AutodiscoverChannel(branch)
	if err != nil {
		return nil, fmt.Errorf("discovering CLI update: %w", err)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	versionInfo, ok := discovery.CLIVersions[platform]
	if len(discovery.CLIVersions) == 0 {
		currentVersion := strings.TrimPrefix(Version, "v")
		baseURL := strings.TrimRight(client.GetBaseURL(), "/")
		return &CLIUpdateInfo{
			CurrentVersion:  currentVersion,
			LatestVersion:   currentVersion,
			UpdateAvailable: false,
			ReleaseURL:      baseURL,
		}, nil
	}
	if !ok || versionInfo.Version == "" {
		return nil, fmt.Errorf("server has no CLI release for %s on channel %s", platform, branch)
	}
	currentVersion := strings.TrimPrefix(Version, "v")
	latestVersion := strings.TrimPrefix(versionInfo.Version, "v")
	filename := BinaryName + "-" + platform
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	baseURL := strings.TrimRight(client.GetBaseURL(), "/")
	info := &CLIUpdateInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: isCLIUpdateNewer(branch, latestVersion, currentVersion),
		ReleaseURL:      baseURL,
		DownloadURL:     baseURL + "/cli/binaries/" + strings.TrimSuffix(filename, ".exe") + "?channel=" + branch,
		AssetName:       filename,
		Checksum:        strings.ToLower(versionInfo.SHA256),
	}
	return info, nil
}

// cliMinimumVersionError blocks normal CLI requests when a server requires a
// newer client. An empty minimum means the server has not configured a floor.
func cliMinimumVersionError(currentVersion, minimumVersion string) error {
	minimumVersion = strings.TrimPrefix(strings.TrimSpace(minimumVersion), "v")
	if minimumVersion == "" || compareCLIVersions(currentVersion, minimumVersion) >= 0 {
		return nil
	}
	return fmt.Errorf("this CLI is too old; the server requires %s — run '%s --update yes' to upgrade", minimumVersion, BinaryName)
}

// shouldAutoUpdateCLI decides whether a newer version is installed silently.
func shouldAutoUpdateCLI(updateAvailable, autoUpdateEnabled, interactive bool) bool {
	return updateAvailable && autoUpdateEnabled && !interactive
}

// promptForCLIUpdate asks an interactive user whether to install an available
// update before the requested CLI operation proceeds.
func promptForCLIUpdate(reader io.Reader, writer io.Writer) (bool, error) {
	if _, err := fmt.Fprintf(writer, "A CLI update is available. Install it now? [y/N] "); err != nil {
		return false, err
	}
	answer, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading update confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// checkCLIUpdateBeforeRequests checks the server's update policy before the
// command makes any ordinary API requests. Autodiscover remains available to
// clients below the configured minimum so they can upgrade.
func checkCLIUpdateBeforeRequests(interactive bool) error {
	if apiClient == nil || cliConfig == nil || strings.TrimSpace(cliConfig.Server.Address) == "" {
		return nil
	}
	discovery, err := apiClient.AutodiscoverChannel(GetCLIUpdateBranch())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: CLI update check failed: %v\n", err)
		return nil
	}
	if err := cliMinimumVersionError(Version, discovery.CLIMinVersion); err != nil {
		log.Printf("event=cli.update_forced")
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	versionInfo, ok := discovery.CLIVersions[platform]
	if !ok || versionInfo.Version == "" {
		return nil
	}
	currentVersion := strings.TrimPrefix(Version, "v")
	latestVersion := strings.TrimPrefix(versionInfo.Version, "v")
	if !isCLIUpdateNewer(GetCLIUpdateBranch(), latestVersion, currentVersion) {
		return nil
	}

	install := shouldAutoUpdateCLI(true, cliConfig.Update.Auto, interactive)
	if interactive {
		install, err = promptForCLIUpdate(os.Stdin, os.Stderr)
		if err != nil {
			return err
		}
	}
	if install {
		return runCLIUpdateApply()
	}
	fmt.Fprintf(os.Stderr, "CLI update %s is available; run '%s --update yes' to install it\n", latestVersion, BinaryName)
	return nil
}

// Daily builds use commit IDs as versions, which have no chronological order.
// Any different published commit on the rolling devel channel is an update.
func isCLIUpdateNewer(channel, latestVersion, currentVersion string) bool {
	channel, _ = config.NormalizeUpdateChannel(channel)
	if channel == "devel" {
		return latestVersion != currentVersion
	}
	return compareCLIVersions(latestVersion, currentVersion) > 0
}

func cliReleaseBinaryName() string {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return fmt.Sprintf("%s-%s-%s%s", BinaryName, runtime.GOOS, runtime.GOARCH, suffix)
}

func fetchLatestCLIStableRelease() (*CLIGitHubRelease, error) {
	ctx, cancel := cliUpdateContext(context.Background())
	defer cancel()
	return fetchLatestCLIStableReleaseContext(ctx)
}

func fetchLatestCLIStableReleaseContext(ctx context.Context) (*CLIGitHubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", paths.GitHubOrg(), paths.GitHubRepo())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating latest release request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
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
	ctx, cancel := cliUpdateContext(context.Background())
	defer cancel()
	return fetchAllCLIReleasesContext(ctx)
}

func fetchAllCLIReleasesContext(ctx context.Context) ([]CLIGitHubRelease, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=50", paths.GitHubOrg(), paths.GitHubRepo())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating release list request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
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
// stable matches every channel; beta = {stable, beta}; devel = {stable, beta,
// devel}.
func matchesCLIBranch(r CLIGitHubRelease, branch string) bool {
	if !r.Prerelease {
		return true
	}
	isBeta := strings.HasSuffix(r.TagName, "-beta")
	isDevel := r.TagName == "devel"
	switch branch {
	case "beta":
		return isBeta
	case "devel":
		return isBeta || isDevel
	default:
		return false
	}
}

// fetchLatestCLIBetaRelease returns the newest of {stable, beta}. Releases are
// returned newest-first, so the first match wins.
func fetchLatestCLIBetaRelease() (*CLIGitHubRelease, error) {
	ctx, cancel := cliUpdateContext(context.Background())
	defer cancel()
	return fetchLatestCLIBetaReleaseContext(ctx)
}

func fetchLatestCLIBetaReleaseContext(ctx context.Context) (*CLIGitHubRelease, error) {
	releases, err := fetchAllCLIReleasesContext(ctx)
	if err != nil {
		return nil, err
	}
	for i := range releases {
		if matchesCLIBranch(releases[i], "beta") {
			return &releases[i], nil
		}
	}
	return nil, nil
}

// fetchLatestCLIDevelRelease returns the newest of {stable, beta, devel}.
// Releases are returned newest-first, so the first match wins.
func fetchLatestCLIDevelRelease() (*CLIGitHubRelease, error) {
	ctx, cancel := cliUpdateContext(context.Background())
	defer cancel()
	return fetchLatestCLIDevelReleaseContext(ctx)
}

func fetchLatestCLIDevelReleaseContext(ctx context.Context) (*CLIGitHubRelease, error) {
	releases, err := fetchAllCLIReleasesContext(ctx)
	if err != nil {
		return nil, err
	}
	for i := range releases {
		if matchesCLIBranch(releases[i], "devel") {
			return &releases[i], nil
		}
	}
	return nil, nil
}

func downloadCLIBinary(downloadURL string, tokenValues ...string) (string, error) {
	ctx, cancel := cliUpdateContext(context.Background())
	defer cancel()
	return downloadCLIBinaryContext(ctx, downloadURL, tokenValues...)
}

func downloadCLIBinaryContext(ctx context.Context, downloadURL string, tokenValues ...string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating download request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", BinaryName+"/"+Version)
	if len(tokenValues) > 0 && strings.TrimSpace(tokenValues[0]) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(tokenValues[0]))
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("downloading binary: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download status %d", resp.StatusCode)
	}

	tmpBase := filepath.Join(os.TempDir(), paths.GitHubOrg())
	if err := os.MkdirAll(tmpBase, 0700); err != nil {
		return "", fmt.Errorf("creating temp base dir: %w", err)
	}
	tmpDir, err := os.MkdirTemp(tmpBase, paths.InternalName()+"-")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(tmpDir, "update-*")
	if err != nil {
		if cleanupErr := os.Remove(tmpDir); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			err = errors.Join(err, fmt.Errorf("remove temp directory: %w", cleanupErr))
		}
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		closeErr := tmpFile.Close()
		return "", fmt.Errorf("writing temp file: %w", errors.Join(err, closeErr, removeCLIUpdateTemp(tmpPath)))
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", errors.Join(err, removeCLIUpdateTemp(tmpPath)))
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return "", fmt.Errorf("chmod temp file: %w", errors.Join(err, removeCLIUpdateTemp(tmpPath)))
	}
	return tmpPath, nil
}

// removeCLIUpdateTemp removes the downloaded update and its empty private dir.
func removeCLIUpdateTemp(tmpPath string) error {
	var cleanupErr error
	if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove temporary file: %w", err))
	}
	if err := os.Remove(filepath.Dir(tmpPath)); err != nil && !os.IsNotExist(err) {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove temporary directory: %w", err))
	}
	return cleanupErr
}

func verifyCLIChecksum(binaryPath, checksumURL, assetName string) error {
	ctx, cancel := cliUpdateContext(context.Background())
	defer cancel()
	return verifyCLIChecksumContext(ctx, binaryPath, checksumURL, assetName)
}

func verifyCLIChecksumContext(ctx context.Context, binaryPath, checksumURL, assetName string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return fmt.Errorf("creating checksum request: %w", err)
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("downloading checksum: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
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
	defer func() { _ = binaryFile.Close() }()
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

func verifyCLIBinaryChecksum(binaryPath, expected string) error {
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("invalid expected SHA-256 checksum")
	}
	binaryFile, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("opening downloaded binary: %w", err)
	}
	defer func() { _ = binaryFile.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, binaryFile); err != nil {
		return fmt.Errorf("hashing downloaded binary: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("sha-256 mismatch (expected %s, got %s)", expected, actual)
	}
	return nil
}

func compareCLIVersions(a, b string) int {
	aNumbers, aPrerelease, aValid := parseCLIVersion(a)
	bNumbers, bPrerelease, bValid := parseCLIVersion(b)
	if aValid != bValid {
		if aValid {
			return 1
		}
		return -1
	}
	if !aValid {
		if a == b {
			return 0
		}
		return strings.Compare(a, b)
	}
	for index := range aNumbers {
		if aNumbers[index] > bNumbers[index] {
			return 1
		}
		if aNumbers[index] < bNumbers[index] {
			return -1
		}
	}
	if aPrerelease == bPrerelease {
		return 0
	}
	if aPrerelease {
		return -1
	}
	return 1
}

func parseCLIVersion(version string) ([3]int, bool, bool) {
	var numbers [3]int
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if len(version) == 14 && allVersionDigits(version) {
		return numbers, false, false
	}
	core, prerelease, _ := strings.Cut(version, "-")
	core, _, _ = strings.Cut(core, "+")
	parts := strings.Split(core, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return numbers, false, false
	}
	for index, part := range parts {
		if part == "" {
			return numbers, false, false
		}
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return numbers, false, false
		}
		numbers[index] = parsed
	}
	for index := len(parts); index < len(numbers); index++ {
		numbers[index] = 0
	}
	return numbers, prerelease != "", true
}

func allVersionDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return value != ""
}
