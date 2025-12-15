// SPDX-License-Identifier: MIT
package parsers

import (
	"github.com/PuerkitoBio/goquery"
)

// SpankBangParser handles SpankBang HTML parsing
type SpankBangParser struct {
	BaseURL string
}

// NewSpankBangParser creates a new SpankBang parser
func NewSpankBangParser() *SpankBangParser {
	return &SpankBangParser{BaseURL: "https://spankbang.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *SpankBangParser) ItemSelector() string {
	return "div.video-item, div.video-list div.thumb"
}

// Parse extracts video data from a selection
func (p *SpankBangParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link
	link := s.Find("a.thumb, a").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Get title
	titleElem := s.Find("a.n, .video-title, .title")
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
	img := s.Find("img, picture source").First()
	item.Thumbnail = ExtractAttr(img, "data-src", "srcset", "src")
	if item.Thumbnail != "" {
		item.Thumbnail = MakeAbsoluteURL(item.Thumbnail, "https:")
	}

	// Get duration
	durElem := s.Find(".l, .duration")
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Get views
	viewsElem := s.Find(".v, .views")
	item.Views, item.ViewsCount = ParseViews(CleanText(viewsElem.Text()))

	// Get quality
	item.Quality = ExtractQuality(s)

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
