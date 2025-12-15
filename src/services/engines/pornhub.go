// SPDX-License-Identifier: MIT
package engines

import (
	"context"

	"github.com/PuerkitoBio/goquery"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/parsers"
	"github.com/apimgr/vidveil/src/services/tor"
)

// PornHubEngine implements the PornHub search engine
// Based on yt-dlp extractor patterns
type PornHubEngine struct {
	*BaseEngine
	parser *parsers.PornHubParser
}

// NewPornHubEngine creates a new PornHub engine
func NewPornHubEngine(cfg *config.Config, torClient *tor.Client) *PornHubEngine {
	return &PornHubEngine{
		BaseEngine: NewBaseEngine("pornhub", "PornHub", "https://www.pornhub.com", 1, cfg, torClient),
		parser:     parsers.NewPornHubParser(),
	}
}

// getAgeCookies returns cookies required for age verification (from yt-dlp)
func (e *PornHubEngine) getAgeCookies() map[string]string {
	return map[string]string{
		"age_verified":          "1",
		"accessAgeDisclaimerPH": "1",
		"accessAgeDisclaimerUK": "1",
		"accessPH":              "1",
		"platform":              "pc",
	}
}

// Search performs a search on PornHub
func (e *PornHubEngine) Search(ctx context.Context, query string, page int) ([]models.Result, error) {
	searchURL := e.BuildSearchURL("/video/search?search={query}&page={page}", query, page)

	resp, err := e.MakeRequestWithMod(ctx, searchURL, AddCookies(e.getAgeCookies()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.Result

	doc.Find(e.parser.ItemSelector()).Each(func(i int, s *goquery.Selection) {
		item := e.parser.Parse(s)
		if item != nil && item.Title != "" && item.URL != "" && !item.IsPremium {
			results = append(results, e.convertToResult(item))
		}
	})

	return results, nil
}

// convertToResult converts VideoItem to models.Result
func (e *PornHubEngine) convertToResult(item *parsers.VideoItem) models.Result {
	return models.Result{
		ID:              GenerateResultID(item.URL, e.Name()),
		URL:             item.URL,
		Title:           item.Title,
		Thumbnail:       item.Thumbnail,
		PreviewURL:      item.PreviewURL,
		Duration:        item.Duration,
		DurationSeconds: item.DurationSeconds,
		Views:           item.Views,
		ViewsCount:      item.ViewsCount,
		Description:     item.Quality,
		Source:          e.Name(),
		SourceDisplay:   e.DisplayName(),
	}
}

// SupportsFeature checks if PornHub supports a specific feature
func (e *PornHubEngine) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeaturePagination:
		return true
	case FeatureThumbnailPreview:
		return true
	case FeatureSorting:
		return true
	default:
		return false
	}
}
