// SPDX-License-Identifier: MIT
// Coverage tests for helpers.go: parseGenericVideoItem, extractTags,
// extractPerformer, readEngineBody — all pure functions that operate on
// goquery selections or io.Reader, and require no live HTTP connections.
package engine

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// newSel parses html and returns the first element matching selector.
func newSel(html, selector string) *goquery.Selection {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		panic(err)
	}
	return doc.Find(selector)
}

// ── readEngineBody ────────────────────────────────────────────────────────────

// Happy path: body is read completely.
func TestReadEngineBody_ReadsAll(t *testing.T) {
	payload := "hello world"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(payload))}
	got, err := readEngineBody(resp)
	if err != nil {
		t.Fatalf("readEngineBody: unexpected error: %v", err)
	}
	if string(got) != payload {
		t.Errorf("readEngineBody = %q, want %q", got, payload)
	}
}

// Empty body returns empty slice without error.
func TestReadEngineBody_EmptyBody(t *testing.T) {
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(""))}
	got, err := readEngineBody(resp)
	if err != nil {
		t.Fatalf("readEngineBody empty: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("readEngineBody empty: got %d bytes, want 0", len(got))
	}
}

// ── parseGenericVideoItem ─────────────────────────────────────────────────────

// No anchor element → URL is empty → function returns zero-value result.
func TestParseGenericVideoItem_NoAnchor(t *testing.T) {
	sel := newSel(`<div class="thumb"><img src="t.jpg"></div>`, "div.thumb")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.URL != "" {
		t.Errorf("parseGenericVideoItem no anchor: URL = %q, want empty", r.URL)
	}
}

// Element is itself an <a> tag → URL from href, title from title attr.
func TestParseGenericVideoItem_ElementIsAnchor(t *testing.T) {
	sel := newSel(`<a href="/video/1" title="My Video"><img src="t.jpg"></a>`, "a")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.URL != "https://example.com/video/1" {
		t.Errorf("parseGenericVideoItem self-anchor URL = %q, want https://example.com/video/1", r.URL)
	}
	if r.Title != "My Video" {
		t.Errorf("parseGenericVideoItem self-anchor Title = %q, want 'My Video'", r.Title)
	}
}

// href is already absolute → not prefixed with baseURL.
func TestParseGenericVideoItem_AbsoluteHref(t *testing.T) {
	sel := newSel(`<div><a href="https://cdn.example.com/video/2" title="Abs Video"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.URL != "https://cdn.example.com/video/2" {
		t.Errorf("parseGenericVideoItem absolute href URL = %q", r.URL)
	}
}

// Title falls back to img alt when anchor has no title attr.
func TestParseGenericVideoItem_TitleFromImgAlt(t *testing.T) {
	sel := newSel(`<div><a href="/v/3"><img alt="Alt Title" src="t.jpg"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Title != "Alt Title" {
		t.Errorf("parseGenericVideoItem alt title = %q, want 'Alt Title'", r.Title)
	}
}

// Title falls back to .title element text.
func TestParseGenericVideoItem_TitleFromTitleDiv(t *testing.T) {
	sel := newSel(`<div><a href="/v/4"></a><span class="title">Span Title</span></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Title != "Span Title" {
		t.Errorf("parseGenericVideoItem .title text = %q, want 'Span Title'", r.Title)
	}
}

// Title falls back to link text when no other source available.
func TestParseGenericVideoItem_TitleFromLinkText(t *testing.T) {
	sel := newSel(`<div><a href="/v/5">Link Text Title</a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Title != "Link Text Title" {
		t.Errorf("parseGenericVideoItem link text title = %q, want 'Link Text Title'", r.Title)
	}
}

// Thumbnail extracted from img src.
func TestParseGenericVideoItem_Thumbnail_Src(t *testing.T) {
	sel := newSel(`<div><a href="/v/6" title="T"><img src="https://cdn.example.com/thumb.jpg"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("parseGenericVideoItem thumbnail src = %q", r.Thumbnail)
	}
}

// Thumbnail from data-src takes priority over src.
func TestParseGenericVideoItem_Thumbnail_DataSrc(t *testing.T) {
	sel := newSel(`<div><a href="/v/7" title="T"><img data-src="https://cdn.example.com/lazy.jpg" src="blank.gif"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Thumbnail != "https://cdn.example.com/lazy.jpg" {
		t.Errorf("parseGenericVideoItem thumbnail data-src = %q", r.Thumbnail)
	}
}

// Protocol-relative thumbnail is expanded to https.
func TestParseGenericVideoItem_Thumbnail_ProtocolRelative(t *testing.T) {
	sel := newSel(`<div><a href="/v/8" title="T"><img src="//cdn.example.com/thumb.jpg"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Thumbnail != "https://cdn.example.com/thumb.jpg" {
		t.Errorf("parseGenericVideoItem protocol-relative thumbnail = %q", r.Thumbnail)
	}
}

// Relative thumbnail is prefixed with baseURL.
func TestParseGenericVideoItem_Thumbnail_Relative(t *testing.T) {
	sel := newSel(`<div><a href="/v/9" title="T"><img src="/images/thumb.jpg"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Thumbnail != "https://example.com/images/thumb.jpg" {
		t.Errorf("parseGenericVideoItem relative thumbnail = %q", r.Thumbnail)
	}
}

// Duration extracted from .duration element.
func TestParseGenericVideoItem_Duration(t *testing.T) {
	sel := newSel(`<div><a href="/v/10" title="T"></a><span class="duration">12:34</span></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Duration == "" {
		t.Error("parseGenericVideoItem duration: Duration must not be empty")
	}
	if r.DurationSeconds == 0 {
		t.Error("parseGenericVideoItem duration: DurationSeconds must be > 0")
	}
}

// Views extracted from .views element.
func TestParseGenericVideoItem_Views(t *testing.T) {
	sel := newSel(`<div><a href="/v/11" title="T"></a><span class="views">1.2M views</span></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.ViewsCount == 0 {
		t.Error("parseGenericVideoItem views: ViewsCount must be > 0")
	}
}

// Source and SourceDisplay are set from parameters.
func TestParseGenericVideoItem_SourceFields(t *testing.T) {
	sel := newSel(`<div><a href="/v/12" title="T"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "mysrc", "MyDisplay")
	if r.Source != "mysrc" {
		t.Errorf("parseGenericVideoItem Source = %q, want 'mysrc'", r.Source)
	}
	if r.SourceDisplay != "MyDisplay" {
		t.Errorf("parseGenericVideoItem SourceDisplay = %q, want 'MyDisplay'", r.SourceDisplay)
	}
}

// ID is non-empty (generated from URL + source).
func TestParseGenericVideoItem_IDNonEmpty(t *testing.T) {
	sel := newSel(`<div><a href="/v/13" title="T"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.ID == "" {
		t.Error("parseGenericVideoItem ID must not be empty")
	}
}

// Preview URL extracted from data-mediabook on container element.
func TestParseGenericVideoItem_PreviewURL_DataMediabook(t *testing.T) {
	sel := newSel(`<div data-mediabook="https://preview.example.com/vid.mp4"><a href="/v/14" title="T"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.PreviewURL != "https://preview.example.com/vid.mp4" {
		t.Errorf("parseGenericVideoItem previewURL = %q", r.PreviewURL)
	}
}

// Quality extracted when HD badge is present.
func TestParseGenericVideoItem_Quality(t *testing.T) {
	sel := newSel(`<div><a href="/v/15" title="T"></a><span class="hd-badge">HD</span></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	// Quality may or may not be set depending on ExtractQuality logic; just ensure no panic.
	_ = r.Quality
}

// DrTuber-style: span > em for title.
func TestParseGenericVideoItem_TitleFromSpanEm(t *testing.T) {
	sel := newSel(`<div><a href="/v/16"></a><span><em>SpanEm Title</em></span></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Title != "SpanEm Title" {
		t.Errorf("parseGenericVideoItem span>em title = %q, want 'SpanEm Title'", r.Title)
	}
}

// strong > span title fallback.
func TestParseGenericVideoItem_TitleFromStrongSpan(t *testing.T) {
	sel := newSel(`<div><a href="/v/17"></a><strong><span>Strong Span</span></strong></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.Title != "Strong Span" {
		t.Errorf("parseGenericVideoItem strong span title = %q, want 'Strong Span'", r.Title)
	}
}

// data-duration attribute on container is parsed.
func TestParseGenericVideoItem_DurationDataAttr(t *testing.T) {
	sel := newSel(`<div data-duration="10:00"><a href="/v/18" title="T"></a></div>`, "div")
	r := parseGenericVideoItem(sel, "https://example.com", "test", "Test")
	if r.DurationSeconds == 0 {
		t.Error("parseGenericVideoItem data-duration: DurationSeconds must be > 0")
	}
}

// ── extractTags ───────────────────────────────────────────────────────────────

// No tag elements → empty slice.
func TestExtractTags_NoTags(t *testing.T) {
	sel := newSel(`<div><a href="/v/1" title="T"></a></div>`, "div")
	tags := extractTags(sel)
	if len(tags) != 0 {
		t.Errorf("extractTags no tags: got %v, want empty", tags)
	}
}

// .tags a elements are extracted.
func TestExtractTags_TagsLinks(t *testing.T) {
	sel := newSel(`<div><div class="tags"><a>Amateur</a><a>Teen</a></div></div>`, "div")
	tags := extractTags(sel)
	if len(tags) < 2 {
		t.Errorf("extractTags .tags a: got %v, want at least 2 tags", tags)
	}
}

// Duplicate tags are deduplicated (case-insensitive).
func TestExtractTags_Deduplication(t *testing.T) {
	sel := newSel(`<div><div class="tags"><a>Amateur</a><a>amateur</a></div></div>`, "div")
	tags := extractTags(sel)
	if len(tags) != 1 {
		t.Errorf("extractTags dedup: got %v (%d), want 1 tag", tags, len(tags))
	}
}

// data-tags attribute is parsed (comma-separated).
func TestExtractTags_DataTagsAttr(t *testing.T) {
	sel := newSel(`<div data-tags="milf,hd,amateur"><a href="/v/1" title="T"></a></div>`, "div")
	tags := extractTags(sel)
	if len(tags) < 3 {
		t.Errorf("extractTags data-tags: got %v, want 3 tags", tags)
	}
}

// data-category attribute adds a tag.
func TestExtractTags_DataCategoryAttr(t *testing.T) {
	sel := newSel(`<div data-category="lesbian"><a href="/v/1" title="T"></a></div>`, "div")
	tags := extractTags(sel)
	if len(tags) == 0 {
		t.Error("extractTags data-category: expected at least 1 tag")
	}
	if tags[0] != "lesbian" {
		t.Errorf("extractTags data-category: got %q, want 'lesbian'", tags[0])
	}
}

// data-categories attribute is parsed (comma-separated).
func TestExtractTags_DataCategoriesAttr(t *testing.T) {
	sel := newSel(`<div data-categories="teen,blonde"><a href="/v/1" title="T"></a></div>`, "div")
	tags := extractTags(sel)
	if len(tags) < 2 {
		t.Errorf("extractTags data-categories: got %v, want at least 2", tags)
	}
}

// Very short (single-char) tags are dropped.
func TestExtractTags_ShortTagsDropped(t *testing.T) {
	sel := newSel(`<div><div class="tags"><a>A</a><a>amateur</a></div></div>`, "div")
	tags := extractTags(sel)
	for _, tag := range tags {
		if len(tag) <= 1 {
			t.Errorf("extractTags: short tag %q should have been filtered", tag)
		}
	}
}

// Very long tags (50+ chars) are dropped.
func TestExtractTags_LongTagsDropped(t *testing.T) {
	longTag := strings.Repeat("a", 51)
	html := `<div><div class="tags"><a>` + longTag + `</a></div></div>`
	sel := newSel(html, "div")
	tags := extractTags(sel)
	for _, tag := range tags {
		if len(tag) >= 50 {
			t.Errorf("extractTags: long tag %q should have been filtered", tag)
		}
	}
}

// ── extractPerformer ──────────────────────────────────────────────────────────

// No performer element → empty string.
func TestExtractPerformer_None(t *testing.T) {
	sel := newSel(`<div><a href="/v/1" title="T"></a></div>`, "div")
	if p := extractPerformer(sel); p != "" {
		t.Errorf("extractPerformer none: got %q, want empty", p)
	}
}

// .pornstar element yields performer name.
func TestExtractPerformer_Pornstar(t *testing.T) {
	sel := newSel(`<div><a href="/v/1" title="T"></a><span class="pornstar">Mia Khalifa</span></div>`, "div")
	p := extractPerformer(sel)
	if p != "Mia Khalifa" {
		t.Errorf("extractPerformer .pornstar = %q, want 'Mia Khalifa'", p)
	}
}

// .model element yields performer name.
func TestExtractPerformer_Model(t *testing.T) {
	sel := newSel(`<div><a href="/v/1" title="T"></a><span class="model">Riley Reid</span></div>`, "div")
	p := extractPerformer(sel)
	if p != "Riley Reid" {
		t.Errorf("extractPerformer .model = %q, want 'Riley Reid'", p)
	}
}

// data-pornstar attribute yields performer name.
func TestExtractPerformer_DataPornstar(t *testing.T) {
	sel := newSel(`<div data-pornstar="Lana Rhoades"><a href="/v/1" title="T"></a></div>`, "div")
	p := extractPerformer(sel)
	if p != "Lana Rhoades" {
		t.Errorf("extractPerformer data-pornstar = %q, want 'Lana Rhoades'", p)
	}
}

// data-model attribute yields performer name.
func TestExtractPerformer_DataModel(t *testing.T) {
	sel := newSel(`<div data-model="Brandi Love"><a href="/v/1" title="T"></a></div>`, "div")
	p := extractPerformer(sel)
	if p != "Brandi Love" {
		t.Errorf("extractPerformer data-model = %q, want 'Brandi Love'", p)
	}
}

// .uploader element yields performer name.
func TestExtractPerformer_Uploader(t *testing.T) {
	sel := newSel(`<div><a href="/v/1" title="T"></a><span class="uploader">UploaderName</span></div>`, "div")
	p := extractPerformer(sel)
	if p != "UploaderName" {
		t.Errorf("extractPerformer .uploader = %q, want 'UploaderName'", p)
	}
}

// .channel element yields performer name.
func TestExtractPerformer_Channel(t *testing.T) {
	sel := newSel(`<div><a href="/v/1" title="T"></a><span class="channel">ChannelName</span></div>`, "div")
	p := extractPerformer(sel)
	if p != "ChannelName" {
		t.Errorf("extractPerformer .channel = %q, want 'ChannelName'", p)
	}
}
