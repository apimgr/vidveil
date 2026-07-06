// SPDX-License-Identifier: MIT
// AI.md PART 12: Slack incoming webhook adapter.
// Format: POST {url} body {"text": "<message>"}
package notify

import (
	"encoding/json"
	"fmt"
)

type slackPayload struct {
	Text string `json:"text"`
}

// formatSlack adapts payload to Slack's incoming webhook format.
func formatSlack(rawURL string, p Payload) (body []byte, contentType, targetURL string, err error) {
	text := fmt.Sprintf("*[%s] %s*\n%s", p.Severity, p.Subject, p.Body)
	sp := slackPayload{Text: text}
	b, mErr := json.Marshal(sp)
	if mErr != nil {
		return nil, "", "", fmt.Errorf("slack: marshal: %w", mErr)
	}
	return b, "application/json", rawURL, nil
}
