// SPDX-License-Identifier: MIT
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Favorite is a private item saved by the authenticated anonymous owner.
type Favorite struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ListFavorites returns favorites belonging to the configured owner token.
func (c *APIClient) ListFavorites() ([]Favorite, error) {
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Favorites []Favorite `json:"favorites"`
		} `json:"data"`
	}
	if err := c.request(http.MethodGet, "/favorites", nil, &response); err != nil {
		return nil, err
	}
	if !response.OK {
		return nil, fmt.Errorf("server could not list favorites")
	}
	return response.Data.Favorites, nil
}

// AddFavorite saves a URL and title for the configured anonymous owner.
func (c *APIClient) AddFavorite(favorite Favorite) error {
	requestBody := struct {
		URL       string `json:"url"`
		Title     string `json:"title"`
		Thumbnail string `json:"thumbnail,omitempty"`
		Source    string `json:"source,omitempty"`
	}{URL: favorite.URL, Title: favorite.Title, Thumbnail: favorite.Thumbnail, Source: favorite.Source}
	var response struct {
		OK bool `json:"ok"`
	}
	if err := c.request(http.MethodPost, "/favorites", requestBody, &response); err != nil {
		return err
	}
	if !response.OK {
		return fmt.Errorf("server could not add favorite")
	}
	return nil
}

// RemoveFavorite deletes one favorite owned by the configured token.
func (c *APIClient) RemoveFavorite(id int64) error {
	var response struct {
		OK bool `json:"ok"`
	}
	if err := c.request(http.MethodDelete, fmt.Sprintf("/favorites/%d", id), nil, &response); err != nil {
		return err
	}
	if !response.OK {
		return fmt.Errorf("server could not remove favorite")
	}
	return nil
}

// ClearFavorites deletes all favorites owned by the configured token.
func (c *APIClient) ClearFavorites() error {
	var response struct {
		OK bool `json:"ok"`
	}
	if err := c.request(http.MethodDelete, "/favorites", nil, &response); err != nil {
		return err
	}
	if !response.OK {
		return fmt.Errorf("server could not clear favorites")
	}
	return nil
}

func (c *APIClient) request(method, endpoint string, payload, result interface{}) error {
	if endpoint == "" || !strings.HasPrefix(endpoint, "/") || strings.HasPrefix(endpoint, "//") || strings.Contains(endpoint, "..") {
		return fmt.Errorf("invalid API endpoint")
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodDelete:
	default:
		return fmt.Errorf("unsupported API method %q", method)
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encoding API request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	requestURL := c.GetAPIBaseURL() + endpoint
	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return fmt.Errorf("creating API request: %w", err)
	}
	c.applyCommonHeaders(req)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{BaseURL: c.baseURL, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading API response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &StatusError{StatusCode: resp.StatusCode, Body: string(responseBody)}
	}
	if result != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("decoding API response: %w", err)
		}
	}
	return nil
}
