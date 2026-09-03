// SPDX-License-Identifier: MIT
// Tests for the httputil package: HTTP client detection and HTML-to-text conversion.
// Same-package access is required to reach unexported helpers wordWrap, centerText, stripTags.
package httputil

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// --- IsOurCliClient ---

// A request with the project-prefixed CLI User-Agent must be detected as our CLI.
func TestIsOurCliClientMatch(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "vidveil-cli/1.0")
	if !IsOurCliClient(r, "vidveil") {
		t.Error("IsOurCliClient() = false for vidveil-cli/1.0, want true")
	}
}

// A plain curl User-Agent must not match our CLI, even when projectName is correct.
func TestIsOurCliClientCurlNotCLI(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "curl/7.0")
	if IsOurCliClient(r, "vidveil") {
		t.Error("IsOurCliClient() = true for curl/7.0, want false")
	}
}

// An empty User-Agent must not match our CLI.
func TestIsOurCliClientEmptyUA(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if IsOurCliClient(r, "vidveil") {
		t.Error("IsOurCliClient() = true for empty UA, want false")
	}
}

// A different project name must not match, even if the UA format is right.
func TestIsOurCliClientWrongProject(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "vidveil-cli/1.0")
	if IsOurCliClient(r, "other") {
		t.Error("IsOurCliClient() = true for mismatched projectName, want false")
	}
}

// --- IsTextBrowser ---

// Lynx must be detected as a text browser.
func TestIsTextBrowserLynx(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Lynx/2.8.9rel.1 libwww-FM/2.14 SSL-MM/1.4.1 OpenSSL/1.1.1")
	if !IsTextBrowser(r) {
		t.Error("IsTextBrowser() = false for Lynx UA, want true")
	}
}

// w3m must be detected as a text browser.
func TestIsTextBrowserW3m(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "w3m/0.5.3+git20230121")
	if !IsTextBrowser(r) {
		t.Error("IsTextBrowser() = false for w3m UA, want true")
	}
}

// Links must be detected as a text browser (space after "Links" in UA).
func TestIsTextBrowserLinks(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Links (2.21; Linux 5.15 x86_64; GNU C 11.2)")
	if !IsTextBrowser(r) {
		t.Error("IsTextBrowser() = false for Links UA, want true")
	}
}

// ELinks must be detected as a text browser.
func TestIsTextBrowserELinks(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "ELinks/0.13.2 (textmode; Linux 5.15)")
	if !IsTextBrowser(r) {
		t.Error("IsTextBrowser() = false for ELinks UA, want true")
	}
}

// Browsh must be detected as a text browser.
func TestIsTextBrowserBrowsh(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "browsh/1.8.0")
	if !IsTextBrowser(r) {
		t.Error("IsTextBrowser() = false for browsh UA, want true")
	}
}

// A mainstream browser must not be detected as a text browser.
func TestIsTextBrowserFirefox(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0")
	if IsTextBrowser(r) {
		t.Error("IsTextBrowser() = true for Firefox UA, want false")
	}
}

// An empty User-Agent must not be detected as a text browser.
func TestIsTextBrowserEmptyUA(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if IsTextBrowser(r) {
		t.Error("IsTextBrowser() = true for empty UA, want false")
	}
}

// --- IsHttpTool ---

// curl must be detected as an HTTP tool.
func TestIsHttpToolCurl(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "curl/7.88.1")
	if !IsHttpTool(r) {
		t.Error("IsHttpTool() = false for curl UA, want true")
	}
}

// wget must be detected as an HTTP tool.
func TestIsHttpToolWget(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Wget/1.21.3 (linux-gnu)")
	if !IsHttpTool(r) {
		t.Error("IsHttpTool() = false for Wget UA, want true")
	}
}

// python-requests must be detected as an HTTP tool.
func TestIsHttpToolPythonRequests(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "python-requests/2.28.0")
	if !IsHttpTool(r) {
		t.Error("IsHttpTool() = false for python-requests UA, want true")
	}
}

// Go's built-in HTTP client must be detected as an HTTP tool.
func TestIsHttpToolGoHTTPClient(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Go-http-client/2.0")
	if !IsHttpTool(r) {
		t.Error("IsHttpTool() = false for Go-http-client UA, want true")
	}
}

// A missing User-Agent header must be treated as an HTTP tool.
func TestIsHttpToolNoUA(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if !IsHttpTool(r) {
		t.Error("IsHttpTool() = false for missing UA, want true")
	}
}

// A mainstream browser must not be detected as an HTTP tool.
func TestIsHttpToolBrowser(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if IsHttpTool(r) {
		t.Error("IsHttpTool() = true for browser UA, want false")
	}
}

// --- IsNonInteractiveClient ---

// Our CLI client is interactive and must NOT be classified as non-interactive.
func TestIsNonInteractiveClientCLI(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "vidveil-cli/1.0")
	if IsNonInteractiveClient(r, "vidveil") {
		t.Error("IsNonInteractiveClient() = true for our CLI, want false")
	}
}

// A text browser is interactive and must NOT be classified as non-interactive.
func TestIsNonInteractiveClientLynx(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Lynx/2.8.9rel.1")
	if IsNonInteractiveClient(r, "vidveil") {
		t.Error("IsNonInteractiveClient() = true for Lynx, want false")
	}
}

// curl is non-interactive and must be classified as such.
func TestIsNonInteractiveClientCurl(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "curl/7.88.1")
	if !IsNonInteractiveClient(r, "vidveil") {
		t.Error("IsNonInteractiveClient() = false for curl, want true")
	}
}

// A mainstream browser is interactive and must NOT be classified as non-interactive.
func TestIsNonInteractiveClientBrowser(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")
	if IsNonInteractiveClient(r, "vidveil") {
		t.Error("IsNonInteractiveClient() = true for browser, want false")
	}
}

// --- HTML2TextConverter ---

// A plain paragraph must contain its text.
func TestHTML2TextPlainParagraph(t *testing.T) {
	got := HTML2TextConverter("<p>Hello world</p>", 80)
	if !strings.Contains(got, "Hello world") {
		t.Errorf("HTML2TextConverter(<p>Hello world</p>) = %q, want to contain \"Hello world\"", got)
	}
}

// An h1 must be uppercased and surrounded by ═ border lines.
func TestHTML2TextH1Uppercase(t *testing.T) {
	got := HTML2TextConverter("<h1>Title</h1>", 80)
	if !strings.Contains(got, "TITLE") {
		t.Errorf("HTML2TextConverter(<h1>Title</h1>) = %q, want to contain \"TITLE\"", got)
	}
	if !strings.Contains(got, "═") {
		t.Errorf("HTML2TextConverter(<h1>Title</h1>) = %q, want to contain \"═\" border", got)
	}
}

// An h2 must use the ─── prefix and suffix format.
func TestHTML2TextH2Format(t *testing.T) {
	got := HTML2TextConverter("<h2>Section</h2>", 80)
	if !strings.Contains(got, "─── Section ───") {
		t.Errorf("HTML2TextConverter(<h2>Section</h2>) = %q, want to contain \"─── Section ───\"", got)
	}
}

// An h3 must use the ► prefix.
func TestHTML2TextH3Format(t *testing.T) {
	got := HTML2TextConverter("<h3>Sub</h3>", 80)
	if !strings.Contains(got, "► Sub") {
		t.Errorf("HTML2TextConverter(<h3>Sub</h3>) = %q, want to contain \"► Sub\"", got)
	}
}

// <strong> must wrap content with asterisks.
func TestHTML2TextStrong(t *testing.T) {
	got := HTML2TextConverter("<strong>bold</strong>", 80)
	if !strings.Contains(got, "*bold*") {
		t.Errorf("HTML2TextConverter(<strong>bold</strong>) = %q, want to contain \"*bold*\"", got)
	}
}

// <em> must wrap content with underscores.
func TestHTML2TextEm(t *testing.T) {
	got := HTML2TextConverter("<em>italic</em>", 80)
	if !strings.Contains(got, "_italic_") {
		t.Errorf("HTML2TextConverter(<em>italic</em>) = %q, want to contain \"_italic_\"", got)
	}
}

// <code> must wrap content with backticks.
func TestHTML2TextCode(t *testing.T) {
	got := HTML2TextConverter("<code>x</code>", 80)
	if !strings.Contains(got, "`x`") {
		t.Errorf("HTML2TextConverter(<code>x</code>) = %q, want to contain \"`x`\"", got)
	}
}

// <pre> must indent lines with 4 spaces.
func TestHTML2TextPreIndent(t *testing.T) {
	got := HTML2TextConverter("<pre>line1\nline2</pre>", 80)
	if !strings.Contains(got, "    line1") {
		t.Errorf("HTML2TextConverter(<pre>line1\\nline2</pre>) = %q, want to contain 4-space-indented \"    line1\"", got)
	}
}

// <hr> must produce a line of ─ characters.
func TestHTML2TextHR(t *testing.T) {
	got := HTML2TextConverter("<hr>", 80)
	if !strings.Contains(got, "─") {
		t.Errorf("HTML2TextConverter(<hr>) = %q, want to contain \"─\"", got)
	}
}

// An <a> with an href must render as "text [url]".
func TestHTML2TextAnchorWithHref(t *testing.T) {
	got := HTML2TextConverter(`<a href="http://x.com">link</a>`, 80)
	if !strings.Contains(got, "link [http://x.com]") {
		t.Errorf("HTML2TextConverter(<a href=...>) = %q, want to contain \"link [http://x.com]\"", got)
	}
}

// An <a> without an href must render as plain text with no brackets.
func TestHTML2TextAnchorNoHref(t *testing.T) {
	got := HTML2TextConverter("<a>nolink</a>", 80)
	if !strings.Contains(got, "nolink") {
		t.Errorf("HTML2TextConverter(<a>nolink</a>) = %q, want to contain \"nolink\"", got)
	}
	if strings.Contains(got, "[") {
		t.Errorf("HTML2TextConverter(<a>nolink</a>) = %q, must not contain \"[\" when no href", got)
	}
}

// An unordered list must render with bullet characters.
func TestHTML2TextUL(t *testing.T) {
	got := HTML2TextConverter("<ul><li>item1</li><li>item2</li></ul>", 80)
	if !strings.Contains(got, "•") {
		t.Errorf("HTML2TextConverter(<ul>...) = %q, want to contain \"•\" bullets", got)
	}
}

// An ordered list must render with numeric prefixes.
func TestHTML2TextOL(t *testing.T) {
	got := HTML2TextConverter("<ol><li>first</li><li>second</li></ol>", 80)
	if !strings.Contains(got, "1.") {
		t.Errorf("HTML2TextConverter(<ol>...) = %q, want to contain \"1.\"", got)
	}
	if !strings.Contains(got, "2.") {
		t.Errorf("HTML2TextConverter(<ol>...) = %q, want to contain \"2.\"", got)
	}
}

// A blockquote must be prefixed with the │ character.
func TestHTML2TextBlockquote(t *testing.T) {
	got := HTML2TextConverter("<blockquote>quoted</blockquote>", 80)
	if !strings.Contains(got, "│") {
		t.Errorf("HTML2TextConverter(<blockquote>...) = %q, want to contain \"│\"", got)
	}
}

// <br> must produce an extra newline in the output.
func TestHTML2TextBR(t *testing.T) {
	got := HTML2TextConverter("before<br>after", 80)
	if !strings.Contains(got, "\n") {
		t.Errorf("HTML2TextConverter(before<br>after) = %q, want to contain newline from <br>", got)
	}
}

// <script> content must be silently skipped.
func TestHTML2TextScriptSkipped(t *testing.T) {
	got := HTML2TextConverter("<script>js code</script>", 80)
	if strings.Contains(got, "js code") {
		t.Errorf("HTML2TextConverter(<script>js code</script>) = %q, must not contain script body", got)
	}
}

// <style> content must be silently skipped.
func TestHTML2TextStyleSkipped(t *testing.T) {
	got := HTML2TextConverter("<style>css { color: red; }</style>", 80)
	if strings.Contains(got, "css") {
		t.Errorf("HTML2TextConverter(<style>css</style>) = %q, must not contain style body", got)
	}
}

// A simple table must render with box-drawing characters and column headers.
func TestHTML2TextTable(t *testing.T) {
	html := "<table><tr><th>Col1</th><th>Col2</th></tr><tr><td>a</td><td>b</td></tr></table>"
	got := HTML2TextConverter(html, 80)
	if !strings.Contains(got, "┌") {
		t.Errorf("HTML2TextConverter(table) = %q, want to contain \"┌\"", got)
	}
	if !strings.Contains(got, "│") {
		t.Errorf("HTML2TextConverter(table) = %q, want to contain \"│\"", got)
	}
	if !strings.Contains(got, "Col1") {
		t.Errorf("HTML2TextConverter(table) = %q, want to contain \"Col1\"", got)
	}
}

// An empty string must return an empty result without panicking.
func TestHTML2TextEmptyInput(t *testing.T) {
	got := HTML2TextConverter("", 80)
	if strings.TrimSpace(got) != "" {
		t.Errorf("HTML2TextConverter(\"\") = %q, want empty or whitespace-only", got)
	}
}

// Malformed HTML must not panic and must return something.
func TestHTML2TextMalformedNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HTML2TextConverter(malformed) panicked: %v", r)
		}
	}()
	_ = HTML2TextConverter("<p>unclosed <b>tag", 80)
}

// A width of 0 must not panic; the function defaults to 80 internally.
func TestHTML2TextWidthZeroNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("HTML2TextConverter(width=0) panicked: %v", r)
		}
	}()
	_ = HTML2TextConverter("<p>text</p>", 0)
}

// --- wordWrap (unexported helper) ---

// A two-word string that exceeds the width must be split across lines.
func TestWordWrapBasic(t *testing.T) {
	got := wordWrap("hello world", 5)
	if got != "hello\nworld" {
		t.Errorf("wordWrap(\"hello world\", 5) = %q, want %q", got, "hello\nworld")
	}
}

// An empty string must return an empty string.
func TestWordWrapEmpty(t *testing.T) {
	got := wordWrap("", 80)
	if got != "" {
		t.Errorf("wordWrap(\"\", 80) = %q, want %q", got, "")
	}
}

// A single word shorter than the width must be returned unchanged.
func TestWordWrapSingleWord(t *testing.T) {
	got := wordWrap("hello", 80)
	if got != "hello" {
		t.Errorf("wordWrap(\"hello\", 80) = %q, want %q", got, "hello")
	}
}

// width<=0 must default to 80 and not panic.
func TestWordWrapNegativeWidth(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("wordWrap(width=-1) panicked: %v", r)
		}
	}()
	got := wordWrap("hello world", -1)
	if got == "" {
		t.Error("wordWrap(\"hello world\", -1) returned empty, want non-empty")
	}
}

// --- centerText (unexported helper) ---

// "hi" centered in width 10 must have 4 leading spaces: (10-2)/2 = 4.
func TestCenterTextBasic(t *testing.T) {
	got := centerText("hi", 10)
	if !strings.HasPrefix(got, "    hi") {
		t.Errorf("centerText(\"hi\", 10) = %q, want prefix of 4 spaces then \"hi\"", got)
	}
}

// Text longer than width must be returned unchanged (no truncation).
func TestCenterTextLongerThanWidth(t *testing.T) {
	got := centerText("toolong", 3)
	if got != "toolong" {
		t.Errorf("centerText(\"toolong\", 3) = %q, want %q (no truncation)", got, "toolong")
	}
}

// Text exactly equal to width must be returned unchanged.
func TestCenterTextExactWidth(t *testing.T) {
	got := centerText("abc", 3)
	if got != "abc" {
		t.Errorf("centerText(\"abc\", 3) = %q, want %q", got, "abc")
	}
}

// --- stripTags (unexported helper) ---

// stripTags does not strip HTML tags; it only trims whitespace.
func TestStripTagsPreservesHTML(t *testing.T) {
	got := stripTags("<b>test</b>")
	if got != "<b>test</b>" {
		t.Errorf("stripTags(\"<b>test</b>\") = %q, want %q", got, "<b>test</b>")
	}
}

// stripTags trims leading and trailing whitespace.
func TestStripTagsTrimsSpace(t *testing.T) {
	got := stripTags("  hello  ")
	if got != "hello" {
		t.Errorf("stripTags(\"  hello  \") = %q, want %q", got, "hello")
	}
}
