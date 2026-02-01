// SPDX-License-Identifier: MIT
package parser

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// MotherlessParser handles Motherless HTML parsing
type MotherlessParser struct {
	BaseURL string
}

// NewMotherlessParser creates a new Motherless parser
func NewMotherlessParser() *MotherlessParser {
	return &MotherlessParser{BaseURL: "https://motherless.com"}
}

// ItemSelector returns the CSS selector for video items
// Motherless uses div.thumb-container for video cards
func (p *MotherlessParser) ItemSelector() string {
	return "div.thumb-container, div.thumb"
}

// Parse extracts video data from a selection
// Motherless structure: <a href="/CODE"><img src="thumb" alt="title"/></a>
// Note: Motherless does NOT provide video preview URLs in search results
func (p *MotherlessParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link - the main anchor wrapping the thumbnail
	link := s.Find("a").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Get thumbnail - direct src (not lazy loaded)
	// Format: https://cdn5-thumbs.motherlessmedia.com/thumbs/CODE-small.jpg
	img := s.Find("img").First()
	item.Thumbnail = ExtractAttr(img, "src")

	// Skip placeholder images
	if strings.Contains(item.Thumbnail, "plc.gif") || item.Thumbnail == "" {
		// Try data-src as fallback
		item.Thumbnail = ExtractAttr(img, "data-src", "data-original")
	}

	if item.Thumbnail != "" {
		item.Thumbnail = MakeAbsoluteURL(item.Thumbnail, "https:")
	}

	// Get title from img alt or link title
	item.Title = ExtractAttr(img, "alt")
	if item.Title == "" {
		item.Title = ExtractAttr(link, "title")
	}
	if item.Title == "" {
		return nil
	}

	// Motherless does NOT provide preview URLs in search results
	// Leave PreviewURL empty

	// Get duration - plain text in the card
	// Look for common duration patterns
	durElem := s.Find(".duration, .dur, .time, .video-time")
	if durElem.Length() > 0 {
		item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.First().Text()))
	}

	// Also try finding duration in any span/small element with time format
	if item.DurationSeconds == 0 {
		s.Find("span, small, div").Each(func(i int, el *goquery.Selection) {
			text := CleanText(el.Text())
			// Check if text looks like duration (MM:SS or H:MM:SS)
			if len(text) >= 4 && len(text) <= 8 && strings.Contains(text, ":") {
				dur, secs := ParseDuration(text)
				if secs > 0 && item.DurationSeconds == 0 {
					item.Duration = dur
					item.DurationSeconds = secs
				}
			}
		})
	}

	// Get views - displayed as text (e.g., "14.7K")
	viewsElem := s.Find(".views, .view-count, .stats")
	if viewsElem.Length() > 0 {
		item.Views, item.ViewsCount = ParseViews(CleanText(viewsElem.First().Text()))
	}

	// Try to find views in text containing "K" or "M" or "views"
	if item.ViewsCount == 0 {
		s.Find("span, small, div").Each(func(i int, el *goquery.Selection) {
			text := CleanText(el.Text())
			textLower := strings.ToLower(text)
			if strings.Contains(textLower, "view") ||
			   (len(text) <= 10 && (strings.Contains(text, "K") || strings.Contains(text, "M"))) {
				views, count := ParseViews(text)
				if count > 0 && item.ViewsCount == 0 {
					item.Views = views
					item.ViewsCount = count
				}
			}
		})
	}

	// Get uploader - linked username
	// Format: <a href="/m/Username">
	uploaderLink := s.Find("a[href*='/m/'], a[href*='/u/']")
	if uploaderLink.Length() > 0 {
		item.Uploader = CleanText(uploaderLink.First().Text())
	}

	// Check for premium content
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
