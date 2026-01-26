// SPDX-License-Identifier: MIT
package parser

import (
	"github.com/PuerkitoBio/goquery"
)

// PornHubParser handles PornHub HTML parsing
type PornHubParser struct {
	BaseURL string
}

// NewPornHubParser creates a new PornHub parser
func NewPornHubParser() *PornHubParser {
	return &PornHubParser{BaseURL: "https://www.pornhub.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *PornHubParser) ItemSelector() string {
	return "li.videoBox, li.pcVideoListItem, div.phimage"
}

// Parse extracts video data from a selection
func (p *PornHubParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link - try multiple selectors
	link := s.Find("a.linkVideoThumb, a.videoPreviewBg, a").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Get title from multiple sources
	titleElem := s.Find("span.title a, a[title]")
	item.Title = ExtractAttr(titleElem, "title")
	if item.Title == "" {
		item.Title = CleanText(titleElem.Text())
	}
	if item.Title == "" {
		item.Title = ExtractAttr(link, "title")
	}
	if item.Title == "" {
		return nil
	}

	// Get thumbnail
	img := s.Find("img").First()
	item.Thumbnail = ExtractAttr(img, "data-thumb_url", "data-src", "data-mediumthumb", "src")
	if item.Thumbnail != "" {
		item.Thumbnail = MakeAbsoluteURL(item.Thumbnail, "https:")
	}

	// Get video preview URL from data-mediabook attribute
	item.PreviewURL = ExtractAttr(img, "data-mediabook")
	if item.PreviewURL == "" {
		item.PreviewURL = ExtractAttr(link, "data-mediabook")
	}

	// Try to extract download URL from data attributes (if available in search results)
	item.DownloadURL = ExtractAttr(s, "data-video-url", "data-download", "data-mp4")
	if item.DownloadURL == "" {
		item.DownloadURL = ExtractAttr(link, "data-video-url", "data-download")
	}
	if item.DownloadURL != "" {
		item.DownloadURL = MakeAbsoluteURL(item.DownloadURL, "https:")
	}

	// Get duration
	durElem := s.Find("var.duration, .duration, .time")
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Get views
	viewsElem := s.Find("var.views, .views, span.views")
	item.Views, item.ViewsCount = ParseViews(CleanText(viewsElem.Text()))

	// Get quality
	item.Quality = ExtractQuality(s)

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	// Extract tags (if available in search results)
	item.Tags = ExtractTags(s, ".tags a", ".video-tag", ".category a", "a[href*='/categories/']", "a[href*='/tags/']")

	// Extract uploader/performer
	item.Uploader = ExtractUploader(s, ".usernameWrap a", ".uploader a", ".channel a", ".pornstar a", "a[href*='/model/']", "a[href*='/pornstar/']")

	return item
}
