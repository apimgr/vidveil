// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/parser"
)

// genericSearch performs a generic search using common patterns
func genericSearch(ctx context.Context, e *BaseEngine, url, selector string) ([]model.Result, error) {
	resp, err := e.MakeRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []model.Result
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		r := parseGenericVideoItem(s, e.baseURL, e.Name(), e.DisplayName())
		if r.Title != "" && r.URL != "" {
			results = append(results, r)
		}
	})
	return results, nil
}

// parseGenericVideoItem extracts video data using common patterns
func parseGenericVideoItem(s *goquery.Selection, baseURL, sourceName, sourceDisplay string) model.Result {
	var r model.Result

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
		r.Description = quality
	}

	r.Source = sourceName
	r.SourceDisplay = sourceDisplay
	r.ID = GenerateResultID(r.URL, sourceName)

	return r
}
