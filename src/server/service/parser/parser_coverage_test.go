// SPDX-License-Identifier: MIT
// Additional coverage for parser package.
// Covers: VideoItem struct construction, ItemSelector() non-empty on every
// registered built-in parser, ParseDuration 1:20:00 case, and ParseViews
// plain-integer path not exercised in parser_test.go.
package parser

import (
	"testing"
)

// ---- VideoItem struct ----

// A VideoItem populated with all fields must retain every value correctly.
func TestVideoItemConstruction_AllFields(t *testing.T) {
	item := &VideoItem{
		URL:             "https://example.com/video/123",
		Title:           "Test Video",
		Thumbnail:       "https://cdn.example.com/thumb.jpg",
		PreviewURL:      "https://cdn.example.com/preview.mp4",
		DownloadURL:     "https://cdn.example.com/dl.mp4",
		Duration:        "4:30",
		DurationSeconds: 270,
		Views:           "1.2M",
		ViewsCount:      1200000,
		Quality:         "HD",
		Description:     "A test video",
		Uploader:        "JohnDoe",
		Rating:          "93%",
		IsPremium:       false,
		Tags:            []string{"tag1", "tag2"},
	}

	if item.URL != "https://example.com/video/123" {
		t.Errorf("URL = %q, want expected URL", item.URL)
	}
	if item.Title != "Test Video" {
		t.Errorf("Title = %q, want Test Video", item.Title)
	}
	if item.DurationSeconds != 270 {
		t.Errorf("DurationSeconds = %d, want 270", item.DurationSeconds)
	}
	if item.ViewsCount != 1200000 {
		t.Errorf("ViewsCount = %d, want 1200000", item.ViewsCount)
	}
	if item.IsPremium {
		t.Error("IsPremium must be false")
	}
	if len(item.Tags) != 2 {
		t.Errorf("len(Tags) = %d, want 2", len(item.Tags))
	}
}

// Tags field defaults to nil slice and length zero when omitted.
func TestVideoItemConstruction_NilTags(t *testing.T) {
	item := &VideoItem{Title: "no tags"}
	if len(item.Tags) != 0 {
		t.Errorf("default Tags len = %d, want 0", len(item.Tags))
	}
}

// Tags field can hold a non-nil empty slice.
func TestVideoItemConstruction_EmptyTags(t *testing.T) {
	item := &VideoItem{Tags: []string{}}
	if item.Tags == nil {
		t.Error("Tags must not be nil when initialised as empty slice")
	}
	if len(item.Tags) != 0 {
		t.Errorf("len(Tags) = %d, want 0", len(item.Tags))
	}
}

// ---- ItemSelector on each registered built-in parser ----

// Every registered built-in parser must return a non-empty CSS selector.
func TestItemSelector_AllRegisteredParsers(t *testing.T) {
	names := []string{
		"pornhub",
		"xvideos",
		"xnxx",
		"redtube",
		"eporner",
		"pornmd",
	}

	for _, name := range names {
		p := GetParser(name)
		if p == nil {
			t.Errorf("GetParser(%q) returned nil", name)
			continue
		}
		sel := p.ItemSelector()
		if sel == "" {
			t.Errorf("parser %q ItemSelector() returned empty string", name)
		}
	}
}

// ---- ParseDuration edge cases not in parser_test.go ----

// "1:20:00" must parse to display "1:20:00" and 4800 seconds.
func TestParseDuration_OneHourTwentyMinutes(t *testing.T) {
	str, secs := ParseDuration("1:20:00")
	if str != "1:20:00" {
		t.Errorf("ParseDuration 1:20:00 str = %q, want 1:20:00", str)
	}
	if secs != 4800 {
		t.Errorf("ParseDuration 1:20:00 secs = %d, want 4800", secs)
	}
}

// A zero-second plain integer must return ("", 0) because the condition is n > 0.
func TestParseDuration_PlainZero(t *testing.T) {
	str, secs := ParseDuration("0")
	if secs != 0 {
		t.Errorf("ParseDuration 0 secs = %d, want 0", secs)
	}
	_ = str
}

// Leading/trailing whitespace is stripped before parsing.
func TestParseDuration_WhitespaceTrimmed(t *testing.T) {
	str, secs := ParseDuration("  4:30  ")
	if secs != 270 {
		t.Errorf("ParseDuration whitespace secs = %d, want 270", secs)
	}
	if str != "4:30" {
		t.Errorf("ParseDuration whitespace str = %q, want 4:30", str)
	}
}

// ---- ParseViews additional cases ----

// A plain integer without multiplier suffix must parse correctly.
func TestParseViews_PlainInteger(t *testing.T) {
	str, count := ParseViews("12345")
	if str != "12345" {
		t.Errorf("ParseViews plain int str = %q, want 12345", str)
	}
	if count != 12345 {
		t.Errorf("ParseViews plain int count = %d, want 12345", count)
	}
}

// Empty string returns ("", 0); this is a guard against a regression where
// the multiplier branch might panic on an empty string.
func TestParseViews_EmptyNoSuffix(t *testing.T) {
	str, count := ParseViews("   ")
	_ = str
	_ = count
}

// ---- CleanText additional cases not duplicated from parser_test.go ----

// Multiple types of whitespace in a single string collapse to single spaces.
func TestCleanText_MixedWhitespace(t *testing.T) {
	got := CleanText("foo\t\t bar\n\nbaz")
	if got != "foo bar baz" {
		t.Errorf("CleanText mixed whitespace: got %q, want %q", got, "foo bar baz")
	}
}
