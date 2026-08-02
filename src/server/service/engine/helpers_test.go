// SPDX-License-Identifier: MIT
// AI.md PART 28: Test coverage for the generic-search field extraction
// helpers - JSON-LD VideoObject parsing, date/duration parsing, and merging
// best-effort fields into a model.VideoResult without clobbering data
// already extracted from the HTML card.
package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
)

func TestParseISO8601Duration(t *testing.T) {
	cases := map[string]int{
		"":         0,
		"garbage":  0,
		"PT5M":     300,
		"PT1H2M3S": 3723,
		"PT30S":    30,
		"PT2H":     7200,
		" PT1M ":   60,
	}
	for input, want := range cases {
		if got := parseISO8601Duration(input); got != want {
			t.Errorf("parseISO8601Duration(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParsePublishedDate(t *testing.T) {
	cases := []string{
		"2024-01-02T15:04:05Z",
		"2024-01-02",
		"01/02/2024",
		"January 2, 2024",
	}
	for _, input := range cases {
		if got := parsePublishedDate(input); got.IsZero() {
			t.Errorf("parsePublishedDate(%q) = zero time, want a parsed date", input)
		}
	}
	if got := parsePublishedDate(""); !got.IsZero() {
		t.Errorf("parsePublishedDate(\"\") = %v, want zero time", got)
	}
	if got := parsePublishedDate("not a date"); !got.IsZero() {
		t.Errorf("parsePublishedDate(garbage) = %v, want zero time", got)
	}
}

func TestDecodeLDNodes_BareObject(t *testing.T) {
	nodes := decodeLDNodes([]byte(`{"@type":"VideoObject","name":"x"}`))
	if len(nodes) != 1 {
		t.Fatalf("decodeLDNodes bare object = %d nodes, want 1", len(nodes))
	}
}

func TestDecodeLDNodes_Array(t *testing.T) {
	nodes := decodeLDNodes([]byte(`[{"@type":"VideoObject"},{"@type":"VideoObject"}]`))
	if len(nodes) != 2 {
		t.Fatalf("decodeLDNodes array = %d nodes, want 2", len(nodes))
	}
}

func TestDecodeLDNodes_Graph(t *testing.T) {
	nodes := decodeLDNodes([]byte(`{"@graph":[{"@type":"VideoObject"},{"@type":"WebPage"}]}`))
	if len(nodes) != 2 {
		t.Fatalf("decodeLDNodes @graph = %d nodes, want 2", len(nodes))
	}
}

func TestDecodeLDNodes_Malformed(t *testing.T) {
	if nodes := decodeLDNodes([]byte(`not json`)); nodes != nil {
		t.Errorf("decodeLDNodes(malformed) = %v, want nil", nodes)
	}
}

func TestLdFloat(t *testing.T) {
	if got := ldFloat(4.5); got != 4.5 {
		t.Errorf("ldFloat(4.5) = %v, want 4.5", got)
	}
	if got := ldFloat("4.5"); got != 4.5 {
		t.Errorf("ldFloat(\"4.5\") = %v, want 4.5", got)
	}
	if got := ldFloat("not a number"); got != 0 {
		t.Errorf("ldFloat(garbage) = %v, want 0", got)
	}
	if got := ldFloat(nil); got != 0 {
		t.Errorf("ldFloat(nil) = %v, want 0", got)
	}
}

func TestLdActorName(t *testing.T) {
	if got := ldActorName("Plain Name"); got != "Plain Name" {
		t.Errorf("ldActorName(string) = %q, want %q", got, "Plain Name")
	}
	if got := ldActorName(map[string]interface{}{"name": "Person Object"}); got != "Person Object" {
		t.Errorf("ldActorName(object) = %q, want %q", got, "Person Object")
	}
	list := []interface{}{map[string]interface{}{"name": "First Actor"}}
	if got := ldActorName(list); got != "First Actor" {
		t.Errorf("ldActorName(array) = %q, want %q", got, "First Actor")
	}
	if got := ldActorName(nil); got != "" {
		t.Errorf("ldActorName(nil) = %q, want empty", got)
	}
}

func TestLdStringList(t *testing.T) {
	got := ldStringList("Tag One, Tag Two")
	if len(got) != 2 || got[0] != "tag one" || got[1] != "tag two" {
		t.Errorf("ldStringList(csv) = %v, want [tag one, tag two]", got)
	}
	got = ldStringList([]interface{}{"Alpha", "Beta"})
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("ldStringList(array) = %v, want [alpha, beta]", got)
	}
	if got := ldStringList(nil); len(got) != 0 {
		t.Errorf("ldStringList(nil) = %v, want empty", got)
	}
}

func TestLdInteractionCount(t *testing.T) {
	if got := ldInteractionCount(float64(1234)); got != 1234 {
		t.Errorf("ldInteractionCount(number) = %d, want 1234", got)
	}
	if got := ldInteractionCount("https://schema.org/WatchAction/5678"); got != 5678 {
		t.Errorf("ldInteractionCount(urn string) = %d, want 5678", got)
	}
	nested := map[string]interface{}{"userInteractionCount": float64(999)}
	if got := ldInteractionCount(nested); got != 999 {
		t.Errorf("ldInteractionCount(nested) = %d, want 999", got)
	}
	if got := ldInteractionCount(nil); got != 0 {
		t.Errorf("ldInteractionCount(nil) = %d, want 0", got)
	}
}

func TestMergeLDVideoInfo_FillsEmptyFieldsOnly(t *testing.T) {
	published := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	ld := ldVideoInfo{
		description: "ld description",
		thumbnail:   "https://example.com/ld-thumb.jpg",
		uploadDate:  published,
		duration:    120,
		tags:        []string{"tag-a", "tag-b"},
		performer:   "LD Performer",
		rating:      4.2,
		viewCount:   500,
	}

	r := model.VideoResult{
		// Title/URL/Thumbnail already extracted from the HTML card - the
		// thumbnail must NOT be overwritten by the JSON-LD value.
		Title:     "Card Title",
		URL:       "https://example.com/v/1",
		Thumbnail: "https://example.com/card-thumb.jpg",
	}
	mergeLDVideoInfo(&r, ld)

	if r.Thumbnail != "https://example.com/card-thumb.jpg" {
		t.Errorf("mergeLDVideoInfo overwrote an already-set Thumbnail: got %q", r.Thumbnail)
	}
	if r.Description != "ld description" {
		t.Errorf("mergeLDVideoInfo Description = %q, want %q", r.Description, "ld description")
	}
	if !r.Published.Equal(published) {
		t.Errorf("mergeLDVideoInfo Published = %v, want %v", r.Published, published)
	}
	if r.DurationSeconds != 120 {
		t.Errorf("mergeLDVideoInfo DurationSeconds = %d, want 120", r.DurationSeconds)
	}
	if len(r.Tags) != 2 {
		t.Errorf("mergeLDVideoInfo Tags = %v, want 2 tags", r.Tags)
	}
	if r.Performer != "LD Performer" {
		t.Errorf("mergeLDVideoInfo Performer = %q, want %q", r.Performer, "LD Performer")
	}
	if r.Rating != 4.2 {
		t.Errorf("mergeLDVideoInfo Rating = %v, want 4.2", r.Rating)
	}
	if r.ViewsCount != 500 {
		t.Errorf("mergeLDVideoInfo ViewsCount = %d, want 500", r.ViewsCount)
	}
}

func TestMergeLDVideoInfo_NeverOverwritesExistingValue(t *testing.T) {
	r := model.VideoResult{
		Performer:  "HTML Performer",
		Rating:     3.0,
		Tags:       []string{"already-set"},
		ViewsCount: 42,
	}
	mergeLDVideoInfo(&r, ldVideoInfo{
		performer: "LD Performer",
		rating:    9.9,
		tags:      []string{"ld-tag"},
		viewCount: 999999,
	})

	if r.Performer != "HTML Performer" {
		t.Errorf("mergeLDVideoInfo clobbered Performer: got %q", r.Performer)
	}
	if r.Rating != 3.0 {
		t.Errorf("mergeLDVideoInfo clobbered Rating: got %v", r.Rating)
	}
	if len(r.Tags) != 1 || r.Tags[0] != "already-set" {
		t.Errorf("mergeLDVideoInfo clobbered Tags: got %v", r.Tags)
	}
	if r.ViewsCount != 42 {
		t.Errorf("mergeLDVideoInfo clobbered ViewsCount: got %d", r.ViewsCount)
	}
}

// genericJSONLDHTML mimics a video-card listing page that emits a
// schema.org VideoObject JSON-LD block alongside a minimal HTML card, the
// way many real sites decorate their search-result markup.
const genericJSONLDHTML = `<!DOCTYPE html><html><head>
<script type="application/ld+json">
{"@type":"VideoObject","name":"LD Video","contentUrl":"/v/ld-1","thumbnailUrl":"https://example.com/ld.jpg",
"uploadDate":"2024-05-01T00:00:00Z","duration":"PT2M5S","keywords":"alpha, beta",
"actor":{"@type":"Person","name":"JSON-LD Actor"},
"aggregateRating":{"ratingValue":4.7},"interactionCount":"https://schema.org/WatchAction/8000"}
</script>
</head><body>
<div class="item"><a href="/v/ld-1" title="LD Video"><img src="/thumb-card.jpg"></a></div>
</body></html>`

func TestGenericSearch_MergesJSONLDIntoCard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(genericJSONLDHTML))
	}))
	t.Cleanup(srv.Close)

	e := NewBaseEngine("test-jsonld", "Test JSON-LD", srv.URL, 1, config.DefaultAppConfig())
	results, err := genericSearch(context.Background(), e, srv.URL, ".item")
	if err != nil {
		t.Fatalf("genericSearch error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("genericSearch results = %d, want 1", len(results))
	}

	r := results[0]
	if r.Thumbnail != srv.URL+"/thumb-card.jpg" {
		t.Errorf("genericSearch Thumbnail should keep HTML card value, got %q", r.Thumbnail)
	}
	if r.DurationSeconds != 125 {
		t.Errorf("genericSearch DurationSeconds = %d, want 125 (from JSON-LD PT2M5S)", r.DurationSeconds)
	}
	if r.Performer != "JSON-LD Actor" {
		t.Errorf("genericSearch Performer = %q, want %q", r.Performer, "JSON-LD Actor")
	}
	if len(r.Tags) != 2 {
		t.Errorf("genericSearch Tags = %v, want [alpha beta]", r.Tags)
	}
	if r.Rating != 4.7 {
		t.Errorf("genericSearch Rating = %v, want 4.7", r.Rating)
	}
	if r.ViewsCount != 8000 {
		t.Errorf("genericSearch ViewsCount = %d, want 8000", r.ViewsCount)
	}
	if r.Published.IsZero() {
		t.Error("genericSearch Published should be set from JSON-LD uploadDate")
	}
}

// detailEnrichmentHandler serves a tiny listing page at "/" (two cards, no
// JSON-LD) and per-video detail pages under "/v/" carrying a VideoObject
// JSON-LD block keyed to that same URL - the shape enrichDetailResults is
// meant to backfill from.
func detailEnrichmentHandler(fetchCount *int32, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`<html><body>
<div class="item"><a href="/v/1" title="Video One"><img src="/thumb1.jpg"></a></div>
<div class="item"><a href="/v/2" title="Video Two"><img src="/thumb2.jpg"></a></div>
</body></html>`))
			return
		}
		mu.Lock()
		*fetchCount++
		mu.Unlock()
		id := strings.TrimPrefix(r.URL.Path, "/v/")
		_, _ = w.Write([]byte(`<html><head><script type="application/ld+json">
{"@type":"VideoObject","name":"Detail ` + id + `","contentUrl":"` + r.URL.Path + `",
"actor":{"@type":"Person","name":"Detail Actor ` + id + `"},"duration":"PT1M40S"}
</script></head><body></body></html>`))
	}
}

func TestEnrichDetailResults_DisabledByDefault(t *testing.T) {
	var fetches int32
	var mu sync.Mutex
	srv := httptest.NewServer(detailEnrichmentHandler(&fetches, &mu))
	t.Cleanup(srv.Close)

	e := NewBaseEngine("test-detail-off", "Test Detail Off", srv.URL, 1, config.DefaultAppConfig())
	results, err := genericSearch(context.Background(), e, srv.URL, ".item")
	if err != nil {
		t.Fatalf("genericSearch error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("genericSearch results = %d, want 2", len(results))
	}
	if results[0].Performer != "" {
		t.Errorf("Performer should stay empty with DetailEnrichment disabled, got %q", results[0].Performer)
	}
	if fetches != 0 {
		t.Errorf("detail pages fetched = %d, want 0 (enrichment disabled)", fetches)
	}
}

func TestEnrichDetailResults_MergesFromDetailPageAndRespectsMaxResults(t *testing.T) {
	var fetches int32
	var mu sync.Mutex
	srv := httptest.NewServer(detailEnrichmentHandler(&fetches, &mu))
	t.Cleanup(srv.Close)

	cfg := config.DefaultAppConfig()
	cfg.Search.DetailEnrichment.Enabled = true
	cfg.Search.DetailEnrichment.MaxResults = 1
	cfg.Search.DetailEnrichment.Timeout = 2

	e := NewBaseEngine("test-detail-on", "Test Detail On", srv.URL, 1, cfg)
	results, err := genericSearch(context.Background(), e, srv.URL, ".item")
	if err != nil {
		t.Fatalf("genericSearch error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("genericSearch results = %d, want 2", len(results))
	}
	if results[0].Performer != "Detail Actor 1" {
		t.Errorf("results[0].Performer = %q, want %q (MaxResults=1 should still enrich the first result)", results[0].Performer, "Detail Actor 1")
	}
	if results[0].DurationSeconds != 100 {
		t.Errorf("results[0].DurationSeconds = %d, want 100", results[0].DurationSeconds)
	}
	if results[1].Performer != "" {
		t.Errorf("results[1].Performer = %q, want empty (MaxResults=1 caps enrichment to the first result)", results[1].Performer)
	}
	if got := atomic.LoadInt32(&fetches); got != 1 {
		t.Errorf("detail pages fetched = %d, want 1 (MaxResults=1)", got)
	}
}

func TestEnrichDetailResults_NoopOnNilConfigOrEmptyResults(t *testing.T) {
	e := &BaseEngine{name: "test-nil-cfg", baseURL: "https://example.com"}
	// appConfig is nil - must not panic and must not attempt any request.
	enrichDetailResults(context.Background(), e, []model.VideoResult{{URL: "https://example.com/v/1", Title: "x"}})

	cfg := config.DefaultAppConfig()
	cfg.Search.DetailEnrichment.Enabled = true
	e2 := NewBaseEngine("test-empty-results", "Test Empty Results", "https://example.com", 1, cfg)
	// Empty results slice - must return immediately without blocking.
	enrichDetailResults(context.Background(), e2, nil)
}
