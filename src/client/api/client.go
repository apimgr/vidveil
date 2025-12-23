// SPDX-License-Identifier: MIT
// TEMPLATE.md PART 33: CLI Client - API Client
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is the API client for VidVeil
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	userAgent  string
}

// SearchResult represents a single search result
type SearchResult struct {
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Thumbnail   string   `json:"thumbnail"`
	Duration    string   `json:"duration"`
	Views       string   `json:"views"`
	Engine      string   `json:"engine"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// SearchResponse is the API response for search
type SearchResponse struct {
	Success bool           `json:"success"`
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Count   int            `json:"count"`
	Error   string         `json:"error,omitempty"`
}

// VersionResponse is the API response for version
type VersionResponse struct {
	Success bool   `json:"success"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Built   string `json:"built"`
}

// NewClient creates a new API client
func NewClient(baseURL, token string, timeout int) *Client {
	if baseURL == "" {
		baseURL = "https://x.scour.li"
	}
	if timeout <= 0 {
		timeout = 30
	}

	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		// TEMPLATE.md PART 33: User-Agent always uses original project name
		userAgent: "vidveil-cli/dev",
	}
}

// SetUserAgent sets the user agent (called from main with version)
func (c *Client) SetUserAgent(version string) {
	c.userAgent = fmt.Sprintf("vidveil-cli/%s", version)
}

// Search performs a video search
func (c *Client) Search(query string, page, limit int, engines []string, safeSearch bool) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if len(engines) > 0 {
		for _, e := range engines {
			params.Add("engines", e)
		}
	}
	if safeSearch {
		params.Set("safe", "true")
	}

	url := fmt.Sprintf("%s/api/v1/search?%s", c.baseURL, params.Encode())

	var resp SearchResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetVersion gets server version info
func (c *Client) GetVersion() (*VersionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/version", c.baseURL)

	var resp VersionResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Health checks if the server is reachable
func (c *Client) Health() (bool, error) {
	url := fmt.Sprintf("%s/api/v1/healthz", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

// get performs a GET request and decodes JSON response
func (c *Client) get(url string, result interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to server at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}
