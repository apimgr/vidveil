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

// RedTube API response structures
type redTubeAPIResponse struct {
	Videos []redTubeVideoWrapper `json:"videos"`
}

type redTubeVideoWrapper struct {
	Video redTubeVideo `json:"video"`
}

type redTubeVideo struct {
	Duration  string `json:"duration"`
	Views     int64  `json:"views"`
	VideoID   string `json:"video_id"`
	Rating    string `json:"rating"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Thumb     string `json:"thumb"`
}

// RedTubeEngine implements the RedTube search engine using their public API
type RedTubeEngine struct {
	*BaseEngine
}

// NewRedTubeEngine creates a new RedTube engine
func NewRedTubeEngine(cfg *config.Config, torClient *tor.Client) *RedTubeEngine {
	return &RedTubeEngine{
		BaseEngine: NewBaseEngine("redtube", "RedTube", "https://www.redtube.com", 1, cfg, torClient),
	}
}

// Search performs a search on RedTube using their public API
func (e *RedTubeEngine) Search(ctx context.Context, query string, page int) ([]model.Result, error) {
	// RedTube public API
	apiURL := fmt.Sprintf("https://api.redtube.com/?data=redtube.Videos.searchVideos&output=json&search=%s&thumbsize=medium&page=%d",
		url.QueryEscape(query), page)

	resp, err := e.MakeRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp redTubeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	var results []model.Result
	for _, vw := range apiResp.Videos {
		v := vw.Video
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

// SupportsFeature checks if RedTube supports a specific feature
func (e *RedTubeEngine) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeaturePagination:
		return true
	case FeatureSorting:
		return true
	default:
		return false
	}
}
