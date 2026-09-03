// SPDX-License-Identifier: MIT
package parser

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// PornMDParser handles PornMD HTML parsing
type PornMDParser struct {
	BaseURL string
}

// NewPornMDParser creates a new PornMD parser
func NewPornMDParser() *PornMDParser {
	return &PornMDParser{BaseURL: "https://www.pornmd.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *PornMDParser) ItemSelector() string {
	return "div.card.sub"
}

// Parse extracts video data from a selection
func (p *PornMDParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link - PornMD uses <a class="item-link"> with title attribute
	link := s.Find("a.item-link, a.rate-link").First()
	if link.Length() == 0 {
		link = s.Find("a").First()
	}
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	// PornMD is a meta-search but URLs may be relative redirects
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Get title from link's title attribute
	item.Title = ExtractAttr(link, "title")
	if item.Title == "" {
		// Try alt text from image
		img := s.Find("img").First()
		item.Title = ExtractAttr(img, "alt")
	}
	if item.Title == "" {
		return nil
	}

	// Get thumbnail - look for img.item-image or any img
	img := s.Find("img.item-image").First()
	if img.Length() == 0 {
		img = s.Find("img").First()
	}
	item.Thumbnail = ExtractAttr(img, "data-src", "src")
	if item.Thumbnail != "" {
		item.Thumbnail = MakeAbsoluteURL(item.Thumbnail, "https:")
	}

	// Try to extract download URL from data attributes (if available in search results)
	item.DownloadURL = ExtractAttr(img, "data-video-url", "data-download")
	if item.DownloadURL == "" {
		item.DownloadURL = ExtractAttr(link, "data-video-url", "data-download")
	}
	if item.DownloadURL != "" {
		item.DownloadURL = MakeAbsoluteURL(item.DownloadURL, "https:")
	}

	// Get duration from .badge.float-right
	// Structure: <span class="badge float-right">
	//   <span class="font-bold italic">HD</span>
	//   4:00  <!-- duration is in the parent span text -->
	// </span>
	durBadge := s.Find(".item-meta-container .badge.float-right").First()
	if durBadge.Length() > 0 {
		// Get all text from the badge (includes "HD" and duration)
		fullText := CleanText(durBadge.Text())
		// Try to parse duration from the text (ParseDuration handles "HD 4:00" or just "4:00")
		item.Duration, item.DurationSeconds = ParseDuration(fullText)
	}

	// Get rating from .item-score span (e.g., "88%")
	ratingSpan := s.Find(".item-score").First()
	if ratingSpan.Length() > 0 {
		ratingText := CleanText(ratingSpan.Text())
		if strings.HasSuffix(ratingText, "%") {
			item.Rating = ratingText
		}
	}

	// Get source info from .item-source
	sourceElem := s.Find(".item-source").First()
	sourceText := CleanText(sourceElem.Text())
	if sourceText != "" {
		item.Description = "Source: " + sourceText
	}

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	// Extract tags (if available in search results)
	item.Tags = ExtractTags(s, ".item-tags a", ".tags a", "a[href*='/tag/']", "a[href*='/category/']")

	// Extract uploader/performer
	item.Uploader = ExtractUploader(s, ".item-source a", ".uploader a")

	return item
}
