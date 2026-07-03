// SPDX-License-Identifier: MIT
// Coverage tests for pure helper functions in the engine package.
// Only functions that do not make network requests are tested here.
package engine

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/parser"
)

// ── mockTorProvider for getClientForCtx tests ─────────────────────────────────

type mockTorProvider struct {
	outboundEnabled bool
	shouldUseTor    bool
}

func (m *mockTorProvider) GetHTTPClient(_ bool) *http.Client { return &http.Client{} }
func (m *mockTorProvider) OutboundEnabled() bool             { return m.outboundEnabled }
func (m *mockTorProvider) UseNetworkEnabled() bool           { return m.outboundEnabled }
func (m *mockTorProvider) AllowUserPreference() bool         { return false }
func (m *mockTorProvider) AllowUserIPForward() bool          { return false }
func (m *mockTorProvider) ShouldUseTor(_ *bool) bool         { return m.shouldUseTor }

// testCfg returns a default AppConfig suitable for engine construction in tests.
func testCfg() *config.AppConfig {
	return config.DefaultAppConfig()
}

// ── extractJSON (xhamster.go) ─────────────────────────────────────────────────

func TestExtractJSON_ValidObject(t *testing.T) {
	input := []byte(`{"key":"value"}extra`)
	got := extractJSON(input)
	if string(got) != `{"key":"value"}` {
		t.Errorf("extractJSON = %q, want %q", got, `{"key":"value"}`)
	}
}

func TestExtractJSON_Empty(t *testing.T) {
	if extractJSON(nil) != nil {
		t.Error("extractJSON(nil): expected nil")
	}
	if extractJSON([]byte{}) != nil {
		t.Error("extractJSON([]byte{}): expected nil")
	}
}

func TestExtractJSON_NotStartingWithBrace(t *testing.T) {
	input := []byte(`[{"key":"value"}]`)
	if extractJSON(input) != nil {
		t.Error("extractJSON: expected nil for input not starting with '{'")
	}
}

func TestExtractJSON_Nested(t *testing.T) {
	input := []byte(`{"a":{"b":"c"}}trailing`)
	got := extractJSON(input)
	if string(got) != `{"a":{"b":"c"}}` {
		t.Errorf("extractJSON nested = %q, want %q", got, `{"a":{"b":"c"}}`)
	}
}

func TestExtractJSON_WithEscapedQuotes(t *testing.T) {
	input := []byte(`{"key":"he said \"hi\""}`)
	got := extractJSON(input)
	if got == nil {
		t.Error("extractJSON: expected non-nil for escaped quotes")
	}
}

func TestExtractJSON_Unclosed(t *testing.T) {
	input := []byte(`{"key":"value"`)
	if extractJSON(input) != nil {
		t.Error("extractJSON: expected nil for unclosed object")
	}
}

// ── formatDuration (xhamster.go) ──────────────────────────────────────────────

func TestFormatDuration_Zero(t *testing.T) {
	if got := formatDuration(0); got != "" {
		t.Errorf("formatDuration(0) = %q, want empty", got)
	}
}

func TestFormatDuration_Negative(t *testing.T) {
	if got := formatDuration(-5); got != "" {
		t.Errorf("formatDuration(-5) = %q, want empty", got)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	if got := formatDuration(45); got != "0:45" {
		t.Errorf("formatDuration(45) = %q, want '0:45'", got)
	}
}

func TestFormatDuration_MinutesSeconds(t *testing.T) {
	if got := formatDuration(125); got != "2:05" {
		t.Errorf("formatDuration(125) = %q, want '2:05'", got)
	}
}

func TestFormatDuration_Hours(t *testing.T) {
	if got := formatDuration(3661); got != "1:01:01" {
		t.Errorf("formatDuration(3661) = %q, want '1:01:01'", got)
	}
}

func TestFormatDuration_FullHour(t *testing.T) {
	if got := formatDuration(3600); got != "1:00:00" {
		t.Errorf("formatDuration(3600) = %q, want '1:00:00'", got)
	}
}

// ── XNXXEngine.convertToResult ────────────────────────────────────────────────

func TestXNXXConvertToResult_Basic(t *testing.T) {
	e := NewXNXXEngine(testCfg())
	item := &parser.VideoItem{
		URL:       "https://www.xnxx.com/video-12345/test",
		Title:     "Test Video",
		Thumbnail: "https://example.com/thumb.jpg",
		Duration:  "5:30",
		Views:     "1M",
	}
	r := e.convertToResult(item)
	if r.URL != item.URL {
		t.Errorf("convertToResult URL = %q, want %q", r.URL, item.URL)
	}
	if r.Title != item.Title {
		t.Errorf("convertToResult Title = %q, want %q", r.Title, item.Title)
	}
	if r.Source != e.Name() {
		t.Errorf("convertToResult Source = %q, want %q", r.Source, e.Name())
	}
}

func TestXNXXConvertToResult_DownloadURLFallback(t *testing.T) {
	e := NewXNXXEngine(testCfg())
	item := &parser.VideoItem{
		URL:         "https://www.xnxx.com/video-12345/test",
		DownloadURL: "",
	}
	r := e.convertToResult(item)
	if r.DownloadURL != item.URL {
		t.Errorf("convertToResult DownloadURL fallback = %q, want %q", r.DownloadURL, item.URL)
	}
}

func TestXNXXConvertToResult_WithDownloadURL(t *testing.T) {
	e := NewXNXXEngine(testCfg())
	item := &parser.VideoItem{
		URL:         "https://www.xnxx.com/video",
		DownloadURL: "https://dl.xnxx.com/video.mp4",
	}
	r := e.convertToResult(item)
	if r.DownloadURL != "https://dl.xnxx.com/video.mp4" {
		t.Errorf("convertToResult DownloadURL = %q, want explicit URL", r.DownloadURL)
	}
}

// ── XVideosEngine.convertToResult ─────────────────────────────────────────────

func TestXVideosConvertToResult_Basic(t *testing.T) {
	e := NewXVideosEngine(testCfg())
	item := &parser.VideoItem{
		URL:      "https://www.xvideos.com/video12345/test",
		Title:    "Test XVideos",
		Duration: "10:00",
	}
	r := e.convertToResult(item)
	if r.Title != item.Title {
		t.Errorf("XVideos convertToResult Title = %q", r.Title)
	}
	if r.Source != e.Name() {
		t.Errorf("XVideos convertToResult Source = %q, want %q", r.Source, e.Name())
	}
}

func TestXVideosConvertToResult_DownloadURLFallback(t *testing.T) {
	e := NewXVideosEngine(testCfg())
	item := &parser.VideoItem{URL: "https://www.xvideos.com/video"}
	r := e.convertToResult(item)
	if r.DownloadURL != item.URL {
		t.Errorf("XVideos DownloadURL fallback = %q, want %q", r.DownloadURL, item.URL)
	}
}

// ── PornHubEngine.convertToResult ─────────────────────────────────────────────

func TestPornHubConvertToResult_Basic(t *testing.T) {
	e := NewPornHubEngine(testCfg())
	item := &parser.VideoItem{
		URL:   "https://www.pornhub.com/view_video.php?viewkey=abc",
		Title: "Test PH Video",
		Views: "500K",
	}
	r := e.convertToResult(item)
	if r.URL != item.URL {
		t.Errorf("PornHub convertToResult URL = %q", r.URL)
	}
}

func TestPornHubConvertToResult_Rating(t *testing.T) {
	e := NewPornHubEngine(testCfg())
	item := &parser.VideoItem{
		URL:    "https://www.pornhub.com/view_video.php?viewkey=abc",
		Rating: "87",
	}
	r := e.convertToResult(item)
	if r.Rating != 87.0 {
		t.Errorf("PornHub convertToResult Rating = %v, want 87.0", r.Rating)
	}
}

func TestPornHubConvertToResult_InvalidRating(t *testing.T) {
	e := NewPornHubEngine(testCfg())
	item := &parser.VideoItem{
		URL:    "https://www.pornhub.com/view_video.php?viewkey=abc",
		Rating: "invalid",
	}
	r := e.convertToResult(item)
	if r.Rating != 0.0 {
		t.Errorf("PornHub convertToResult invalid Rating = %v, want 0.0", r.Rating)
	}
}

func TestPornHubConvertToResult_DownloadURLFallback(t *testing.T) {
	e := NewPornHubEngine(testCfg())
	item := &parser.VideoItem{URL: "https://www.pornhub.com/view_video.php?viewkey=abc"}
	r := e.convertToResult(item)
	if r.DownloadURL != item.URL {
		t.Errorf("PornHub DownloadURL fallback = %q, want %q", r.DownloadURL, item.URL)
	}
}

// ── PornHubEngine.formatViews ──────────────────────────────────────────────────

func TestFormatViews_Zero(t *testing.T) {
	got := formatViews(0)
	if got != "0 views" {
		t.Errorf("formatViews(0) = %q, want '0 views'", got)
	}
}

// ── PornMDEngine.convertToResult ──────────────────────────────────────────────

func TestPornMDConvertToResult_Basic(t *testing.T) {
	e := NewPornMDEngine(testCfg())
	item := &parser.VideoItem{
		URL:   "https://www.pornmd.com/video/123/test",
		Title: "PornMD Test",
	}
	r := e.convertToResult(item)
	if r.URL != item.URL {
		t.Errorf("PornMD convertToResult URL = %q", r.URL)
	}
	if r.Source != e.Name() {
		t.Errorf("PornMD convertToResult Source = %q, want %q", r.Source, e.Name())
	}
}

func TestPornMDConvertToResult_DescriptionWithQuality(t *testing.T) {
	e := NewPornMDEngine(testCfg())
	item := &parser.VideoItem{
		URL:         "https://www.pornmd.com/video/123/test",
		Quality:     "HD",
		Description: "some desc",
	}
	r := e.convertToResult(item)
	if !strings.Contains(r.Description, "HD") {
		t.Errorf("PornMD description should contain quality, got %q", r.Description)
	}
	if !strings.Contains(r.Description, "some desc") {
		t.Errorf("PornMD description should contain item description, got %q", r.Description)
	}
}

// ── RedTubeEngine.convertToResult ─────────────────────────────────────────────

func TestRedTubeConvertToResult_Basic(t *testing.T) {
	e := NewRedTubeEngine(testCfg())
	item := &parser.VideoItem{
		URL:   "https://www.redtube.com/123456",
		Title: "RedTube Test",
		Views: "10K",
	}
	r := e.convertToResult(item)
	if r.URL != item.URL {
		t.Errorf("RedTube convertToResult URL = %q", r.URL)
	}
}

func TestRedTubeConvertToResult_Rating(t *testing.T) {
	e := NewRedTubeEngine(testCfg())
	item := &parser.VideoItem{
		URL:    "https://www.redtube.com/123456",
		Rating: "92.5",
	}
	r := e.convertToResult(item)
	if r.Rating != 92.5 {
		t.Errorf("RedTube convertToResult Rating = %v, want 92.5", r.Rating)
	}
}

func TestRedTubeConvertToResult_DownloadURLFallback(t *testing.T) {
	e := NewRedTubeEngine(testCfg())
	item := &parser.VideoItem{URL: "https://www.redtube.com/123456"}
	r := e.convertToResult(item)
	if r.DownloadURL != item.URL {
		t.Errorf("RedTube DownloadURL fallback = %q, want %q", r.DownloadURL, item.URL)
	}
}

// ── MotherlessEngine.convertToResult ──────────────────────────────────────────

func TestMotherlessConvertToResult_Basic(t *testing.T) {
	e := NewMotherlessEngine(testCfg())
	item := &parser.VideoItem{
		URL:   "https://motherless.com/ABC1234",
		Title: "Motherless Test",
	}
	r := e.convertToResult(item)
	if r.URL != item.URL {
		t.Errorf("Motherless convertToResult URL = %q", r.URL)
	}
	if r.Source != e.Name() {
		t.Errorf("Motherless convertToResult Source = %q, want %q", r.Source, e.Name())
	}
}

// ── getClientForCtx ───────────────────────────────────────────────────────────

func TestGetClientForCtx_NilTorProvider_ReturnsHTTPClient(t *testing.T) {
	e := NewPornHubEngine(testCfg())
	client := e.getClientForCtx(context.Background())
	if client == nil {
		t.Error("getClientForCtx nil torProvider: returned nil client")
	}
}

func TestGetClientForCtx_TorDisabled_ReturnsHTTPClient(t *testing.T) {
	e := NewPornHubEngine(testCfg())
	e.SetTorProvider(&mockTorProvider{outboundEnabled: false, shouldUseTor: false})
	client := e.getClientForCtx(context.Background())
	if client == nil {
		t.Error("getClientForCtx tor disabled: returned nil client")
	}
}

func TestGetClientForCtx_TorEnabled_ShouldUseTor_ReturnsTorClient(t *testing.T) {
	e := NewPornHubEngine(testCfg())
	e.SetTorProvider(&mockTorProvider{outboundEnabled: true, shouldUseTor: true})
	client := e.getClientForCtx(context.Background())
	if client == nil {
		t.Error("getClientForCtx tor enabled + shouldUseTor: returned nil client")
	}
}

func TestGetClientForCtx_TorEnabled_ShouldNotUseTor_ReturnsDirect(t *testing.T) {
	e := NewPornHubEngine(testCfg())
	e.SetTorProvider(&mockTorProvider{outboundEnabled: true, shouldUseTor: false})
	client := e.getClientForCtx(context.Background())
	if client == nil {
		t.Error("getClientForCtx tor enabled + shouldNotUseTor: returned nil client")
	}
}
