// SPDX-License-Identifier: MIT
package models

import (
	"time"
)

// Result represents a single video search result
type Result struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	URL             string    `json:"url"`
	Thumbnail       string    `json:"thumbnail"`
	PreviewURL      string    `json:"preview_url,omitempty"`
	Duration        string    `json:"duration"`
	DurationSeconds int       `json:"duration_seconds"`
	Views           string    `json:"views"`
	ViewsCount      int64     `json:"views_count"`
	Rating          float64   `json:"rating,omitempty"`
	Source          string    `json:"source"`
	SourceDisplay   string    `json:"source_display"`
	Published       time.Time `json:"published,omitempty"`
	Description     string    `json:"description,omitempty"`
}

// SearchResponse represents the API response for a search
type SearchResponse struct {
	Success    bool             `json:"success"`
	Data       SearchData       `json:"data"`
	Pagination PaginationData   `json:"pagination"`
	Error      string           `json:"error,omitempty"`
	Code       string           `json:"code,omitempty"`
}

// SearchData holds the search results and metadata
type SearchData struct {
	Query         string   `json:"query"`
	Results       []Result `json:"results"`
	EnginesUsed   []string `json:"engines_used"`
	EnginesFailed []string `json:"engines_failed"`
	SearchTimeMS  int64    `json:"search_time_ms"`
}

// PaginationData holds pagination information
type PaginationData struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
	Pages int `json:"pages"`
}

// EngineInfo represents information about a search engine
type EngineInfo struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Enabled     bool     `json:"enabled"`
	Available   bool     `json:"available"`
	Features    []string `json:"features"`
	Tier        int      `json:"tier"`
}

// EnginesResponse represents the API response for engines list
type EnginesResponse struct {
	Success bool         `json:"success"`
	Data    []EngineInfo `json:"data"`
}
