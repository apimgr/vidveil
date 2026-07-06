// SPDX-License-Identifier: MIT
// AI.md PART 12: Discord incoming webhook adapter.
// Format: POST {url} body {"content": "<message>", "username": "{app_name}"}
package notify

import (
	"encoding/json"
	"fmt"
)

type discordPayload struct {
	Content  string `json:"content"`
	Username string `json:"username"`
}

// formatDiscord adapts payload to Discord's incoming webhook format.
func formatDiscord(rawURL string, p Payload) (body []byte, contentType, targetURL string, err error) {
	text := fmt.Sprintf("**[%s] %s**\n%s", p.Severity, p.Subject, p.Body)
	dp := discordPayload{
		Content:  text,
		Username: p.ProjectName,
	}
	b, mErr := json.Marshal(dp)
	if mErr != nil {
		return nil, "", "", fmt.Errorf("discord: marshal: %w", mErr)
	}
	return b, "application/json", rawURL, nil
}
