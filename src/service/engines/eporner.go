// SPDX-License-Identifier: MIT
package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/model"
	"github.com/apimgr/vidveil/src/service/tor"
)

// EpornerEngine searches Eporner using their public JSON API
type EpornerEngine struct {
	*BaseEngine
}

// epornerAPIResponse represents the API response
type epornerAPIResponse struct {
	Count      int            `json:"count"`
	TotalCount int            `json:"total_count"`
	Videos     []epornerVideo `json:"videos"`
}

type epornerVideo struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Keywords     string                 `json:"keywords"`
	Views        int                    `json:"views"`
	Rate         string                 `json:"rate"`
	URL          string                 `json:"url"`
	Added        string                 `json:"added"`
	LengthSec    int                    `json:"length_sec"`
	LengthMin    string                 `json:"length_min"`
	DefaultThumb map[string]interface{} `json:"default_thumb"`
}

// NewEpornerEngine creates a new Eporner engine
func NewEpornerEngine(cfg *config.Config, torClient *tor.Client) *EpornerEngine {
	return &EpornerEngine{
		BaseEngine: NewBaseEngine("eporner", "Eporner", "https://www.eporner.com", 2, cfg, torClient),
	}
}

// Search performs a search on Eporner using their public JSON API
func (e *EpornerEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	// Use Eporner's public JSON API
	// API docs: https://www.eporner.com/api/
	perPage := 50
	// Use order=top-rated for search - returns best rated videos matching the query
	// Available options: latest, longest, shortest, top-rated, most-popular, top-weekly, top-monthly
	apiURL := fmt.Sprintf("%s/api/v2/video/search/?query=%s&per_page=%d&page=%d&thumbsize=big&order=top-rated&format=json",
		e.baseURL, url.QueryEscape(query), perPage, page)

	resp, err := e.MakeRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp epornerAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	var results []model.Result
	for _, v := range apiResp.Videos {
		// Get thumbnail URL
		thumb := ""
		if v.DefaultThumb != nil {
			if src, ok := v.DefaultThumb["src"].(string); ok {
				thumb = src
			}
		}

		// Format view count
		views := formatViewCount(v.Views)

		results = append(results, model.Result{
			ID:              GenerateResultID(v.URL, e.Name()),
			URL:             v.URL,
			Title:           v.Title,
			Thumbnail:       thumb,
			Duration:        v.LengthMin,
			DurationSeconds: v.LengthSec,
			Views:           views,
			ViewsCount:      int64(v.Views),
			Description:     v.Keywords,
			Source:          e.Name(),
			SourceDisplay:   e.DisplayName(),
		})
	}
	return results, nil
}

// SupportsFeature returns whether the engine supports a feature
func (e *EpornerEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination || feature == FeatureSorting
}

// formatViewCount formats view count for display
func formatViewCount(views int) string {
	if views >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(views)/1000000)
	} else if views >= 1000 {
		return fmt.Sprintf("%.1fK", float64(views)/1000)
	}
	return fmt.Sprintf("%d", views)
}
