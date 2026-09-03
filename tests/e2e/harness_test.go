//go:build e2e

// Package e2e holds the on-demand browser end-to-end suite (AI.md PART 28).
// It is guarded by the "e2e" build tag so `make test` never runs it.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// suite holds every resource shared by the three tiers.
type suite struct {
	projectRoot string
	workDir     string
	artifactDir string
	binary      string
	port        int
	// localBase is the URL the Go test process uses (Tier 1)
	localBase string
	// browserBase is the URL the headless browser uses (Tiers 2 and 3)
	browserBase string
	server      *exec.Cmd
	serverLog   *os.File
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

var s *suite

// TestMain builds the server, seeds a fresh isolated data directory, starts the
// server on a port in the PART 5 range, connects to headless Chromium, and
// tears everything down afterwards.
func TestMain(m *testing.M) {
	code := 1
	func() {
		var err error
		s, err = newSuite()
		if err != nil {
			fmt.Fprintf(os.Stderr, "e2e setup failed: %v\n", err)
			return
		}
		defer s.teardown()
		code = m.Run()
	}()
	os.Exit(code)
}

func newSuite() (*suite, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, err
	}

	st := &suite{projectRoot: root}

	st.workDir = os.Getenv("E2E_WORK_DIR")
	if st.workDir == "" {
		st.workDir, err = os.MkdirTemp("", "vidveil-e2e-")
		if err != nil {
			return nil, err
		}
	}

	st.artifactDir = os.Getenv("E2E_ARTIFACT_DIR")
	if st.artifactDir == "" {
		st.artifactDir = filepath.Join(st.workDir, "artifacts")
	}
	if err := os.MkdirAll(st.artifactDir, 0o750); err != nil {
		return nil, err
	}

	if err := st.build(); err != nil {
		return nil, err
	}
	if err := st.start(); err != nil {
		return nil, err
	}
	if err := st.connectBrowser(); err != nil {
		st.stopServer()
		return nil, err
	}
	return st, nil
}

// findProjectRoot walks up from the working directory until it finds go.mod.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func (st *suite) build() error {
	st.binary = filepath.Join(st.workDir, "vidveil")
	cmd := exec.Command("go", "build", "-trimpath", "-o", st.binary, "./src")
	cmd.Dir = st.projectRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=-buildvcs=false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build failed: %v\n%s", err, out)
	}
	return nil
}

// freePort picks an unused port from the 64000-64999 range (AI.md PART 5).
func freePort() (int, error) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	for attempt := 0; attempt < 200; attempt++ {
		port := 64000 + rnd.Intn(1000)
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			continue
		}
		if cerr := ln.Close(); cerr != nil {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("no free port in 64000-64999")
}

func (st *suite) start() error {
	port, err := freePort()
	if err != nil {
		return err
	}
	st.port = port

	configDir := filepath.Join(st.workDir, "config")
	dataDir := filepath.Join(st.workDir, "data")
	cacheDir := filepath.Join(st.workDir, "cache")
	logDir := filepath.Join(st.workDir, "log")
	for _, dir := range []string{configDir, dataDir, cacheDir, logDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}

	// Hermetic config: no scheduler (no GeoIP/update/blocklist downloads) and no
	// rate limiting, so the suite passes offline and a full crawl is not throttled
	cfg := fmt.Sprintf(`server:
  port: %d
  address: "0.0.0.0"
  mode: development
  scheduler:
    enabled: false
  rate_limit:
    enabled: false
  update:
    auto_install: false
`, port)
	if err := os.WriteFile(filepath.Join(configDir, "server.yml"), []byte(cfg), 0o600); err != nil {
		return err
	}

	logPath := filepath.Join(st.artifactDir, "server.log")
	st.serverLog, err = os.Create(logPath)
	if err != nil {
		return err
	}

	st.server = exec.Command(st.binary,
		"--config", configDir,
		"--data", dataDir,
		"--cache", cacheDir,
		"--log", logDir,
		"--address", "0.0.0.0",
		"--port", fmt.Sprintf("%d", port),
		"--mode", "development",
	)
	st.server.Stdout = st.serverLog
	st.server.Stderr = st.serverLog
	st.server.Env = append(os.Environ(), "NO_COLOR=1", "TZ=UTC")
	if err := st.server.Start(); err != nil {
		return err
	}

	st.localBase = fmt.Sprintf("http://127.0.0.1:%d", port)
	host := os.Getenv("E2E_SERVER_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	st.browserBase = fmt.Sprintf("http://%s:%d", host, port)

	return st.waitReady()
}

func (st *suite) waitReady() error {
	deadline := time.Now().Add(90 * time.Second)
	client := &http.Client{Timeout: 5 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(st.localBase + "/server/healthz")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	logBytes, _ := os.ReadFile(filepath.Join(st.artifactDir, "server.log"))
	return fmt.Errorf("server never became healthy on %s\n%s", st.localBase, logBytes)
}

// connectBrowser attaches to the Chromium sidecar when E2E_CHROME_URL is set,
// and otherwise launches a local headless Chromium.
func (st *suite) connectBrowser() error {
	if endpoint := os.Getenv("E2E_CHROME_URL"); endpoint != "" {
		ws, err := devtoolsWebSocket(endpoint)
		if err != nil {
			return err
		}
		st.allocCtx, st.allocCancel = chromedp.NewRemoteAllocator(context.Background(), ws)
		return nil
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("headless", true),
	)
	st.allocCtx, st.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	return nil
}

// devtoolsWebSocket resolves the browser websocket endpoint, rewriting the host
// reported by Chromium (usually localhost) to the address we can actually reach.
func devtoolsWebSocket(endpoint string) (string, error) {
	base, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(strings.TrimRight(endpoint, "/") + "/json/version")
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		var payload struct {
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || payload.WebSocketDebuggerURL == "" {
			lastErr = fmt.Errorf("unexpected /json/version payload: %s", body)
			time.Sleep(time.Second)
			continue
		}
		ws, err := url.Parse(payload.WebSocketDebuggerURL)
		if err != nil {
			return "", err
		}
		ws.Host = base.Host
		return ws.String(), nil
	}
	return "", fmt.Errorf("chromium devtools endpoint %s never answered: %v", endpoint, lastErr)
}

func (st *suite) stopServer() {
	if st.server == nil || st.server.Process == nil {
		return
	}
	_ = st.server.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_, _ = st.server.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = st.server.Process.Kill()
	}
	st.server = nil
}

func (st *suite) teardown() {
	if st.allocCancel != nil {
		st.allocCancel()
	}
	st.stopServer()
	if st.serverLog != nil {
		_ = st.serverLog.Close()
	}
}

// httpClient returns a cookie-jar client that never follows redirects, so tests
// can assert on 3xx status codes and Location headers directly.
func httpClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// get performs a Tier 1 (SSR) request and returns the response and its body.
func get(t *testing.T, client *http.Client, path string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, s.localBase+path, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", path, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) vidveil-e2e")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return resp, string(body)
}

// newTab creates a browser tab and records console errors and failed requests.
type tab struct {
	ctx      context.Context
	cancel   context.CancelFunc
	errorsMu chan struct{}
	errors   *[]string
}

func newTab(t *testing.T) *tab {
	t.Helper()
	ctx, cancel := chromedp.NewContext(s.allocCtx)
	timed, timeout := context.WithTimeout(ctx, 90*time.Second)

	collected := &[]string{}
	lock := make(chan struct{}, 1)
	lock <- struct{}{}

	chromedp.ListenTarget(timed, func(ev interface{}) {
		var msg string
		switch e := ev.(type) {
		case *runtime.EventExceptionThrown:
			if e.ExceptionDetails != nil {
				msg = "exception: " + e.ExceptionDetails.Error()
			}
		case *runtime.EventConsoleAPICalled:
			if e.Type == runtime.APITypeError {
				parts := make([]string, 0, len(e.Args))
				for _, arg := range e.Args {
					parts = append(parts, string(arg.Value))
				}
				msg = "console.error: " + strings.Join(parts, " ")
			}
		}
		if msg == "" {
			return
		}
		<-lock
		*collected = append(*collected, msg)
		lock <- struct{}{}
	})

	tb := &tab{
		ctx: timed,
		cancel: func() {
			timeout()
			cancel()
		},
		errorsMu: lock,
		errors:   collected,
	}
	t.Cleanup(tb.cancel)
	return tb
}

// consoleErrors returns a copy of the console errors seen so far in this tab.
func (tb *tab) consoleErrors() []string {
	<-tb.errorsMu
	out := append([]string(nil), *tb.errors...)
	tb.errorsMu <- struct{}{}
	return out
}

// saveArtifact writes failure evidence to the tempdir, never the project tree.
func saveArtifact(t *testing.T, name, content string) {
	t.Helper()
	safe := strings.NewReplacer("/", "_", " ", "_").Replace(name)
	path := filepath.Join(s.artifactDir, safe)
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Logf("could not write artifact %s: %v", path, err)
		return
	}
	t.Logf("artifact written: %s", path)
}
