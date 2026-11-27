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

func TestShiftOperatorHeadings(t *testing.T) {
	got := shiftOperatorHeadings(`<h1 id="main">Main</h1><h2>Section</h2><h5>Small</h5>`)
	for _, want := range []string{`<h3 id="main">Main</h3>`, `<h4>Section</h4>`, `<h6>Small</h6>`} {
		if !strings.Contains(got, want) {
			t.Errorf("shifted markup %q does not contain %q", got, want)
		}
	}
}
