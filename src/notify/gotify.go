// SPDX-License-Identifier: MIT
// AI.md PART 12: Gotify notification adapter.
// Format: POST {url}/message?token={token} body {"title", "message", "priority"}
package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type gotifyPayload struct {
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
}

// formatGotify adapts payload to the Gotify API format.
// The URL should be the Gotify server base URL; token is appended as a query param.
// e.g.: https://gotify.example.com — token provided via "token" query param on the URL
func formatGotify(rawURL string, p Payload) (body []byte, contentType, targetURL string, err error) {
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return nil, "", "", fmt.Errorf("gotify: invalid URL: %w", parseErr)
	}

	// Construct the /message endpoint path.
	base := strings.TrimRight(parsed.Scheme+"://"+parsed.Host+parsed.Path, "/")
	q := parsed.Query()
	targetURL = base + "/message?" + q.Encode()

	priority := 5
	if p.Severity == SeverityCritical {
		priority = 10
	} else if p.Severity == SeverityWarning {
		priority = 7
	}

	gp := gotifyPayload{
		Title:    fmt.Sprintf("[%s] %s", p.ProjectName, p.Subject),
		Message:  p.Body,
		Priority: priority,
	}
	b, mErr := json.Marshal(gp)
	if mErr != nil {
		return nil, "", "", fmt.Errorf("gotify: marshal: %w", mErr)
	}
	return b, "application/json", targetURL, nil
}
