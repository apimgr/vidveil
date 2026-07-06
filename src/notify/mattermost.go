// SPDX-License-Identifier: MIT
// AI.md PART 12: Mattermost incoming webhook adapter.
// Mattermost uses a Slack-compatible body format.
package notify

import (
	"encoding/json"
	"fmt"
)

type mattermostPayload struct {
	Text string `json:"text"`
}

// formatMattermost adapts payload to Mattermost's Slack-compatible incoming webhook format.
func formatMattermost(rawURL string, p Payload) (body []byte, contentType, targetURL string, err error) {
	text := fmt.Sprintf("**[%s] %s**\n%s", p.Severity, p.Subject, p.Body)
	mp := mattermostPayload{Text: text}
	b, mErr := json.Marshal(mp)
	if mErr != nil {
		return nil, "", "", fmt.Errorf("mattermost: marshal: %w", mErr)
	}
	return b, "application/json", rawURL, nil
}
