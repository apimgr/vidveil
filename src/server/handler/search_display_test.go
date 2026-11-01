// SPDX-License-Identifier: MIT
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/server/model"
)

func TestParseDisplayCountOffset(t *testing.T) {
	tests := []struct {
		query string
		want  int
	}{
		{query: "display_offset=25", want: 25},
		{query: "display_offset=invalid", want: 0},
		{query: "display_offset=-1", want: 0},
		{query: "", want: 0},
	}
	for _, test := range tests {
		req := httptest.NewRequest("GET", "/api/v1/search?"+test.query, nil)
		if got := parseDisplayCountOffset(req); got != test.want {
			t.Errorf("parseDisplayCountOffset(%q) = %d, want %d", test.query, got, test.want)
		}
	}
}

func TestFormatResultCountFormats(t *testing.T) {
	formats := formatResultCountFormats(12847, "en")
	if len(formats) != 12848 {
		t.Fatalf("count format entries = %d, want 12848", len(formats))
	}
	if formats[0] != "0" || formats[12847] != "12,847" {
		t.Errorf("formatted count range endpoints = %q and %q, want 0 and 12,847", formats[0], formats[12847])
	}
}

func TestSearchDisplayKeepsRawSearchTime(t *testing.T) {
	response := model.SearchResponse{Data: model.SearchData{
		SearchTimeMS: 150,
		Display: &model.SearchDisplay{
			SearchTime:      "0.2 seconds",
			ResultCount:     12847,
			ResultCountText: "12,847",
		},
	}}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Data struct {
			SearchTimeMS int64 `json:"search_time_ms"`
			Display      struct {
				SearchTime      string `json:"search_time"`
				ResultCount     int    `json:"result_count"`
				ResultCountText string `json:"result_count_text"`
			} `json:"display"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Data.SearchTimeMS != 150 {
		t.Errorf("search_time_ms = %d, want raw 150 milliseconds", decoded.Data.SearchTimeMS)
	}
	if decoded.Data.Display.SearchTime != "0.2 seconds" {
		t.Errorf("display.search_time = %q, want human-readable duration", decoded.Data.Display.SearchTime)
	}
	if decoded.Data.Display.ResultCount != 12847 || decoded.Data.Display.ResultCountText != "12,847" {
		t.Errorf("display result count = %#v, want raw and localized count", decoded.Data.Display)
	}
}

func TestAPISearchHumanDisplayIsOptInAndKeepsRawFields(t *testing.T) {
	h := newAPITestHandler()
	req := httptest.NewRequest("GET", "/api/v1/search?q=test&display=human", nil)
	req.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	h.APISearch(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("APISearch status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var body struct {
		Data struct {
			SearchTimeMS int64 `json:"search_time_ms"`
			Display      *struct {
				SearchTime      string   `json:"search_time"`
				ResultCount     int      `json:"result_count"`
				ResultCountText string   `json:"result_count_text"`
				ResultFormats   []string `json:"result_count_formats"`
			} `json:"display"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Display == nil {
		t.Fatal("display=human response is missing data.display")
	}
	if body.Data.SearchTimeMS < 0 || body.Data.Display.SearchTime == "" {
		t.Errorf("raw and presentation search times = %d / %q", body.Data.SearchTimeMS, body.Data.Display.SearchTime)
	}
	if body.Data.Display.ResultCount != 0 || body.Data.Display.ResultCountText != "0" || len(body.Data.Display.ResultFormats) != 1 {
		t.Errorf("empty result display metadata = %#v", body.Data.Display)
	}

	plainRequest := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	plainRequest.Header.Set("Accept", "application/json")
	plainResponse := httptest.NewRecorder()
	h.APISearch(plainResponse, plainRequest)
	var plainBody struct {
		Data struct {
			Display json.RawMessage `json:"display"`
		} `json:"data"`
	}
	if err := json.Unmarshal(plainResponse.Body.Bytes(), &plainBody); err != nil {
		t.Fatal(err)
	}
	if len(plainBody.Data.Display) != 0 {
		t.Errorf("ordinary API response unexpectedly includes presentation data: %s", plainBody.Data.Display)
	}
}

func TestAPISearchSSEIncludesLocalizedDisplayDuration(t *testing.T) {
	h := newAPITestHandler()
	req := httptest.NewRequest("GET", "/api/v1/search?q=test&display_offset=7", nil)
	req.Header.Set("Accept", "text/event-stream")
	response := httptest.NewRecorder()
	h.APISearch(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("APISearch SSE status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `"elapsed_ms"`) || !strings.Contains(response.Body.String(), `"elapsed_display"`) {
		t.Errorf("SSE completion must preserve raw time and provide a localized display value: %s", response.Body.String())
	}
}
