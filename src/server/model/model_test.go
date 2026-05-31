// SPDX-License-Identifier: MIT
// Tests for the model package: struct construction and JSON round-trips for all
// exported types in result.go. Also verifies JSON tag names and omitempty behaviour.
package model

import (
	"encoding/json"
	"testing"
	"time"
)

// ---- VideoResult ----

func TestVideoResultConstruction(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	v := VideoResult{
		ID:              "abc123",
		Title:           "Test Video",
		URL:             "https://example.com/video",
		Thumbnail:       "https://example.com/thumb.jpg",
		PreviewURL:      "https://example.com/preview.mp4",
		DownloadURL:     "https://example.com/download.mp4",
		Duration:        "10:30",
		DurationSeconds: 630,
		Views:           "1.2M",
		ViewsCount:      1200000,
		Rating:          4.5,
		Quality:         "1080p",
		Source:          "testsource",
		SourceDisplay:   "Test Source",
		Published:       now,
		Description:     "A test video",
		Tags:            []string{"tag1", "tag2"},
		Performer:       "Test Performer",
	}

	if v.ID != "abc123" {
		t.Errorf("ID = %q, want %q", v.ID, "abc123")
	}
	if v.DurationSeconds != 630 {
		t.Errorf("DurationSeconds = %d, want 630", v.DurationSeconds)
	}
	if v.ViewsCount != 1200000 {
		t.Errorf("ViewsCount = %d, want 1200000", v.ViewsCount)
	}
	if len(v.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(v.Tags))
	}
}

func TestVideoResultJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := VideoResult{
		ID:              "vid-001",
		Title:           "Round-trip Video",
		URL:             "https://example.com/v/001",
		Thumbnail:       "https://example.com/t/001.jpg",
		Duration:        "5:00",
		DurationSeconds: 300,
		Views:           "500K",
		ViewsCount:      500000,
		Source:          "source1",
		SourceDisplay:   "Source One",
		Published:       now,
		Tags:            []string{"a", "b", "c"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal VideoResult error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("json.Marshal VideoResult returned empty bytes")
	}

	var decoded VideoResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal VideoResult error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title: got %q, want %q", decoded.Title, original.Title)
	}
	if decoded.DurationSeconds != original.DurationSeconds {
		t.Errorf("DurationSeconds: got %d, want %d", decoded.DurationSeconds, original.DurationSeconds)
	}
	if decoded.ViewsCount != original.ViewsCount {
		t.Errorf("ViewsCount: got %d, want %d", decoded.ViewsCount, original.ViewsCount)
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Errorf("Tags length: got %d, want %d", len(decoded.Tags), len(original.Tags))
	}
}

func TestVideoResultOmitsZeroRating(t *testing.T) {
	v := VideoResult{
		ID:     "novid",
		Source: "src",
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// rating is omitempty — must be absent when zero
	if _, exists := m["rating"]; exists {
		t.Error("rating field should be omitted when zero")
	}
	// preview_url is omitempty — must be absent when empty
	if _, exists := m["preview_url"]; exists {
		t.Error("preview_url field should be omitted when empty")
	}
	// download_url is omitempty — must be absent when empty
	if _, exists := m["download_url"]; exists {
		t.Error("download_url field should be omitted when empty")
	}
}

// ---- SearchResponse ----

func TestSearchResponseConstruction(t *testing.T) {
	sr := SearchResponse{
		Ok:      true,
		Error:   "",
		Message: "",
		Data: SearchData{
			Query:        "test query",
			Results:      []VideoResult{{ID: "1", Source: "s"}},
			EnginesUsed:  []string{"eng1"},
			SearchTimeMS: 150,
		},
		Pagination: PaginationData{
			Page:  1,
			Limit: 20,
			Total: 1,
			Pages: 1,
		},
	}

	if !sr.Ok {
		t.Error("Ok should be true")
	}
	if len(sr.Data.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(sr.Data.Results))
	}
}

func TestSearchResponseJSONRoundTrip(t *testing.T) {
	original := SearchResponse{
		Ok: true,
		Data: SearchData{
			Query:        "hello",
			Results:      []VideoResult{{ID: "r1", Title: "Result 1", Source: "src"}},
			EnginesUsed:  []string{"eng_a"},
			EnginesFailed: []string{},
			SearchTimeMS: 42,
		},
		Pagination: PaginationData{Page: 2, Limit: 10, Total: 25, Pages: 3},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal SearchResponse error: %v", err)
	}

	var decoded SearchResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal SearchResponse error: %v", err)
	}

	if decoded.Ok != original.Ok {
		t.Errorf("Ok: got %v, want %v", decoded.Ok, original.Ok)
	}
	if decoded.Data.Query != original.Data.Query {
		t.Errorf("Data.Query: got %q, want %q", decoded.Data.Query, original.Data.Query)
	}
	if decoded.Pagination.Page != original.Pagination.Page {
		t.Errorf("Pagination.Page: got %d, want %d", decoded.Pagination.Page, original.Pagination.Page)
	}
	if decoded.Pagination.Pages != original.Pagination.Pages {
		t.Errorf("Pagination.Pages: got %d, want %d", decoded.Pagination.Pages, original.Pagination.Pages)
	}
}

func TestSearchResponseOkFieldSerializesLowercase(t *testing.T) {
	sr := SearchResponse{Ok: true}
	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, exists := m["ok"]; !exists {
		t.Error("SearchResponse Ok field must serialize as \"ok\" (lowercase)")
	}
	if _, exists := m["Ok"]; exists {
		t.Error("SearchResponse must not serialize field as \"Ok\" (capitalized)")
	}
}

func TestSearchResponseErrorOmittedWhenEmpty(t *testing.T) {
	sr := SearchResponse{Ok: true}
	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// error and message are omitempty — absent when empty
	if _, exists := m["error"]; exists {
		t.Error("error field should be omitted when empty")
	}
	if _, exists := m["message"]; exists {
		t.Error("message field should be omitted when empty")
	}
}

// ---- SearchData ----

func TestSearchDataJSONRoundTrip(t *testing.T) {
	original := SearchData{
		Query:           "cats",
		SearchQuery:     "cats videos",
		Results:         []VideoResult{{ID: "cat1", Title: "Cats", Source: "src"}},
		EnginesUsed:     []string{"engine1", "engine2"},
		EnginesFailed:   []string{"engine3"},
		SearchTimeMS:    88,
		HasBang:         true,
		BangEngines:     []string{"engine1"},
		Cached:          true,
		RelatedSearches: []string{"kittens", "cute cats"},
		SpellSuggestion: "cat",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal SearchData error: %v", err)
	}

	var decoded SearchData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal SearchData error: %v", err)
	}

	if decoded.Query != original.Query {
		t.Errorf("Query: got %q, want %q", decoded.Query, original.Query)
	}
	if decoded.SearchTimeMS != original.SearchTimeMS {
		t.Errorf("SearchTimeMS: got %d, want %d", decoded.SearchTimeMS, original.SearchTimeMS)
	}
	if !decoded.HasBang {
		t.Error("HasBang: got false, want true")
	}
	if !decoded.Cached {
		t.Error("Cached: got false, want true")
	}
	if decoded.SpellSuggestion != original.SpellSuggestion {
		t.Errorf("SpellSuggestion: got %q, want %q", decoded.SpellSuggestion, original.SpellSuggestion)
	}
	if len(decoded.Results) != 1 {
		t.Errorf("Results length: got %d, want 1", len(decoded.Results))
	}
	if len(decoded.RelatedSearches) != 2 {
		t.Errorf("RelatedSearches length: got %d, want 2", len(decoded.RelatedSearches))
	}
}

func TestSearchDataOmitsOptionalFieldsWhenZero(t *testing.T) {
	sd := SearchData{
		Query:        "minimal",
		EnginesUsed:  []string{"e1"},
		Results:      []VideoResult{},
		SearchTimeMS: 10,
	}
	data, err := json.Marshal(sd)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, field := range []string{"has_bang", "bang_engines", "cached", "related_searches", "spell_suggestion", "search_query"} {
		if _, exists := m[field]; exists {
			t.Errorf("field %q should be omitted when zero/empty", field)
		}
	}
}

// ---- PaginationData ----

func TestPaginationDataJSONRoundTrip(t *testing.T) {
	original := PaginationData{Page: 3, Limit: 25, Total: 100, Pages: 4}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal PaginationData error: %v", err)
	}

	var decoded PaginationData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal PaginationData error: %v", err)
	}

	if decoded.Page != original.Page {
		t.Errorf("Page: got %d, want %d", decoded.Page, original.Page)
	}
	if decoded.Limit != original.Limit {
		t.Errorf("Limit: got %d, want %d", decoded.Limit, original.Limit)
	}
	if decoded.Total != original.Total {
		t.Errorf("Total: got %d, want %d", decoded.Total, original.Total)
	}
	if decoded.Pages != original.Pages {
		t.Errorf("Pages: got %d, want %d", decoded.Pages, original.Pages)
	}
}

func TestPaginationDataFieldNames(t *testing.T) {
	p := PaginationData{Page: 1, Limit: 10, Total: 5, Pages: 1}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{"page", "limit", "total", "pages"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q", key)
		}
	}
}

// ---- EngineInfo ----

func TestEngineInfoConstruction(t *testing.T) {
	caps := &EngineCapabilities{HasPreview: true, HasDownload: false}
	ei := EngineInfo{
		Name:        "testengine",
		DisplayName: "Test Engine",
		Enabled:     true,
		Available:   true,
		Features:    []string{"search", "preview"},
		Tier:        1,
		Capabilities: caps,
		Privacy: EnginePrivacyScore{
			RequiresJS:  false,
			SetsCookies: true,
			HasTracking: false,
		},
	}

	if ei.Name != "testengine" {
		t.Errorf("Name = %q, want %q", ei.Name, "testengine")
	}
	if !ei.Enabled {
		t.Error("Enabled should be true")
	}
	if ei.Capabilities == nil {
		t.Error("Capabilities should not be nil")
	}
	if !ei.Capabilities.HasPreview {
		t.Error("Capabilities.HasPreview should be true")
	}
}

func TestEngineInfoJSONRoundTrip(t *testing.T) {
	caps := &EngineCapabilities{HasPreview: true, HasDownload: true}
	original := EngineInfo{
		Name:        "eng1",
		DisplayName: "Engine One",
		Enabled:     true,
		Available:   false,
		Features:    []string{"f1"},
		Tier:        2,
		Capabilities: caps,
		Privacy: EnginePrivacyScore{RequiresJS: true, SetsCookies: false, HasTracking: true},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal EngineInfo error: %v", err)
	}

	var decoded EngineInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal EngineInfo error: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Tier != original.Tier {
		t.Errorf("Tier: got %d, want %d", decoded.Tier, original.Tier)
	}
	if decoded.Capabilities == nil {
		t.Fatal("Capabilities: got nil, want non-nil")
	}
	if decoded.Capabilities.HasPreview != caps.HasPreview {
		t.Errorf("Capabilities.HasPreview: got %v, want %v", decoded.Capabilities.HasPreview, caps.HasPreview)
	}
	if decoded.Privacy.RequiresJS != original.Privacy.RequiresJS {
		t.Errorf("Privacy.RequiresJS: got %v, want %v", decoded.Privacy.RequiresJS, original.Privacy.RequiresJS)
	}
}

func TestEngineInfoCapabilitiesOmittedWhenNil(t *testing.T) {
	ei := EngineInfo{Name: "nocaps", DisplayName: "No Caps"}
	data, err := json.Marshal(ei)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// capabilities is omitempty — absent when nil
	if _, exists := m["capabilities"]; exists {
		t.Error("capabilities field should be omitted when nil")
	}
}

// ---- EngineHealthStats ----

func TestEngineHealthStatsConstruction(t *testing.T) {
	now := time.Now().UTC()
	hs := EngineHealthStats{
		CircuitState:     "closed",
		CircuitFailures:  0,
		LastFailureAt:    time.Time{},
		TotalSuccesses:   100,
		TotalFailures:    2,
		LastSuccessAt:    now,
		AvgLatencyMs:     42,
		UptimePct:        98.0,
		RateLimitedUntil: time.Time{},
		IsRateLimited:    false,
	}

	if hs.CircuitState != "closed" {
		t.Errorf("CircuitState = %q, want %q", hs.CircuitState, "closed")
	}
	if hs.TotalSuccesses != 100 {
		t.Errorf("TotalSuccesses = %d, want 100", hs.TotalSuccesses)
	}
	if hs.UptimePct != 98.0 {
		t.Errorf("UptimePct = %f, want 98.0", hs.UptimePct)
	}
}

func TestEngineHealthStatsJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := EngineHealthStats{
		CircuitState:    "open",
		CircuitFailures: 5,
		LastFailureAt:   now,
		TotalSuccesses:  50,
		TotalFailures:   10,
		LastSuccessAt:   now.Add(-1 * time.Minute),
		AvgLatencyMs:    200,
		UptimePct:       83.3,
		IsRateLimited:   true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal EngineHealthStats error: %v", err)
	}

	var decoded EngineHealthStats
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal EngineHealthStats error: %v", err)
	}

	if decoded.CircuitState != original.CircuitState {
		t.Errorf("CircuitState: got %q, want %q", decoded.CircuitState, original.CircuitState)
	}
	if decoded.CircuitFailures != original.CircuitFailures {
		t.Errorf("CircuitFailures: got %d, want %d", decoded.CircuitFailures, original.CircuitFailures)
	}
	if decoded.TotalSuccesses != original.TotalSuccesses {
		t.Errorf("TotalSuccesses: got %d, want %d", decoded.TotalSuccesses, original.TotalSuccesses)
	}
	if decoded.AvgLatencyMs != original.AvgLatencyMs {
		t.Errorf("AvgLatencyMs: got %d, want %d", decoded.AvgLatencyMs, original.AvgLatencyMs)
	}
	if decoded.IsRateLimited != original.IsRateLimited {
		t.Errorf("IsRateLimited: got %v, want %v", decoded.IsRateLimited, original.IsRateLimited)
	}
}

// ---- EngineStatInfo ----

func TestEngineStatInfoConstruction(t *testing.T) {
	es := EngineStatInfo{
		ResponseTimeMS: 350,
		ResultCount:    15,
		Error:          "",
	}

	if es.ResponseTimeMS != 350 {
		t.Errorf("ResponseTimeMS = %d, want 350", es.ResponseTimeMS)
	}
	if es.ResultCount != 15 {
		t.Errorf("ResultCount = %d, want 15", es.ResultCount)
	}
}

func TestEngineStatInfoJSONRoundTrip(t *testing.T) {
	original := EngineStatInfo{
		ResponseTimeMS: 500,
		ResultCount:    8,
		Error:          "timeout",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal EngineStatInfo error: %v", err)
	}

	var decoded EngineStatInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal EngineStatInfo error: %v", err)
	}

	if decoded.ResponseTimeMS != original.ResponseTimeMS {
		t.Errorf("ResponseTimeMS: got %d, want %d", decoded.ResponseTimeMS, original.ResponseTimeMS)
	}
	if decoded.ResultCount != original.ResultCount {
		t.Errorf("ResultCount: got %d, want %d", decoded.ResultCount, original.ResultCount)
	}
	if decoded.Error != original.Error {
		t.Errorf("Error: got %q, want %q", decoded.Error, original.Error)
	}
}

func TestEngineStatInfoErrorOmittedWhenEmpty(t *testing.T) {
	es := EngineStatInfo{ResponseTimeMS: 100, ResultCount: 5}
	data, err := json.Marshal(es)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	// error is omitempty — absent when empty string
	if _, exists := m["error"]; exists {
		t.Error("error field should be omitted when empty")
	}
}

// ---- EnginesResponse ----

func TestEnginesResponseJSONRoundTrip(t *testing.T) {
	original := EnginesResponse{
		Ok: true,
		Data: []EngineInfo{
			{Name: "e1", DisplayName: "Engine 1", Enabled: true, Available: true},
			{Name: "e2", DisplayName: "Engine 2", Enabled: false, Available: false},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal EnginesResponse error: %v", err)
	}

	var decoded EnginesResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal EnginesResponse error: %v", err)
	}

	if decoded.Ok != original.Ok {
		t.Errorf("Ok: got %v, want %v", decoded.Ok, original.Ok)
	}
	if len(decoded.Data) != len(original.Data) {
		t.Errorf("Data length: got %d, want %d", len(decoded.Data), len(original.Data))
	}
	if decoded.Data[0].Name != "e1" {
		t.Errorf("Data[0].Name: got %q, want %q", decoded.Data[0].Name, "e1")
	}
}

// ---- EngineHealthInfo ----

func TestEngineHealthInfoEmbedsBothTypes(t *testing.T) {
	now := time.Now().UTC()
	original := EngineHealthInfo{
		EngineInfo: EngineInfo{
			Name:        "healthy_engine",
			DisplayName: "Healthy Engine",
			Enabled:     true,
			Available:   true,
		},
		Health: EngineHealthStats{
			CircuitState:   "closed",
			TotalSuccesses: 1000,
			LastSuccessAt:  now,
			UptimePct:      99.9,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal EngineHealthInfo error: %v", err)
	}

	var decoded EngineHealthInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal EngineHealthInfo error: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Health.CircuitState != original.Health.CircuitState {
		t.Errorf("Health.CircuitState: got %q, want %q", decoded.Health.CircuitState, original.Health.CircuitState)
	}
	if decoded.Health.TotalSuccesses != original.Health.TotalSuccesses {
		t.Errorf("Health.TotalSuccesses: got %d, want %d", decoded.Health.TotalSuccesses, original.Health.TotalSuccesses)
	}
}
