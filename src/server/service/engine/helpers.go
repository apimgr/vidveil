// SPDX-License-Identifier: MIT
package engine

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
)

// genericSearch performs a generic search using common patterns
func genericSearch(ctx context.Context, e *BaseEngine, url, selector string) ([]model.VideoResult, error) {
	resp, err := e.MakeRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read body for debug logging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Log raw response when debug is enabled
	DebugLogEngineResponse(e.Name(), url, body)

	// Parse HTML from body
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var results []model.VideoResult
	fieldStats := map[string]int{
		"preview": 0,
		"thumb":   0,
		"quality": 0,
	}
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		r := parseGenericVideoItem(s, e.baseURL, e.Name(), e.DisplayName())
		if r.Title != "" && r.URL != "" {
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
	DebugLogEngineParseResult(e.Name(), len(results), fieldStats)

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

	// Find preview URL - common data attributes for video preview/rollover
	previewAttrs := []string{
		"data-mediabook", "data-preview", "data-video-preview", "data-rollover",
		"data-preview-url", "data-gif", "data-webm", "data-mp4",
		"data-thumb-url", "data-trailer", "data-teaser",
	}
	// Check on the container element
	for _, attr := range previewAttrs {
		if preview := parser.ExtractAttr(s, attr); preview != "" {
			if !strings.HasPrefix(preview, "http") {
				if strings.HasPrefix(preview, "//") {
					preview = "https:" + preview
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
					}
				}
				r.PreviewURL = preview
				break
			}
		}
	}

	// Find duration - try multiple selectors and also data attributes
	durSelectors := []string{
		".duration", ".dur", ".time", ".length", ".video-duration",
		"var.duration", "span.duration", ".thumb-icon.video-duration",
		"em.time_thumb em", ".time_thumb", ".video_duration", ".video__time",
		".thumb__time", ".thumb-time", ".thumb-duration", ".video-time",
		"time", "[data-duration]", ".meta-duration", ".card-duration",
	}
	for _, sel := range durSelectors {
		if d := s.Find(sel).First(); d.Length() > 0 {
			// Try data-content attribute first (used by some sites)
			durText := parser.ExtractAttr(d, "data-content", "data-duration")
			if durText == "" {
				durText = parser.CleanText(d.Text())
			}
			if durText != "" {
				r.Duration, r.DurationSeconds = parser.ParseDuration(durText)
				break
			}
		}
	}
	// Also check data attributes on the element itself
	if r.DurationSeconds == 0 {
		if dur := parser.ExtractAttr(s, "data-duration"); dur != "" {
			r.Duration, r.DurationSeconds = parser.ParseDuration(dur)
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
