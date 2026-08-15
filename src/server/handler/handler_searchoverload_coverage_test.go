// SPDX-License-Identifier: MIT
// AI.md PART 12 "Rate Limiting": coverage tests for isSearchOverloaded and
// writeSearchOverloadJSON, the shared helpers every SearchWithOperators
// caller (SearchPage, APISearch, SearchRSSFeed, SearchAtomFeed) uses to turn
// the EngineManager searchSem overload envelope into a real HTTP 429 +
// Retry-After response instead of silently answering 200.
package handler

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/server/model"
)

// ── isSearchOverloaded ──────────────────────────────────────────────────────

func TestIsSearchOverloaded_NilResults_ReturnsFalse(t *testing.T) {
	if isSearchOverloaded(nil) {
		t.Error("isSearchOverloaded(nil) = true, want false")
	}
}

func TestIsSearchOverloaded_NormalOkResponse_ReturnsFalse(t *testing.T) {
	resp := &model.SearchResponse{Ok: true}
	if isSearchOverloaded(resp) {
		t.Error("isSearchOverloaded on Ok:true response = true, want false")
	}
}

func TestIsSearchOverloaded_OtherErrorCode_ReturnsFalse(t *testing.T) {
	resp := &model.SearchResponse{Ok: false, Error: "BAD_REQUEST"}
	if isSearchOverloaded(resp) {
		t.Error("isSearchOverloaded on Error=BAD_REQUEST = true, want false")
	}
}

func TestIsSearchOverloaded_RateLimitedEnvelope_ReturnsTrue(t *testing.T) {
	resp := &model.SearchResponse{Ok: false, Error: CodeRateLimited}
	if !isSearchOverloaded(resp) {
		t.Error("isSearchOverloaded on Ok:false, Error:RATE_LIMITED = false, want true")
	}
}

// ── writeSearchOverloadJSON ─────────────────────────────────────────────────

func TestWriteSearchOverloadJSON_SetsStatusAndRetryAfterHeader(t *testing.T) {
	h := newAPITestHandler()
	w := httptest.NewRecorder()
	h.writeSearchOverloadJSON(w)

	if w.Code != 429 {
		t.Errorf("writeSearchOverloadJSON status = %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != strconv.Itoa(searchOverloadRetryAfterSeconds) {
		t.Errorf("writeSearchOverloadJSON Retry-After = %q, want %q", got, strconv.Itoa(searchOverloadRetryAfterSeconds))
	}
	body := w.Body.String()
	if !strings.Contains(body, `"ok": false`) || !strings.Contains(body, CodeRateLimited) {
		t.Errorf("writeSearchOverloadJSON body missing canonical envelope fields: %s", body)
	}
}
