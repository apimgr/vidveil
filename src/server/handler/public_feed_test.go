// SPDX-License-Identifier: MIT
package handler

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

func TestPublicFeedFormatsAndConditionalRequests(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.SEO.Feed.MaxItems = 5
	h := &SearchHandler{appConfig: cfg, recentSearches: []recentSearch{{Query: `privacy & safety`, At: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)}}}
	request := publicFeedRequest(http.MethodGet, "/feed.xml")
	response := httptest.NewRecorder()
	h.PublicFeed(response, request, true)
	if response.Code != http.StatusOK {
		t.Fatalf("Atom status = %d, want 200", response.Code)
	}
	for _, expected := range []string{"application/atom+xml; charset=utf-8", urlvar.BuildURL(request, "/feed.xml"), "privacy &amp; safety", urlvar.BuildURL(request, "/search?q=privacy+%26+safety")} {
		if !strings.Contains(response.Header().Get("Content-Type")+response.Body.String(), expected) {
			t.Errorf("Atom response missing %q: %s", expected, response.Body.String())
		}
	}
	if response.Header().Get("Cache-Control") != "public, max-age=300" || response.Header().Get("ETag") == "" || response.Header().Get("Last-Modified") == "" {
		t.Fatalf("missing feed cache validators: %#v", response.Header())
	}

	conditional := publicFeedRequest(http.MethodGet, "/feed.xml")
	conditional.Header.Set("If-None-Match", response.Header().Get("ETag"))
	notModified := httptest.NewRecorder()
	h.PublicFeed(notModified, conditional, true)
	if notModified.Code != http.StatusNotModified || notModified.Body.Len() != 0 {
		t.Fatalf("conditional Atom response = %d %q, want empty 304", notModified.Code, notModified.Body.String())
	}

	head := publicFeedRequest(http.MethodHead, "/rss.xml")
	feed := httptest.NewRecorder()
	h.PublicFeed(feed, head, false)
	if feed.Code != http.StatusOK || feed.Body.Len() != 0 || !strings.Contains(feed.Header().Get("Content-Type"), "application/rss+xml") {
		t.Fatalf("RSS HEAD response = %d, content-type %q, body %q", feed.Code, feed.Header().Get("Content-Type"), feed.Body.String())
	}
}

func TestPublicFeedEmptyIsValid(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	for _, atom := range []bool{true, false} {
		response := httptest.NewRecorder()
		h.PublicFeed(response, publicFeedRequest(http.MethodGet, "/feed.xml"), atom)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "<?xml") {
			t.Fatalf("empty feed atom=%t: status=%d body=%q", atom, response.Code, response.Body.String())
		}
	}
}

func TestPublicFeedRejectsOtherMethods(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	response := httptest.NewRecorder()
	h.PublicFeed(response, publicFeedRequest(http.MethodPost, "/feed.xml"), true)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST feed response = %d Allow=%q, want 405 GET, HEAD", response.Code, response.Header().Get("Allow"))
	}
}

func publicFeedRequest(method, path string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	request.Host = "vidveil.example"
	request.TLS = &tls.ConnectionState{}
	return request
}
