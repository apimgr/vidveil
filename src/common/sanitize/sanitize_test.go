// SPDX-License-Identifier: MIT
package sanitize

import (
	"strings"
	"testing"
)

func TestDocumentStripsH1(t *testing.T) {
	got := string(Document(`<h1>Unshifted</h1><h3>Allowed</h3>`))
	if strings.Contains(got, "<h1") || !strings.Contains(got, "<h3>Allowed</h3>") {
		t.Fatalf("document sanitization did not enforce the heading policy: %q", got)
	}
}
