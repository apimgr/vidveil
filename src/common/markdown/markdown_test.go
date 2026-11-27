// SPDX-License-Identifier: MIT
package markdown

import (
	"strings"
	"testing"
)

func TestToDocumentHTMLShiftsHeadingsByTwoLevels(t *testing.T) {
	got := string(ToDocumentHTML("# Main\n\n## Section\n\n#### Detail\n\n###### Capped"))
	for _, heading := range []string{"<h3>Main</h3>", "<h4>Section</h4>", "<h6>Detail</h6>", "<h6>Capped</h6>"} {
		if !strings.Contains(got, heading) {
			t.Errorf("rendered document %q does not contain %q", got, heading)
		}
	}
}

func TestToHTMLDoesNotInterpretHeadings(t *testing.T) {
	got := string(ToHTML("# Plain text"))
	if strings.Contains(got, "<h") {
		t.Fatalf("ToHTML interpreted an operator heading: %q", got)
	}
}
