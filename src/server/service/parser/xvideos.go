// SPDX-License-Identifier: MIT
package parser

import (
	"github.com/PuerkitoBio/goquery"
)

// XVideosParser handles XVideos HTML parsing
type XVideosParser struct {
	BaseURL string
}

// NewXVideosParser creates a new XVideos parser
func NewXVideosParser() *XVideosParser {
	return &XVideosParser{BaseURL: "https://www.xvideos.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *XVideosParser) ItemSelector() string {
	return "div.thumb-block, div.mozaique div.thumb"
}

// Parse extracts video data from a selection
func (p *XVideosParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link
	link := s.Find("a").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Get title
	titleElem := s.Find("p.title a, a[title]")
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
	item.Thumbnail = ExtractAttr(img, "data-src", "src")
	if item.Thumbnail != "" {
		item.Thumbnail = MakeAbsoluteURL(item.Thumbnail, "https:")
	}

	// Get preview video URL (XVideos stores this in data-preview on img or parent)
	item.PreviewURL = ExtractAttr(img, "data-preview")
	if item.PreviewURL == "" {
		// Try parent element
		item.PreviewURL = ExtractAttr(s.Find(".thumb-inside, .thumb").First(), "data-preview")
	}
	if item.PreviewURL != "" {
		item.PreviewURL = MakeAbsoluteURL(item.PreviewURL, "https:")
	}

	// Get duration
	durElem := s.Find(".duration, span.duration")
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Get views
	viewsElem := s.Find(".metadata span.views, .views")
	item.Views, item.ViewsCount = ParseViews(CleanText(viewsElem.Text()))

	// Get quality
	item.Quality = ExtractQuality(s)

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
