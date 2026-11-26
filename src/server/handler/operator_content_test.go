package handler

import (
	"strings"
	"testing"
)

func TestStripExternalAnchorLinks(t *testing.T) {
	input := `<p><a href="https://outside.example/policy">Policy</a> <a href='//cdn.example/page'>Page</a> <a href="/local">Local</a></p>`
	got := stripExternalAnchorLinks(input)
	for _, want := range []string{"Policy", "Page", `<a href="/local">Local</a>`} {
		if !strings.Contains(got, want) {
			t.Errorf("stripped markup %q does not contain %q", got, want)
		}
	}
	for _, unwanted := range []string{"https://outside.example", "//cdn.example"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("stripped markup %q still contains external URL %q", got, unwanted)
		}
	}
}
