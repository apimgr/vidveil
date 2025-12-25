// SPDX-License-Identifier: MIT
package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/tor"
)

// PornHub API response structures
type pornHubAPIResponse struct {
	Videos []pornHubVideo `json:"videos"`
}

type pornHubVideo struct {
	Duration  string  `json:"duration"`
	Views     int64   `json:"views"`
	VideoID   string  `json:"video_id"`
	Rating    float64 `json:"rating"`
	Title     string  `json:"title"`
	URL       string  `json:"url"`
	Thumb     string  `json:"thumb"`
}

// PornHubEngine implements the PornHub search engine using their public API
type PornHubEngine struct {
	*BaseEngine
}

// NewPornHubEngine creates a new PornHub engine
func NewPornHubEngine(cfg *config.Config, torClient *tor.Client) *PornHubEngine {
	return &PornHubEngine{
		BaseEngine: NewBaseEngine("pornhub", "PornHub", "https://www.pornhub.com", 1, cfg, torClient),
	}
}

// Search performs a search on PornHub using their webmasters API
func (e *PornHubEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	// PornHub webmasters API
	apiURL := fmt.Sprintf("https://www.pornhub.com/webmasters/search?search=%s&thumbsize=medium&page=%d",
		url.QueryEscape(query), page)

	resp, err := e.MakeRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp pornHubAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	var results []model.Result
	for _, v := range apiResp.Videos {
		results = append(results, model.Result{
			ID:              GenerateResultID(v.URL, e.Name()),
			URL:             v.URL,
			Title:           v.Title,
			Thumbnail:       v.Thumb,
			Duration:        v.Duration,
			DurationSeconds: ParseDuration(v.Duration),
			Views:           formatViews(v.Views),
			ViewsCount:      v.Views,
			Source:          e.Name(),
			SourceDisplay:   e.DisplayName(),
		})
	}

	return results, nil
}

// SupportsFeature checks if PornHub supports a specific feature
func (e *PornHubEngine) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeaturePagination:
		return true
	case FeatureSorting:
		return true
	default:
		return false
	}
}

// formatViews formats view count to human readable string
func formatViews(views int64) string {
	if views >= 1000000 {
		return fmt.Sprintf("%.1fM views", float64(views)/1000000)
	}
	if views >= 1000 {
		return fmt.Sprintf("%.1fK views", float64(views)/1000)
	}
	return strconv.FormatInt(views, 10) + " views"
}
