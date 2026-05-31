// SPDX-License-Identifier: MIT
// Tests for parser package: CleanText, ParseDuration, ParseViews, ParseViewCount,
// MakeAbsoluteURL, ParseRating, RegisterParser/GetParser, ExtractAttr,
// ExtractQuality, ExtractTags, ExtractUploader, IsPremiumContent.
package parser

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// newDoc parses raw HTML and returns the document's root Selection.
func newDoc(html string) *goquery.Selection {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		panic("newDoc: " + err.Error())
	}
	return doc.Selection
}

// ---- CleanText ----

// Leading/trailing whitespace and interior runs are collapsed to single spaces.
func TestCleanTextInteriorSpaces(t *testing.T) {
	got := CleanText("  hello   world  ")
	if got != "hello world" {
		t.Errorf("CleanText spaces: got %q, want %q", got, "hello world")
	}
}

// Empty input must return empty string without panicking.
func TestCleanTextEmpty(t *testing.T) {
	if got := CleanText(""); got != "" {
		t.Errorf("CleanText empty: got %q, want empty string", got)
	}
}

// Tabs and newlines count as whitespace and are collapsed.
func TestCleanTextTabsAndNewlines(t *testing.T) {
	got := CleanText("\t\n test \t")
	if got != "test" {
		t.Errorf("CleanText tab/newline: got %q, want %q", got, "test")
	}
}

// A string that is already clean is returned unchanged.
func TestCleanTextAlreadyClean(t *testing.T) {
	want := "one two three"
	if got := CleanText(want); got != want {
		t.Errorf("CleanText already-clean: got %q, want %q", got, want)
	}
}

// A string of only whitespace must produce an empty string.
func TestCleanTextOnlyWhitespace(t *testing.T) {
	if got := CleanText("   \t\n   "); got != "" {
		t.Errorf("CleanText only whitespace: got %q, want empty string", got)
	}
}

// ---- ParseDuration ----

// mm:ss format returns the time string and its seconds value.
func TestParseDurationMMSS(t *testing.T) {
	str, secs := ParseDuration("4:30")
	if str != "4:30" {
		t.Errorf("ParseDuration mm:ss str: got %q, want %q", str, "4:30")
	}
	if secs != 270 {
		t.Errorf("ParseDuration mm:ss secs: got %d, want 270", secs)
	}
}

// hh:mm:ss format returns the time string and its seconds value.
func TestParseDurationHHMMSS(t *testing.T) {
	str, secs := ParseDuration("1:23:45")
	if str != "1:23:45" {
		t.Errorf("ParseDuration hh:mm:ss str: got %q, want %q", str, "1:23:45")
	}
	if secs != 5025 {
		t.Errorf("ParseDuration hh:mm:ss secs: got %d, want 5025", secs)
	}
}

// Mixed text like "HD 4:00" must extract the embedded time token.
func TestParseDurationMixedTextWithTime(t *testing.T) {
	str, secs := ParseDuration("HD 4:00")
	if str != "4:00" {
		t.Errorf("ParseDuration mixed text str: got %q, want %q", str, "4:00")
	}
	if secs != 240 {
		t.Errorf("ParseDuration mixed text secs: got %d, want 240", secs)
	}
}

// "12 minutes" verbose form must be normalised to "12:00" and 720 seconds.
func TestParseDurationMinutesLong(t *testing.T) {
	str, secs := ParseDuration("12 minutes")
	if str != "12:00" {
		t.Errorf("ParseDuration minutes str: got %q, want %q", str, "12:00")
	}
	if secs != 720 {
		t.Errorf("ParseDuration minutes secs: got %d, want 720", secs)
	}
}

// "12min" abbreviated form must also normalise to "12:00" and 720 seconds.
func TestParseDurationMinAbbr(t *testing.T) {
	str, secs := ParseDuration("12min")
	if str != "12:00" {
		t.Errorf("ParseDuration min abbr str: got %q, want %q", str, "12:00")
	}
	if secs != 720 {
		t.Errorf("ParseDuration min abbr secs: got %d, want 720", secs)
	}
}

// A plain integer treated as seconds must be displayed as m:ss (no leading hour).
func TestParseDurationPlainSeconds(t *testing.T) {
	str, secs := ParseDuration("745")
	if secs != 745 {
		t.Errorf("ParseDuration plain secs value: got %d, want 745", secs)
	}
	// 745 s = 12 min 25 s; h == 0 so format is "m:ss"
	if str != "12:25" {
		t.Errorf("ParseDuration plain secs str: got %q, want %q", str, "12:25")
	}
}

// Plain seconds that exceed one hour must use hh:mm:ss format.
func TestParseDurationPlainSecondsOverAnHour(t *testing.T) {
	str, secs := ParseDuration("3661")
	if secs != 3661 {
		t.Errorf("ParseDuration over-hour secs value: got %d, want 3661", secs)
	}
	if str != "1:01:01" {
		t.Errorf("ParseDuration over-hour str: got %q, want %q", str, "1:01:01")
	}
}

// Empty input must return empty string and zero.
func TestParseDurationEmpty(t *testing.T) {
	str, secs := ParseDuration("")
	if str != "" || secs != 0 {
		t.Errorf("ParseDuration empty: got (%q, %d), want (%q, 0)", str, secs, "")
	}
}

// A string that matches no pattern returns the original string and zero seconds.
func TestParseDurationInvalid(t *testing.T) {
	str, secs := ParseDuration("invalid")
	if str != "invalid" {
		t.Errorf("ParseDuration invalid str: got %q, want %q", str, "invalid")
	}
	if secs != 0 {
		t.Errorf("ParseDuration invalid secs: got %d, want 0", secs)
	}
}

// ---- ParseViews ----

// "1.2M" must parse to 1_200_000.
func TestParseViewsMillions(t *testing.T) {
	str, count := ParseViews("1.2M")
	if str != "1.2M" {
		t.Errorf("ParseViews 1.2M str: got %q, want %q", str, "1.2M")
	}
	if count != 1200000 {
		t.Errorf("ParseViews 1.2M count: got %d, want 1200000", count)
	}
}

// "500K" must parse to 500_000.
func TestParseViewsThousands(t *testing.T) {
	str, count := ParseViews("500K")
	if str != "500K" {
		t.Errorf("ParseViews 500K str: got %q, want %q", str, "500K")
	}
	if count != 500000 {
		t.Errorf("ParseViews 500K count: got %d, want 500000", count)
	}
}

// "1.5B" must parse to 1_500_000_000.
func TestParseViewsBillions(t *testing.T) {
	str, count := ParseViews("1.5B")
	if str != "1.5B" {
		t.Errorf("ParseViews 1.5B str: got %q, want %q", str, "1.5B")
	}
	if count != 1500000000 {
		t.Errorf("ParseViews 1.5B count: got %d, want 1500000000", count)
	}
}

// Comma-formatted plain integers must be parsed correctly.
func TestParseViewsCommaFormatted(t *testing.T) {
	str, count := ParseViews("1,234,567")
	if str != "1,234,567" {
		t.Errorf("ParseViews comma str: got %q, want %q", str, "1,234,567")
	}
	if count != 1234567 {
		t.Errorf("ParseViews comma count: got %d, want 1234567", count)
	}
}

// A " views" suffix must be stripped before parsing.
func TestParseViewsWithViewsSuffix(t *testing.T) {
	_, count := ParseViews("500K views")
	if count != 500000 {
		t.Errorf("ParseViews 'views' suffix count: got %d, want 500000", count)
	}
}

// A value with a suffix but an unparseable numeric part returns the original and zero.
func TestParseViewsInvalidNumericWithSuffix(t *testing.T) {
	str, count := ParseViews("fooK")
	if count != 0 {
		t.Errorf("ParseViews invalid numeric with suffix: got %d, want 0", count)
	}
	if str == "" {
		t.Error("ParseViews invalid numeric with suffix: original string must be preserved")
	}
}

// Empty string must return empty string and zero.
func TestParseViewsEmpty(t *testing.T) {
	str, count := ParseViews("")
	if str != "" || count != 0 {
		t.Errorf("ParseViews empty: got (%q, %d), want ('', 0)", str, count)
	}
}

// ---- ParseViewCount ----

// ParseViewCount is the count-only shorthand for ParseViews.
func TestParseViewCountMillions(t *testing.T) {
	if got := ParseViewCount("1.2M"); got != 1200000 {
		t.Errorf("ParseViewCount 1.2M: got %d, want 1200000", got)
	}
}

// Empty string must return zero without panicking.
func TestParseViewCountEmpty(t *testing.T) {
	if got := ParseViewCount(""); got != 0 {
		t.Errorf("ParseViewCount empty: got %d, want 0", got)
	}
}

// ---- MakeAbsoluteURL ----

// An already-absolute http URL must be returned unchanged.
func TestMakeAbsoluteURLAlreadyHTTP(t *testing.T) {
	in := "http://example.com/path"
	if got := MakeAbsoluteURL(in, "https://site.com"); got != in {
		t.Errorf("MakeAbsoluteURL http: got %q, want %q", got, in)
	}
}

// An already-absolute https URL must be returned unchanged.
func TestMakeAbsoluteURLAlreadyHTTPS(t *testing.T) {
	in := "https://example.com/path"
	if got := MakeAbsoluteURL(in, "https://site.com"); got != in {
		t.Errorf("MakeAbsoluteURL https: got %q, want %q", got, in)
	}
}

// A protocol-relative URL must be prepended with "https:".
func TestMakeAbsoluteURLProtocolRelative(t *testing.T) {
	want := "https://example.com/path"
	if got := MakeAbsoluteURL("//example.com/path", "https://site.com"); got != want {
		t.Errorf("MakeAbsoluteURL //: got %q, want %q", got, want)
	}
}

// A root-relative path must be joined to the base host.
func TestMakeAbsoluteURLRootRelative(t *testing.T) {
	want := "https://site.com/path"
	if got := MakeAbsoluteURL("/path", "https://site.com"); got != want {
		t.Errorf("MakeAbsoluteURL /path: got %q, want %q", got, want)
	}
}

// A bare relative path must be joined with a "/" separator.
func TestMakeAbsoluteURLBareRelative(t *testing.T) {
	want := "https://site.com/path"
	if got := MakeAbsoluteURL("path", "https://site.com"); got != want {
		t.Errorf("MakeAbsoluteURL bare: got %q, want %q", got, want)
	}
}

// An empty href must return an empty string.
func TestMakeAbsoluteURLEmpty(t *testing.T) {
	if got := MakeAbsoluteURL("", "https://site.com"); got != "" {
		t.Errorf("MakeAbsoluteURL empty: got %q, want empty string", got)
	}
}

// ---- ParseRating ----

// Percentage strings must be returned with their numeric value on [0,100].
func TestParseRatingPercent(t *testing.T) {
	str, val := ParseRating("93%")
	if str != "93%" {
		t.Errorf("ParseRating %%: str got %q, want %q", str, "93%")
	}
	if val != 93.0 {
		t.Errorf("ParseRating %%: val got %f, want 93.0", val)
	}
}

// "x/5" notation must be converted to a 0-100 score.
func TestParseRatingOutOfFive(t *testing.T) {
	str, val := ParseRating("4.5/5")
	if str != "4.5/5" {
		t.Errorf("ParseRating /5: str got %q, want %q", str, "4.5/5")
	}
	if val != 90.0 {
		t.Errorf("ParseRating /5: val got %f, want 90.0", val)
	}
}

// "x stars" must be treated as a 5-star scale and converted to 0-100.
func TestParseRatingStarsPlural(t *testing.T) {
	str, val := ParseRating("4.5 stars")
	if str != "4.5" {
		t.Errorf("ParseRating stars str: got %q, want %q", str, "4.5")
	}
	if val != 90.0 {
		t.Errorf("ParseRating stars val: got %f, want 90.0", val)
	}
}

// "x star" (singular) must be treated identically to the plural form.
func TestParseRatingStarSingular(t *testing.T) {
	_, val := ParseRating("4.5 star")
	if val != 90.0 {
		t.Errorf("ParseRating star singular val: got %f, want 90.0", val)
	}
}

// A bare float ≤ 10 but > 5 must be treated as a 10-point scale.
func TestParseRatingTenPointScale(t *testing.T) {
	str, val := ParseRating("7.5")
	if str != "7.5" {
		t.Errorf("ParseRating 10-pt str: got %q, want %q", str, "7.5")
	}
	if val != 75.0 {
		t.Errorf("ParseRating 10-pt val: got %f, want 75.0", val)
	}
}

// Empty string must return ("", 0) without panicking.
func TestParseRatingEmpty(t *testing.T) {
	str, val := ParseRating("")
	if str != "" || val != 0 {
		t.Errorf("ParseRating empty: got (%q, %f), want ('', 0)", str, val)
	}
}

// A value ≤ 5 (5-star scale) must map correctly.
func TestParseRatingFiveStarScale(t *testing.T) {
	_, val := ParseRating("5.0")
	if val != 100.0 {
		t.Errorf("ParseRating 5-star max: got %f, want 100.0", val)
	}
}

// A bare float > 10 must be returned as-is on the 100 scale.
func TestParseRatingHundredScale(t *testing.T) {
	_, val := ParseRating("85.5")
	if val != 85.5 {
		t.Errorf("ParseRating 100-scale: got %f, want 85.5", val)
	}
}

// A string that has no parseable numeric content must return ("", 0) — covers
// the final fallback branch after stars/star replacement leaves a non-float.
func TestParseRatingNonNumeric(t *testing.T) {
	str, val := ParseRating("not-a-rating")
	if val != 0 {
		t.Errorf("ParseRating non-numeric val: got %f, want 0", val)
	}
	if str == "" {
		t.Error("ParseRating non-numeric: original text must be preserved in str")
	}
}

// ---- RegisterParser / GetParser ----

// Built-in parsers registered during init must be retrievable.
func TestGetParserPornhubNonNil(t *testing.T) {
	if p := GetParser("pornhub"); p == nil {
		t.Error("GetParser(pornhub) returned nil; parser must be registered in init()")
	}
}

func TestGetParserXvideosNonNil(t *testing.T) {
	if p := GetParser("xvideos"); p == nil {
		t.Error("GetParser(xvideos) returned nil; parser must be registered in init()")
	}
}

func TestGetParserXnxxNonNil(t *testing.T) {
	if p := GetParser("xnxx"); p == nil {
		t.Error("GetParser(xnxx) returned nil; parser must be registered in init()")
	}
}

func TestGetParserRedtubeNonNil(t *testing.T) {
	if p := GetParser("redtube"); p == nil {
		t.Error("GetParser(redtube) returned nil; parser must be registered in init()")
	}
}

func TestGetParserEpornerNonNil(t *testing.T) {
	if p := GetParser("eporner"); p == nil {
		t.Error("GetParser(eporner) returned nil; parser must be registered in init()")
	}
}

func TestGetParserPornmdNonNil(t *testing.T) {
	if p := GetParser("pornmd"); p == nil {
		t.Error("GetParser(pornmd) returned nil; parser must be registered in init()")
	}
}

// GetParser with an unknown name must return nil.
func TestGetParserNonexistentReturnsNil(t *testing.T) {
	if p := GetParser("nonexistent-site-xyz"); p != nil {
		t.Error("GetParser(nonexistent) must return nil")
	}
}

// A freshly registered parser must be immediately retrievable.
func TestRegisterAndGetParser(t *testing.T) {
	stub := &stubParser{}
	RegisterParser("test-parser-stub", stub)
	got := GetParser("test-parser-stub")
	if got == nil {
		t.Fatal("GetParser returned nil after RegisterParser")
	}
	if got != stub {
		t.Error("GetParser returned a different value than what was registered")
	}
}

// Registering the same name twice must overwrite the previous entry.
func TestRegisterParserOverwrites(t *testing.T) {
	first := &stubParser{}
	second := &stubParser{}
	RegisterParser("overwrite-test", first)
	RegisterParser("overwrite-test", second)
	if got := GetParser("overwrite-test"); got != second {
		t.Error("second RegisterParser call must overwrite the first")
	}
}

// stubParser satisfies VideoSiteParser for registration tests.
type stubParser struct{}

func (s *stubParser) ItemSelector() string           { return ".item" }
func (s *stubParser) Parse(_ *goquery.Selection) *VideoItem { return nil }

// ---- ExtractAttr ----

// The first matching non-empty attribute is returned.
func TestExtractAttrFirstMatch(t *testing.T) {
	sel := newDoc(`<a href="http://x.com" src="other.jpg">link</a>`).Find("a")
	got := ExtractAttr(sel, "href", "src")
	if got != "http://x.com" {
		t.Errorf("ExtractAttr first match: got %q, want %q", got, "http://x.com")
	}
}

// When the first named attribute is absent or empty, the next is tried.
func TestExtractAttrFallback(t *testing.T) {
	sel := newDoc(`<a data-src="fallback.jpg">link</a>`).Find("a")
	got := ExtractAttr(sel, "href", "data-src")
	if got != "fallback.jpg" {
		t.Errorf("ExtractAttr fallback: got %q, want %q", got, "fallback.jpg")
	}
}

// When none of the attributes exist the function returns an empty string.
func TestExtractAttrNonePresent(t *testing.T) {
	sel := newDoc(`<a>link</a>`).Find("a")
	got := ExtractAttr(sel, "href", "src")
	if got != "" {
		t.Errorf("ExtractAttr none present: got %q, want empty string", got)
	}
}

// An empty attribute value is skipped in favour of the next attribute.
func TestExtractAttrSkipsEmpty(t *testing.T) {
	sel := newDoc(`<img alt="" src="image.jpg">`).Find("img")
	got := ExtractAttr(sel, "alt", "src")
	if got != "image.jpg" {
		t.Errorf("ExtractAttr skip empty: got %q, want %q", got, "image.jpg")
	}
}

// ---- IsPremiumContent ----

// A URL containing "premium" must be detected as premium.
func TestIsPremiumContentURLContainsPremium(t *testing.T) {
	sel := newDoc(`<div></div>`).Find("div")
	if !IsPremiumContent(sel, "https://site.com/premium/video") {
		t.Error("IsPremiumContent: URL with 'premium' must return true")
	}
}

// A URL containing "gold" must be detected as premium.
func TestIsPremiumContentURLContainsGold(t *testing.T) {
	sel := newDoc(`<div></div>`).Find("div")
	if !IsPremiumContent(sel, "https://site.com/gold/video") {
		t.Error("IsPremiumContent: URL with 'gold' must return true")
	}
}

// A URL containing "vip" must be detected as premium.
func TestIsPremiumContentURLContainsVip(t *testing.T) {
	sel := newDoc(`<div></div>`).Find("div")
	if !IsPremiumContent(sel, "https://site.com/vip/video") {
		t.Error("IsPremiumContent: URL with 'vip' must return true")
	}
}

// A plain URL with no premium indicators and no premium HTML must return false.
func TestIsPremiumContentPlainFalse(t *testing.T) {
	sel := newDoc(`<div class="video-item"><span class="title">My Video</span></div>`).Find("div")
	if IsPremiumContent(sel, "https://site.com/video/123") {
		t.Error("IsPremiumContent: plain URL and HTML must return false")
	}
}

// HTML containing the word "exclusive" inside a node must be detected as premium.
func TestIsPremiumContentHTMLExclusive(t *testing.T) {
	sel := newDoc(`<div><span class="badge">exclusive</span></div>`).Find("div")
	if !IsPremiumContent(sel, "https://site.com/video/123") {
		t.Error("IsPremiumContent: HTML with 'exclusive' must return true")
	}
}

// ---- ExtractTags ----

// Tags matching a CSS selector are returned in document order.
func TestExtractTagsBasic(t *testing.T) {
	sel := newDoc(`<div><a class="tag">comedy</a><a class="tag">action</a></div>`).Find("div")
	tags := ExtractTags(sel, ".tag")
	if len(tags) != 2 {
		t.Fatalf("ExtractTags basic: got %d tags, want 2", len(tags))
	}
	if tags[0] != "comedy" || tags[1] != "action" {
		t.Errorf("ExtractTags basic: got %v, want [comedy action]", tags)
	}
}

// Duplicate tags (case-insensitive) must be deduplicated.
func TestExtractTagsDeduplication(t *testing.T) {
	sel := newDoc(`<div><a class="tag">Comedy</a><a class="tag">comedy</a><a class="tag">Action</a></div>`).Find("div")
	tags := ExtractTags(sel, ".tag")
	if len(tags) != 2 {
		t.Errorf("ExtractTags dedup: got %d tags, want 2 (deduplicated)", len(tags))
	}
}

// An empty selection must return an empty (or nil) slice without panicking.
func TestExtractTagsEmptySelection(t *testing.T) {
	sel := newDoc(`<div></div>`).Find("div")
	tags := ExtractTags(sel, ".tag")
	if len(tags) != 0 {
		t.Errorf("ExtractTags empty selection: got %d tags, want 0", len(tags))
	}
}

// More than 10 unique tags must be capped at exactly 10.
func TestExtractTagsLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<div>")
	for i := 0; i < 15; i++ {
		sb.WriteString(`<a class="tag">uniquetag`)
		sb.WriteByte(byte('a' + i))
		sb.WriteString(`</a>`)
	}
	sb.WriteString("</div>")
	sel := newDoc(sb.String()).Find("div")
	tags := ExtractTags(sel, ".tag")
	if len(tags) != 10 {
		t.Errorf("ExtractTags limit: got %d tags, want exactly 10", len(tags))
	}
}

// Tags with fewer than two characters must be silently dropped.
func TestExtractTagsSkipsShortTags(t *testing.T) {
	sel := newDoc(`<div><a class="tag">a</a><a class="tag">ok</a></div>`).Find("div")
	tags := ExtractTags(sel, ".tag")
	for _, tag := range tags {
		if len(tag) < 2 {
			t.Errorf("ExtractTags: short tag %q must be skipped", tag)
		}
	}
}

// ---- ExtractUploader ----

// The uploader text is returned when the selector matches.
func TestExtractUploaderFound(t *testing.T) {
	sel := newDoc(`<div><span class="uploader">JohnDoe</span></div>`).Find("div")
	got := ExtractUploader(sel, ".uploader")
	if got != "JohnDoe" {
		t.Errorf("ExtractUploader found: got %q, want %q", got, "JohnDoe")
	}
}

// When no selector matches an empty string is returned.
func TestExtractUploaderMissing(t *testing.T) {
	sel := newDoc(`<div><span class="name">Author</span></div>`).Find("div")
	got := ExtractUploader(sel, ".uploader")
	if got != "" {
		t.Errorf("ExtractUploader missing: got %q, want empty string", got)
	}
}

// The first non-empty selector result wins when multiple are provided.
func TestExtractUploaderFallback(t *testing.T) {
	sel := newDoc(`<div><span class="channel">ChanName</span></div>`).Find("div")
	got := ExtractUploader(sel, ".uploader", ".channel")
	if got != "ChanName" {
		t.Errorf("ExtractUploader fallback: got %q, want %q", got, "ChanName")
	}
}

// Whitespace-only uploader elements must be skipped.
func TestExtractUploaderWhitespaceSkipped(t *testing.T) {
	sel := newDoc(`<div><span class="uploader">   </span><span class="channel">Real</span></div>`).Find("div")
	got := ExtractUploader(sel, ".uploader", ".channel")
	if got != "Real" {
		t.Errorf("ExtractUploader whitespace skip: got %q, want %q", got, "Real")
	}
}

// ---- ExtractQuality ----

// An element with class "hd-badge" must be detected as a quality indicator.
func TestExtractQualityHDBadge(t *testing.T) {
	sel := newDoc(`<div><span class="hd-badge">HD</span></div>`).Find("div")
	got := ExtractQuality(sel)
	if got != "HD" {
		t.Errorf("ExtractQuality hd-badge: got %q, want %q", got, "HD")
	}
}

// An element with class "quality" must be returned as-is.
func TestExtractQualityClass(t *testing.T) {
	sel := newDoc(`<div><span class="quality">4K</span></div>`).Find("div")
	got := ExtractQuality(sel)
	if got != "4K" {
		t.Errorf("ExtractQuality class: got %q, want %q", got, "4K")
	}
}

// An element whose text is empty but whose class contains "4k" must return "4K".
func TestExtractQualityClassBased4K(t *testing.T) {
	sel := newDoc(`<div><span class="badge-4k"></span></div>`).Find("div")
	got := ExtractQuality(sel)
	if got != "4K" {
		t.Errorf("ExtractQuality class-based 4k: got %q, want %q", got, "4K")
	}
}

// An element whose text is empty but whose class contains "hd" must return "HD".
func TestExtractQualityClassBasedHD(t *testing.T) {
	sel := newDoc(`<div><span class="quality-hd"></span></div>`).Find("div")
	got := ExtractQuality(sel)
	if got != "HD" {
		t.Errorf("ExtractQuality class-based hd: got %q, want %q", got, "HD")
	}
}

// When no quality indicator is present an empty string is returned.
func TestExtractQualityNone(t *testing.T) {
	sel := newDoc(`<div><span class="title">My Video</span></div>`).Find("div")
	got := ExtractQuality(sel)
	if got != "" {
		t.Errorf("ExtractQuality none: got %q, want empty string", got)
	}
}
