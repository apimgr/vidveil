// SPDX-License-Identifier: MIT
// AI.md PART 14: deterministic coverage for createHTTPClient, including its
// CheckRedirect closure (redirect cap and header-preservation branches). No
// network is required; the closure is invoked directly with synthetic requests.
package engine

import (
	"net/http"
	"testing"
)

func TestCreateHTTPClient(t *testing.T) {
	c := createHTTPClient(5)
	if c == nil {
		t.Fatal("createHTTPClient returned nil")
	}
	if c.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
	if c.Jar == nil {
		t.Error("expected a cookie jar")
	}
	if c.CheckRedirect == nil {
		t.Fatal("expected a CheckRedirect func")
	}

	// Header-preservation branch: fewer than 10 hops, first request carries a
	// header that must be copied onto the follow-up request.
	first, _ := http.NewRequest(http.MethodGet, "http://a/", nil)
	first.Header.Set("X-Keep", "yes")
	next, _ := http.NewRequest(http.MethodGet, "http://b/", nil)
	if err := c.CheckRedirect(next, []*http.Request{first}); err != nil {
		t.Fatalf("CheckRedirect returned error under the cap: %v", err)
	}
	if next.Header.Get("X-Keep") != "yes" {
		t.Errorf("header not preserved across redirect: %q", next.Header.Get("X-Keep"))
	}

	// Redirect-cap branch: 10 prior hops must error.
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = first
	}
	if err := c.CheckRedirect(next, via); err == nil {
		t.Error("expected an error when redirect cap is reached")
	}
}
