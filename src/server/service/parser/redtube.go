// SPDX-License-Identifier: MIT
package parser

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// RedTubeParser handles RedTube HTML parsing
type RedTubeParser struct {
	BaseURL string
}

// NewRedTubeParser creates a new RedTube parser
func NewRedTubeParser() *RedTubeParser {
	return &RedTubeParser{BaseURL: "https://www.redtube.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *RedTubeParser) ItemSelector() string {
	return "li.videoblock_list, li.thumbnail-card, li.videoblock-default, li.video-box"
}

// Parse extracts video data from a selection
func (p *RedTubeParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Title is in a.video-title-text (not the thumbnail link)
	titleElem := s.Find("a.video-title-text, a.tm_video_title")
	item.Title = ExtractAttr(titleElem, "title")
	if item.Title == "" {
		item.Title = CleanText(titleElem.Text())
	}
	// No title = skip this item
	if item.Title == "" {
		return nil
	}

	// URL from title link or thumbnail link
	href := ExtractAttr(titleElem, "href")
	if href == "" {
		link := s.Find("a.video_link, a.tm_video_link, a").First()
		href = ExtractAttr(link, "href")
	}
	if href == "" {
		return nil
	}
	if !strings.HasPrefix(href, "http") {
		href = p.BaseURL + href
	}
	item.URL = href

	// Thumbnail from img.js_thumbImageTag or similar
	img := s.Find("img.js_thumbImageTag, img.thumb, img").First()
	item.Thumbnail = ExtractAttr(img, "data-src", "data-srcset", "src")
	if item.Thumbnail != "" && !strings.HasPrefix(item.Thumbnail, "http") && !strings.HasPrefix(item.Thumbnail, "data:") {
		item.Thumbnail = "https:" + item.Thumbnail
	}

	// Get video preview URL from data-mediabook attribute (same as PornHub)
	item.PreviewURL = ExtractAttr(img, "data-mediabook")
	if item.PreviewURL != "" {
		item.PreviewURL = strings.ReplaceAll(item.PreviewURL, "&amp;", "&")
	}

	// Duration in .video-properties or .tm_video_duration
	durElem := s.Find(".video-properties, .tm_video_duration, .duration span")
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Views in .info-views
	viewsElem := s.Find(".info-views")
	item.Views, item.ViewsCount = ParseViews(CleanText(viewsElem.First().Text()))

	// Get quality
	item.Quality = ExtractQuality(s)

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
