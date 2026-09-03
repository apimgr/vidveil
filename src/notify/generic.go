// SPDX-License-Identifier: MIT
// AI.md PART 12: Generic webhook adapter.
// Format: POST {url} JSON body with the canonical Payload fields.
package notify

import (
	"encoding/json"
	"fmt"
)

// formatGeneric sends the canonical JSON payload to any HTTPS endpoint.
// The receiving server can verify the X-Webhook-Signature header.
func formatGeneric(rawURL string, p Payload) (body []byte, contentType, targetURL string, err error) {
	b, mErr := json.Marshal(p)
	if mErr != nil {
		return nil, "", "", fmt.Errorf("generic: marshal: %w", mErr)
	}
	return b, "application/json", rawURL, nil
}
