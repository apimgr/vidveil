// SPDX-License-Identifier: MIT
// AI.md PART 12: Telegram Bot API webhook adapter.
// Format: POST {url}&text={urlencoded message}
package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// formatTelegram adapts payload to the Telegram Bot sendMessage format.
// The URL must include the bot token and chat_id query parameter, e.g.:
// https://api.telegram.org/bot{TOKEN}/sendMessage?chat_id={CHAT}
func formatTelegram(rawURL string, p Payload) (body []byte, contentType, targetURL string, err error) {
	text := fmt.Sprintf("[%s] %s\n%s", p.Severity, p.Subject, p.Body)
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return nil, "", "", fmt.Errorf("telegram: invalid URL: %w", parseErr)
	}
	q := parsed.Query()
	q.Set("text", text)
	q.Set("parse_mode", "HTML")
	parsed.RawQuery = q.Encode()

	payload := map[string]string{
		"text":       text,
		"parse_mode": "HTML",
	}
	b, mErr := json.Marshal(payload)
	if mErr != nil {
		return nil, "", "", fmt.Errorf("telegram: marshal: %w", mErr)
	}
	return b, "application/json", parsed.String(), nil
}
