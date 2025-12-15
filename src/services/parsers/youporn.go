// SPDX-License-Identifier: MIT
package parsers

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// YouPornParser handles YouPorn HTML parsing
type YouPornParser struct {
	BaseURL string
}

// NewYouPornParser creates a new YouPorn parser
func NewYouPornParser() *YouPornParser {
	return &YouPornParser{BaseURL: "https://www.youporn.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *YouPornParser) ItemSelector() string {
	return "div.video-box, li.video-box, .thumbnail-card, .js_video-box"
}

// Parse extracts video data from a selection
func (p *YouPornParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Link selectors for new site structure
	link := s.Find("a.video-box-image, a.js_video-box-url, a.tm_video_link, a").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	if !strings.HasPrefix(href, "http") {
		href = p.BaseURL + href
	}
	item.URL = href

	// Try multiple title selectors - video-title-text is the main one
	titleElem := s.Find(".video-title-text, a.video-box-title, span.video-box-title, .title")
	item.Title = CleanText(titleElem.Text())
	if item.Title == "" {
		item.Title = ExtractAttr(titleElem, "title")
	}
	if item.Title == "" {
		item.Title = ExtractAttr(link, "title")
	}
	if item.Title == "" {
		return nil
	}

	// Thumbnail
	img := s.Find("img").First()
	item.Thumbnail = ExtractAttr(img, "data-src", "src")
	if item.Thumbnail != "" && !strings.HasPrefix(item.Thumbnail, "http") {
		item.Thumbnail = "https:" + item.Thumbnail
	}

	// Get video preview URL from data-mediabook attribute
	item.PreviewURL = ExtractAttr(img, "data-mediabook")
	if item.PreviewURL != "" {
		item.PreviewURL = strings.ReplaceAll(item.PreviewURL, "&amp;", "&")
	}

	// Duration
	durElem := s.Find(".video-duration, .tm_video_duration, .duration")
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Views
	viewsElem := s.Find(".video-views, .views")
	item.Views, item.ViewsCount = ParseViews(CleanText(viewsElem.Text()))

	// Quality
	item.Quality = ExtractQuality(s)

	// Premium check
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
