// SPDX-License-Identifier: MIT
package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
)

// MaxEngineResponseBytes caps the response body size read from any third-party
// engine to prevent unbounded memory allocation from a malicious or
// misbehaving upstream (32 MiB is well above any legitimate search HTML page).
const MaxEngineResponseBytes int64 = 32 * 1024 * 1024

// readEngineBody reads the response body from an engine endpoint with a hard
// size cap. All engine response bodies MUST be read via this helper - never
// io.ReadAll directly on resp.Body (PART 11 security: bound untrusted input).
func readEngineBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, MaxEngineResponseBytes))
}

// genericSearch performs a generic search using common patterns
func genericSearch(ctx context.Context, e *BaseEngine, url, selector string) ([]model.VideoResult, error) {
	resp, err := e.MakeRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read body for debug logging (size-capped)
	body, err := readEngineBody(resp)
	if err != nil {
		return nil, err
	}

	// Log response metadata when debug is enabled (never the raw body)
	debugLogEngineResponse(e.Name(), url, len(body))

	// Parse HTML from body
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Best-effort schema.org VideoObject data emitted as JSON-LD, keyed by
	// the video's absolute URL so it can be merged into the matching card.
	ldIndex := parseJSONLDVideos(doc, e.baseURL)

	var results []model.VideoResult
	fieldStats := map[string]int{
		"preview": 0,
		"thumb":   0,
		"quality": 0,
	}
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		r := parseGenericVideoItem(s, e.baseURL, e.Name(), e.DisplayName())
		if r.Title != "" && r.URL != "" {
			if ld, ok := ldIndex[r.URL]; ok {
				mergeLDVideoInfo(&r, ld)
			}
			results = append(results, r)
			if r.PreviewURL != "" {
				fieldStats["preview"]++
			}
			if r.Thumbnail != "" {
				fieldStats["thumb"]++
			}
			if r.Quality != "" {
				fieldStats["quality"]++
			}
		}
	})

	// Log parse results when debug is enabled
	debugLogEngineParseResult(e.Name(), results, fieldStats)

	return results, nil
}

// parseGenericVideoItem extracts video data using common patterns
func parseGenericVideoItem(s *goquery.Selection, baseURL, sourceName, sourceDisplay string) model.VideoResult {
	var r model.VideoResult

	// Find link - check if element itself is a link first
	var link *goquery.Selection
	if s.Is("a") {
		link = s
	} else {
		link = s.Find("a").First()
	}
	href := parser.ExtractAttr(link, "href")
	if href == "" {
		return r
	}
	if !strings.HasPrefix(href, "http") {
		href = baseURL + href
	}
	r.URL = href

	// Find title - try multiple patterns
	r.Title = parser.ExtractAttr(link, "title")
	if r.Title == "" {
		// Try alt from image (common in card layouts)
		img := s.Find("img").First()
		r.Title = parser.ExtractAttr(img, "alt")
	}
	if r.Title == "" {
		// Try specific title selectors
		titleElem := s.Find(".title, .name, .video-title, a.video-title, h4, h3")
		r.Title = parser.CleanText(titleElem.First().Text())
	}
	if r.Title == "" {
		// DrTuber-style: span > em for title
		titleEm := s.Find("span > em")
		r.Title = parser.CleanText(titleEm.First().Text())
	}
	if r.Title == "" {
		// Try strong > span for title
		titleSpan := s.Find("strong span, strong em")
		r.Title = parser.CleanText(titleSpan.First().Text())
	}
	if r.Title == "" {
		r.Title = parser.CleanText(link.Text())
	}

	// Find thumbnail
	img := s.Find("img").First()
	r.Thumbnail = parser.ExtractAttr(img, "data-src", "data-original", "data-lazy-src", "src")
	if r.Thumbnail != "" && !strings.HasPrefix(r.Thumbnail, "http") {
		if strings.HasPrefix(r.Thumbnail, "//") {
			r.Thumbnail = "https:" + r.Thumbnail
		} else {
			r.Thumbnail = baseURL + r.Thumbnail
		}
	}

	// Find preview URL - common data attributes for video preview/rollover.
	// "data-preview-custom" is specifically used by PornHat.
	previewAttrs := []string{
		"data-mediabook", "data-preview", "data-video-preview", "data-rollover",
		"data-preview-url", "data-gif", "data-webm", "data-mp4",
		"data-thumb-url", "data-trailer", "data-teaser",
		"data-preview-custom",
	}
	// Check on the container element
	for _, attr := range previewAttrs {
		if preview := parser.ExtractAttr(s, attr); preview != "" {
			if !strings.HasPrefix(preview, "http") {
				if strings.HasPrefix(preview, "//") {
					preview = "https:" + preview
				} else {
					preview = baseURL + preview
				}
			}
			r.PreviewURL = preview
			break
		}
	}
	// Check on the image element
	if r.PreviewURL == "" {
		for _, attr := range previewAttrs {
			if preview := parser.ExtractAttr(img, attr); preview != "" {
				if !strings.HasPrefix(preview, "http") {
					if strings.HasPrefix(preview, "//") {
						preview = "https:" + preview
					} else {
						preview = baseURL + preview
					}
				}
				r.PreviewURL = preview
				break
			}
		}
	}
	// Check on the link element
	if r.PreviewURL == "" {
		for _, attr := range previewAttrs {
			if preview := parser.ExtractAttr(link, attr); preview != "" {
				if !strings.HasPrefix(preview, "http") {
					if strings.HasPrefix(preview, "//") {
						preview = "https:" + preview
					} else {
						preview = baseURL + preview
					}
				}
				r.PreviewURL = preview
				break
			}
		}
	}

	// Find duration - try multiple selectors and also data attributes
	durSelectors := []string{
		// Generic common patterns
		".duration", ".dur", ".time", ".length", ".video-duration",
		"var.duration", "span.duration", ".video_duration", ".video__time",
		".thumb__time", ".thumb-time", ".thumb-duration", ".video-time",
		"time", ".meta-duration", ".card-duration",
		// TNAFlix: "thumb-icon video-duration"
		".thumb-icon.video-duration",
		// PornTrex / PornHat / similar platforms
		".item-time", ".time-badge", ".card-time",
		".video__duration", ".vid-duration", ".label-duration",
		// 4tube / Fux / PornerBros (share a codebase)
		"var.time", "var.thumb_time", "strong.time", ".thumb_time",
		// AnyPorn / generic tube sites
		".movie-duration", ".clip-time", ".playtime",
		"span.time", "span.length", "div.time", "div.duration",
		// data attribute as element
		"[data-duration]", "[data-seconds]",
	}
	for _, sel := range durSelectors {
		if d := s.Find(sel).First(); d.Length() > 0 {
			// Try data attributes first (used by some sites)
			durText := parser.ExtractAttr(d, "data-content", "data-duration", "data-seconds")
			if durText == "" {
				durText = parser.CleanText(d.Text())
			}
			if durText != "" {
				r.Duration, r.DurationSeconds = parser.ParseDuration(durText)
				break
			}
		}
	}
	// Also check data attributes on the container element itself
	if r.DurationSeconds == 0 {
		for _, attr := range []string{"data-duration", "data-seconds", "data-length"} {
			if dur := parser.ExtractAttr(s, attr); dur != "" {
				r.Duration, r.DurationSeconds = parser.ParseDuration(dur)
				if r.DurationSeconds > 0 {
					break
				}
			}
		}
	}

	// Find views - expanded selectors
	viewsSelectors := []string{
		".views", ".view", ".cnt", "span.views", ".video-views",
		".video__views", ".thumb__views", ".meta-views", ".stats",
		".view-count", ".viewCount", ".video-count", ".added-views",
	}
	for _, sel := range viewsSelectors {
		if v := s.Find(sel).First(); v.Length() > 0 {
			viewsText := parser.CleanText(v.Text())
			if viewsText != "" {
				r.Views, r.ViewsCount = parser.ParseViews(viewsText)
				break
			}
		}
	}

	// Find rating - common selectors
	ratingSelectors := []string{".rating", ".rate", ".video-rating", ".thumb__rating", ".score", ".likes", ".percent"}
	for _, sel := range ratingSelectors {
		if rt := s.Find(sel).First(); rt.Length() > 0 {
			ratingText := parser.CleanText(rt.Text())
			if ratingText != "" {
				_, rating := parser.ParseRating(ratingText)
				if rating > 0 {
					r.Rating = rating
					break
				}
			}
		}
	}

	// Check for quality
	quality := parser.ExtractQuality(s)
	if quality != "" {
		r.Quality = quality
	}

	// Extract tags/categories - common patterns across sites
	r.Tags = extractTags(s)

	// Extract performer if available
	r.Performer = extractPerformer(s)

	// Extract published/upload date - common patterns
	r.Published = extractPublished(s)

	r.Source = sourceName
	r.SourceDisplay = sourceDisplay
	r.ID = GenerateResultID(r.URL, sourceName)

	return r
}

// extractTags extracts tags/categories from video card elements
func extractTags(s *goquery.Selection) []string {
	var tags []string
	seen := make(map[string]bool)

	addTag := func(tag string) {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag != "" && len(tag) > 1 && len(tag) < 50 && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}

	// Common tag selectors
	tagSelectors := []string{
		".tags a", ".tag a", ".categories a", ".category a",
		"a.tag", "a.category", ".video-tags a", ".video-categories a",
		".thumb-tags a", ".card-tags a", "[data-tags]", ".keywords a",
		".labels a", ".label", ".badge", ".chip",
	}

	for _, sel := range tagSelectors {
		s.Find(sel).Each(func(i int, el *goquery.Selection) {
			text := parser.CleanText(el.Text())
			addTag(text)
		})
	}

	// Check data attributes for tags
	if dataTags, exists := s.Attr("data-tags"); exists {
		for _, tag := range strings.Split(dataTags, ",") {
			addTag(tag)
		}
	}

	// Check for category data attribute
	if dataCat, exists := s.Attr("data-category"); exists {
		addTag(dataCat)
	}
	if dataCats, exists := s.Attr("data-categories"); exists {
		for _, cat := range strings.Split(dataCats, ",") {
			addTag(cat)
		}
	}

	return tags
}

// extractPerformer extracts performer/model name from video card
func extractPerformer(s *goquery.Selection) string {
	// Common performer selectors
	performerSelectors := []string{
		".pornstar", ".model", ".performer", ".actor", ".actress",
		".uploader", ".author", ".channel", ".studio",
		"a.pornstar", "a.model", ".video-pornstar", ".video-model",
		"[data-pornstar]", "[data-model]", "[data-performer]",
	}

	for _, sel := range performerSelectors {
		if el := s.Find(sel).First(); el.Length() > 0 {
			text := parser.CleanText(el.Text())
			if text != "" {
				return text
			}
		}
	}

	// Check data attributes
	if performer, exists := s.Attr("data-pornstar"); exists && performer != "" {
		return performer
	}
	if performer, exists := s.Attr("data-model"); exists && performer != "" {
		return performer
	}

	return ""
}

// extractPublished looks for a publish/upload date on a video card using
// common selectors and data attributes. Best-effort: returns the zero
// time.Time when no recognizable date is found.
func extractPublished(s *goquery.Selection) time.Time {
	dateSelectors := []string{
		"time[datetime]", ".date", ".added", ".added-date", ".upload-date",
		".video-date", ".publish-date", ".added_at", ".video-added",
	}
	for _, sel := range dateSelectors {
		d := s.Find(sel).First()
		if d.Length() == 0 {
			continue
		}
		dateText := parser.ExtractAttr(d, "datetime", "data-date")
		if dateText == "" {
			dateText = parser.CleanText(d.Text())
		}
		if t := parsePublishedDate(dateText); !t.IsZero() {
			return t
		}
	}

	for _, attr := range []string{"data-upload-date", "data-added", "data-date", "data-published"} {
		if v := parser.ExtractAttr(s, attr); v != "" {
			if t := parsePublishedDate(v); !t.IsZero() {
				return t
			}
		}
	}

	return time.Time{}
}

// publishedDateLayouts are the date/time formats attempted by
// parsePublishedDate, in order of preference.
var publishedDateLayouts = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"01/02/2006",
	"January 2, 2006",
	"Jan 2, 2006",
	"2 January 2006",
}

// parsePublishedDate attempts to parse a free-form date string against a
// list of common layouts. Best-effort: returns the zero time.Time on
// failure rather than an error, since callers treat this as optional data.
func parsePublishedDate(text string) time.Time {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}
	}
	for _, layout := range publishedDateLayouts {
		if t, err := time.Parse(layout, text); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// iso8601DurationRe matches schema.org/ISO 8601 durations of the form
// "PT1H2M3S" (hours/minutes/seconds, all optional).
var iso8601DurationRe = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// parseISO8601Duration converts an ISO 8601 duration string into seconds.
// Returns 0 for empty or unrecognized input.
func parseISO8601Duration(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	m := iso8601DurationRe.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	hours, _ := strconv.Atoi(m[1])
	minutes, _ := strconv.Atoi(m[2])
	seconds, _ := strconv.Atoi(m[3])
	return hours*3600 + minutes*60 + seconds
}

// ldVideoInfo holds the best-effort fields extracted from a single
// schema.org VideoObject JSON-LD node.
type ldVideoInfo struct {
	description string
	thumbnail   string
	uploadDate  time.Time
	duration    int
	tags        []string
	performer   string
	rating      float64
	viewCount   int64
}

// parseJSONLDVideos scans every `<script type="application/ld+json">` block
// in the document for schema.org VideoObject data (directly, wrapped in an
// ItemList, or under "@graph") and returns a map keyed by the video's
// absolute URL. Best-effort: sites that don't emit JSON-LD simply yield an
// empty map, never an error.
func parseJSONLDVideos(doc *goquery.Document, baseURL string) map[string]ldVideoInfo {
	out := make(map[string]ldVideoInfo)
	doc.Find(`script[type="application/ld+json"]`).Each(func(i int, sel *goquery.Selection) {
		raw := strings.TrimSpace(sel.Text())
		if raw == "" {
			return
		}
		for _, node := range decodeLDNodes([]byte(raw)) {
			collectLDVideoObjects(node, baseURL, out)
		}
	})
	return out
}

// decodeLDNodes normalizes a JSON-LD script's contents into a flat list of
// top-level nodes, handling a bare object, an array of objects, and the
// "@graph" wrapper form. Malformed JSON yields an empty (non-nil-panicking)
// result rather than an error.
func decodeLDNodes(data []byte) []map[string]interface{} {
	var single map[string]interface{}
	if err := json.Unmarshal(data, &single); err == nil {
		if graph, ok := single["@graph"].([]interface{}); ok {
			var nodes []map[string]interface{}
			for _, g := range graph {
				if m, ok := g.(map[string]interface{}); ok {
					nodes = append(nodes, m)
				}
			}
			return nodes
		}
		return []map[string]interface{}{single}
	}

	var list []map[string]interface{}
	if err := json.Unmarshal(data, &list); err == nil {
		return list
	}

	return nil
}

// collectLDVideoObjects walks a decoded JSON-LD node, recursing into
// ItemList/itemListElement wrappers, and records any VideoObject found into
// out, keyed by every URL-like field it exposes (contentUrl, embedUrl, url,
// @id) so callers can match against whatever URL form they built.
func collectLDVideoObjects(node map[string]interface{}, baseURL string, out map[string]ldVideoInfo) {
	if node == nil {
		return
	}

	typ, _ := node["@type"].(string)
	if typ == "ItemList" {
		elems, _ := node["itemListElement"].([]interface{})
		for _, el := range elems {
			em, ok := el.(map[string]interface{})
			if !ok {
				continue
			}
			if item, ok := em["item"].(map[string]interface{}); ok {
				collectLDVideoObjects(item, baseURL, out)
			} else {
				collectLDVideoObjects(em, baseURL, out)
			}
		}
		return
	}

	if typ != "VideoObject" {
		return
	}

	info := ldVideoInfo{}
	info.description, _ = node["description"].(string)
	info.thumbnail = ldFirstString(node["thumbnailUrl"])
	if uploadDate, ok := node["uploadDate"].(string); ok {
		info.uploadDate = parsePublishedDate(uploadDate)
	}
	if duration, ok := node["duration"].(string); ok {
		info.duration = parseISO8601Duration(duration)
	}
	info.tags = ldStringList(node["keywords"])
	if len(info.tags) == 0 {
		info.tags = ldStringList(node["genre"])
	}
	info.performer = ldActorName(node["actor"])
	if rating, ok := node["aggregateRating"].(map[string]interface{}); ok {
		info.rating = ldFloat(rating["ratingValue"])
	}
	info.viewCount = ldInteractionCount(node["interactionCount"])

	for _, key := range []string{"contentUrl", "embedUrl", "url", "@id"} {
		v, _ := node[key].(string)
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out[parser.MakeAbsoluteURL(v, baseURL)] = info
	}
}

// mergeLDVideoInfo fills any still-empty fields on r from ld. Never
// overwrites a value already extracted from the HTML card.
func mergeLDVideoInfo(r *model.VideoResult, ld ldVideoInfo) {
	if r.Description == "" {
		r.Description = ld.description
	}
	if r.Thumbnail == "" && ld.thumbnail != "" {
		r.Thumbnail = ld.thumbnail
	}
	if r.Published.IsZero() && !ld.uploadDate.IsZero() {
		r.Published = ld.uploadDate
	}
	if r.DurationSeconds == 0 && ld.duration > 0 {
		r.DurationSeconds = ld.duration
		r.Duration = formatDuration(ld.duration)
	}
	if len(r.Tags) == 0 && len(ld.tags) > 0 {
		r.Tags = ld.tags
	}
	if r.Performer == "" && ld.performer != "" {
		r.Performer = ld.performer
	}
	if r.Rating == 0 && ld.rating > 0 {
		r.Rating = ld.rating
	}
	if r.ViewsCount == 0 && ld.viewCount > 0 {
		r.ViewsCount = ld.viewCount
		r.Views = formatViewCount(int(ld.viewCount))
	}
}

// ldFirstString returns the first non-empty string from a JSON-LD field
// that may be either a bare string or an array of strings.
func ldFirstString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case []interface{}:
		for _, item := range val {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// ldStringList normalizes a JSON-LD field that may be a comma-separated
// string or an array of strings into a lowercase string slice.
func ldStringList(v interface{}) []string {
	var out []string
	switch val := v.(type) {
	case string:
		for _, part := range strings.Split(val, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part != "" {
				out = append(out, part)
			}
		}
	case []interface{}:
		for _, item := range val {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(strings.ToLower(s))
				if s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// ldFloat coerces a JSON-LD numeric field (which may be encoded as either
// a JSON number or a string) into a float64.
func ldFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			return f
		}
	}
	return 0
}

// ldActorName extracts a performer name from a JSON-LD "actor" field, which
// may be a Person object, an array of Person objects, or a bare string.
func ldActorName(v interface{}) string {
	switch val := v.(type) {
	case map[string]interface{}:
		if name, ok := val["name"].(string); ok {
			return strings.TrimSpace(name)
		}
	case []interface{}:
		for _, item := range val {
			if m, ok := item.(map[string]interface{}); ok {
				if name, ok := m["name"].(string); ok && strings.TrimSpace(name) != "" {
					return strings.TrimSpace(name)
				}
			}
		}
	case string:
		return strings.TrimSpace(val)
	}
	return ""
}

// ldInteractionCount extracts a view/interaction count from a JSON-LD
// "interactionCount" field, which may be a bare number, a string (plain or
// in the "https://schema.org/WatchAction/1234" form), or a nested
// InteractionCounter object exposing "userInteractionCount".
func ldInteractionCount(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		parts := strings.Split(val, "/")
		last := strings.TrimSpace(parts[len(parts)-1])
		if n, err := strconv.ParseInt(last, 10, 64); err == nil {
			return n
		}
	case map[string]interface{}:
		return ldInteractionCount(val["userInteractionCount"])
	}
	return 0
}
