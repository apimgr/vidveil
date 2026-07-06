// SPDX-License-Identifier: MIT
// AI.md PART 12: Pushover notification adapter.
// Format: standard Pushover API params via JSON POST.
package notify

import (
	"encoding/json"
	"fmt"
)

type pushoverPayload struct {
	Token    string `json:"token"`
	User     string `json:"user"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
	Sound    string `json:"sound,omitempty"`
}

// formatPushover adapts payload to the Pushover API format.
// The URL must contain "token" and "user" query params, e.g.:
// https://api.pushover.net/1/messages.json?token={TOKEN}&user={USER}
func formatPushover(rawURL string, p Payload) (body []byte, contentType, targetURL string, err error) {
	priority := 0
	if p.Severity == SeverityCritical {
		priority = 1
	}

	pp := pushoverPayload{
		Title:    fmt.Sprintf("[%s] %s", p.ProjectName, p.Subject),
		Message:  p.Body,
		Priority: priority,
	}

	b, mErr := json.Marshal(pp)
	if mErr != nil {
		return nil, "", "", fmt.Errorf("pushover: marshal: %w", mErr)
	}
	return b, "application/json", rawURL, nil
}
