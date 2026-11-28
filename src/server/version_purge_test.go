// SPDX-License-Identifier: MIT
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildStampWriterLimitsCookieAndPurgeToSuccessfulDocuments(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		content    string
		dest       string
		stale      bool
		wantCookie bool
		wantPurge  bool
	}{
		{name: "successful document", status: http.StatusOK, content: "text/html; charset=utf-8", dest: "document", wantCookie: true},
		{name: "redirect", status: http.StatusFound, content: "text/html; charset=utf-8", dest: "document"},
		{name: "error document", status: http.StatusNotFound, content: "text/html; charset=utf-8", dest: "document"},
		{name: "json response", status: http.StatusOK, content: "application/json", dest: "document"},
		{name: "subresource", status: http.StatusOK, content: "text/html", dest: "image"},
		{name: "stale successful document", status: http.StatusOK, content: "text/html", dest: "document", stale: true, wantCookie: true, wantPurge: true},
		{name: "stale error document", status: http.StatusInternalServerError, content: "text/html", dest: "document", stale: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/server/about", nil)
			request.Header.Set("Sec-Fetch-Dest", test.dest)
			request.Header.Set("Sec-Fetch-Mode", "navigate")
			if test.stale {
				request.AddCookie(&http.Cookie{Name: "vidveil_build", Value: "old"})
			}
			recorder := httptest.NewRecorder()
			recorder.Header().Set("Content-Type", test.content)
			writer := &buildStampWriter{ResponseWriter: recorder, request: request, stamp: "new", purge: test.stale}
			writer.WriteHeader(test.status)
			_, _ = writer.Write([]byte("body"))
			response := recorder.Result()
			cookies := response.Cookies()
			gotCookie := false
			for _, cookie := range cookies {
				gotCookie = gotCookie || cookie.Name == "vidveil_build"
			}
			if gotCookie != test.wantCookie {
				t.Errorf("build cookie present = %t, want %t", gotCookie, test.wantCookie)
			}
			gotPurge := strings.Contains(response.Header.Get("Clear-Site-Data"), "storage")
			if gotPurge != test.wantPurge {
				t.Errorf("Clear-Site-Data purge = %t, want %t", gotPurge, test.wantPurge)
			}
		})
	}
}
