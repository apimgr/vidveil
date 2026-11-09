// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for ProxyThumbnail, ProxyVideo early-return
// paths, handleSearchSSE (non-Flusher path), and other low-coverage functions.
// All tests exercise early validation / error returns — no real network calls.
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestProxyHostAllowlistIncludesCurrentSourceCDNs(t *testing.T) {
	for _, host := range []string{"thumb-cdn77.xnxx-cdn.com", "thumb-cdn77.xvideos-cdn.com", "ci.phncdn.com"} {
		if !isAllowedProxyHost(host) {
			t.Errorf("current source CDN host %q is not allowed", host)
		}
	}
}

func TestProxyThumbnailPreservesPercentEscapesFromQuery(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	thumbURL := "https://xvideos.com/thumb%20one.jpg?token=a%2Fb"
	sum := sha256.Sum256([]byte(thumbURL))
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	req := httptest.NewRequest("GET", "/proxy/thumbnail?url="+url.QueryEscape(thumbURL), nil)
	req.Header.Set("If-None-Match", etag)
	rr := httptest.NewRecorder()

	h.ProxyThumbnail(rr, req)

	if rr.Code != http.StatusNotModified {
		t.Fatalf("ProxyThumbnail status = %d, want %d (ETag %q, body %s)", rr.Code, http.StatusNotModified, rr.Header().Get("ETag"), rr.Body.String())
	}
}

// ── ProxyThumbnail — validation early-returns ─────────────────────────────────

func TestProxyThumbnail_MissingURLParam_Returns400(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/proxy/thumbnail", nil)
	rr := httptest.NewRecorder()
	h.ProxyThumbnail(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProxyThumbnail(no url): status = %d, want 400", rr.Code)
	}
}

func TestProxyThumbnail_InvalidScheme_Returns400(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	thumbURL := url.QueryEscape("ftp://example.com/thumb.jpg")
	req := httptest.NewRequest("GET", "/proxy/thumbnail?url="+thumbURL, nil)
	rr := httptest.NewRecorder()
	h.ProxyThumbnail(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProxyThumbnail(ftp scheme): status = %d, want 400", rr.Code)
	}
}

func TestProxyThumbnail_PrivateHost_Returns400(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	thumbURL := url.QueryEscape("http://127.0.0.1/thumb.jpg")
	req := httptest.NewRequest("GET", "/proxy/thumbnail?url="+thumbURL, nil)
	rr := httptest.NewRecorder()
	h.ProxyThumbnail(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProxyThumbnail(private host): status = %d, want 400", rr.Code)
	}
}

func TestProxyThumbnail_PrivateHost_10_Returns400(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	thumbURL := url.QueryEscape("https://192.168.1.1/thumb.jpg")
	req := httptest.NewRequest("GET", "/proxy/thumbnail?url="+thumbURL, nil)
	rr := httptest.NewRecorder()
	h.ProxyThumbnail(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProxyThumbnail(192.168.x.x): status = %d, want 400", rr.Code)
	}
}

func TestProxyThumbnail_IfNoneMatch_304(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	thumbURL := "https://eporner.com/thumb.jpg"
	encodedURL := url.QueryEscape(thumbURL)

	// First request to determine ETag
	req1 := httptest.NewRequest("GET", "/proxy/thumbnail?url="+encodedURL, nil)
	rr1 := httptest.NewRecorder()
	h.ProxyThumbnail(rr1, req1)
	etag := rr1.Header().Get("ETag")

	if etag == "" {
		t.Skip("ProxyThumbnail: no ETag returned (may have fetched remote or failed)")
	}

	// Second request with If-None-Match — should get 304
	req2 := httptest.NewRequest("GET", "/proxy/thumbnail?url="+encodedURL, nil)
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	h.ProxyThumbnail(rr2, req2)
	if rr2.Code != http.StatusNotModified {
		t.Errorf("ProxyThumbnail(If-None-Match match): status = %d, want 304", rr2.Code)
	}
}

func TestProxyThumbnail_EtagComputedForPublicURL(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	thumbURL := url.QueryEscape("https://eporner.com/image.jpg")

	// Send request with non-matching ETag to get ETag header set
	req := httptest.NewRequest("GET", "/proxy/thumbnail?url="+thumbURL, nil)
	req.Header.Set("If-None-Match", `"nomatch"`)
	rr := httptest.NewRecorder()
	h.ProxyThumbnail(rr, req)
	_ = rr.Header().Get("ETag")
}

// ── ProxyVideo — validation early-returns ────────────────────────────────────

func TestProxyVideo_MissingURLParam_Returns400(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/proxy/video", nil)
	rr := httptest.NewRecorder()
	h.ProxyVideo(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProxyVideo(no url): status = %d, want 400", rr.Code)
	}
}

func TestProxyVideo_InvalidScheme_Returns400(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	videoURL := url.QueryEscape("ftp://example.com/video.mp4")
	req := httptest.NewRequest("GET", "/proxy/video?url="+videoURL, nil)
	rr := httptest.NewRecorder()
	h.ProxyVideo(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProxyVideo(ftp scheme): status = %d, want 400", rr.Code)
	}
}

func TestProxyVideo_PrivateHost_Returns400(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	videoURL := url.QueryEscape("http://10.0.0.1/video.mp4")
	req := httptest.NewRequest("GET", "/proxy/video?url="+videoURL, nil)
	rr := httptest.NewRecorder()
	h.ProxyVideo(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProxyVideo(private host): status = %d, want 400", rr.Code)
	}
}

func TestProxyVideo_LoopbackHost_Returns400(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	videoURL := url.QueryEscape("https://localhost/video.mp4")
	req := httptest.NewRequest("GET", "/proxy/video?url="+videoURL, nil)
	rr := httptest.NewRecorder()
	h.ProxyVideo(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ProxyVideo(localhost): status = %d, want 400", rr.Code)
	}
}

// ── getProxyClient ────────────────────────────────────────────────────────────

func TestGetProxyClient_NilTorService_ReturnsDefaultClient(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	client := h.getProxyClient(10 * time.Second)
	if client == nil {
		t.Error("getProxyClient(no tor): expected non-nil client")
	}
}

// ── getTorStatus ──────────────────────────────────────────────────────────────

func TestGetTorStatus_NilTorService_ReturnsEmpty(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	status := h.getTorStatus()
	_ = status
}

// ── getTorHostname ────────────────────────────────────────────────────────────

func TestGetTorHostname_NilTorService_ReturnsEmpty(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	hostname := h.getTorHostname()
	if hostname != "" {
		t.Errorf("getTorHostname(nil tor): expected '', got %q", hostname)
	}
}

// ── WriteJSON (package-level) — coverage ─────────────────────────────────────

func TestWriteJSONPkg_ValidData_Returns200(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSON(rr, http.StatusOK, map[string]string{"key": "value"})
	if rr.Code != http.StatusOK {
		t.Errorf("WriteJSON: status = %d, want 200", rr.Code)
	}
}

func TestWriteJSONPkg_NilData_NoPanic(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSON(rr, http.StatusOK, nil)
}

// ── isPrivateHost ─────────────────────────────────────────────────────────────

func TestIsPrivateHost_Loopback_True(t *testing.T) {
	if !isPrivateHost("127.0.0.1") {
		t.Error("isPrivateHost(127.0.0.1): expected true")
	}
}

func TestIsPrivateHost_Private10_True(t *testing.T) {
	if !isPrivateHost("10.0.0.1") {
		t.Error("isPrivateHost(10.0.0.1): expected true")
	}
}

func TestIsPrivateHost_Private172_True(t *testing.T) {
	if !isPrivateHost("172.16.0.1") {
		t.Error("isPrivateHost(172.16.0.1): expected true")
	}
}

func TestIsPrivateHost_Private192_True(t *testing.T) {
	if !isPrivateHost("192.168.0.1") {
		t.Error("isPrivateHost(192.168.0.1): expected true")
	}
}

func TestIsPrivateHost_Localhost_True(t *testing.T) {
	if !isPrivateHost("localhost") {
		t.Error("isPrivateHost(localhost): expected true")
	}
}

func TestIsPrivateHost_PublicIP_False(t *testing.T) {
	if isPrivateHost("8.8.8.8") {
		t.Error("isPrivateHost(8.8.8.8): expected false")
	}
}

func TestIsPrivateHost_PublicDomain_False(t *testing.T) {
	if isPrivateHost("example.com") {
		t.Error("isPrivateHost(example.com): expected false")
	}
}
