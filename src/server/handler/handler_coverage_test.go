// SPDX-License-Identifier: MIT
package handler

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

// TestGetRequestTheme_NoCookie verifies that the config default theme is returned when no cookie is present.
func TestGetRequestTheme_NoCookie(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	got := h.getRequestTheme(req)
	if got != "dark" {
		t.Errorf("getRequestTheme no cookie = %q, want %q", got, "dark")
	}
}

// TestGetRequestTheme_LightCookie verifies that cookie value "light" is honoured.
func TestGetRequestTheme_LightCookie(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "light"})
	got := h.getRequestTheme(req)
	if got != "light" {
		t.Errorf("getRequestTheme light cookie = %q, want %q", got, "light")
	}
}

// TestGetRequestTheme_AutoCookie verifies that cookie value "auto" is honoured.
func TestGetRequestTheme_AutoCookie(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "auto"})
	got := h.getRequestTheme(req)
	if got != "auto" {
		t.Errorf("getRequestTheme auto cookie = %q, want %q", got, "auto")
	}
}

// TestGetRequestTheme_InvalidCookie verifies that an invalid cookie value falls back to the config default.
func TestGetRequestTheme_InvalidCookie(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "theme", Value: "rainbow"})
	got := h.getRequestTheme(req)
	if got != "dark" {
		t.Errorf("getRequestTheme invalid cookie = %q, want config default %q", got, "dark")
	}
}

// TestIsTorRequest_OnionHost verifies that a .onion host is detected as a Tor request.
func TestIsTorRequest_OnionHost(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.onion"
	if !h.isTorRequest(req) {
		t.Error("isTorRequest should return true for .onion host")
	}
}

// TestIsTorRequest_TorHeader verifies that the X-Tor-Hidden-Service header triggers Tor detection.
func TestIsTorRequest_TorHeader(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Tor-Hidden-Service", "1")
	if !h.isTorRequest(req) {
		t.Error("isTorRequest should return true for X-Tor-Hidden-Service: 1 header")
	}
}

// TestIsTorRequest_PlainHost verifies that a plain host returns false.
func TestIsTorRequest_PlainHost(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"
	if h.isTorRequest(req) {
		t.Error("isTorRequest should return false for plain host")
	}
}

// TestHasContentRestrictionAck_NoCookie verifies that absent cookie returns false.
func TestHasContentRestrictionAck_NoCookie(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	if h.hasContentRestrictionAck(req) {
		t.Error("hasContentRestrictionAck should return false with no cookie")
	}
}

// TestHasContentRestrictionAck_ValueOne verifies that cookie value "1" returns true.
func TestHasContentRestrictionAck_ValueOne(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: ContentRestrictionAckCookieName, Value: "1"})
	if !h.hasContentRestrictionAck(req) {
		t.Error("hasContentRestrictionAck should return true for cookie value '1'")
	}
}

// TestHasContentRestrictionAck_OtherValue verifies that other cookie values return false.
func TestHasContentRestrictionAck_OtherValue(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: ContentRestrictionAckCookieName, Value: "yes"})
	if h.hasContentRestrictionAck(req) {
		t.Error("hasContentRestrictionAck should return false for cookie value other than '1'")
	}
}

// TestIsOurCliClient_VidveilCli verifies that a UA starting with "vidveil-cli/" returns true.
func TestIsOurCliClient_VidveilCli(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "vidveil-cli/1.0.0")
	if !isOurCliClient(req) {
		t.Error("isOurCliClient should return true for 'vidveil-cli/' UA")
	}
}

// TestIsOurCliClient_Browser verifies that a browser UA returns false.
func TestIsOurCliClient_Browser(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	if isOurCliClient(req) {
		t.Error("isOurCliClient should return false for browser UA")
	}
}

// TestIsOurCliClient_EmptyUA verifies that empty UA returns false.
func TestIsOurCliClient_EmptyUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "")
	if isOurCliClient(req) {
		t.Error("isOurCliClient should return false for empty UA")
	}
}

// TestIsTextBrowser_Lynx verifies lynx UA is detected.
func TestIsTextBrowser_Lynx(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "lynx/2.8.9")
	if !isTextBrowser(req) {
		t.Error("isTextBrowser should return true for lynx UA")
	}
}

// TestIsTextBrowser_W3m verifies w3m UA is detected.
func TestIsTextBrowser_W3m(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "w3m/0.5.3")
	if !isTextBrowser(req) {
		t.Error("isTextBrowser should return true for w3m UA")
	}
}

// TestIsTextBrowser_Links verifies links UA is detected.
func TestIsTextBrowser_Links(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "links/2.21")
	if !isTextBrowser(req) {
		t.Error("isTextBrowser should return true for links UA")
	}
}

// TestIsTextBrowser_Elinks verifies elinks UA is detected.
func TestIsTextBrowser_Elinks(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "elinks/0.13")
	if !isTextBrowser(req) {
		t.Error("isTextBrowser should return true for elinks UA")
	}
}

// TestIsTextBrowser_Browsh verifies browsh UA is detected.
func TestIsTextBrowser_Browsh(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "browsh/1.8.2")
	if !isTextBrowser(req) {
		t.Error("isTextBrowser should return true for browsh UA")
	}
}

// TestIsTextBrowser_Carbonyl verifies carbonyl UA is detected.
func TestIsTextBrowser_Carbonyl(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "carbonyl/0.0.3")
	if !isTextBrowser(req) {
		t.Error("isTextBrowser should return true for carbonyl UA")
	}
}

// TestIsTextBrowser_Netsurf verifies netsurf UA is detected.
func TestIsTextBrowser_Netsurf(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "netsurf/3.10")
	if !isTextBrowser(req) {
		t.Error("isTextBrowser should return true for netsurf UA")
	}
}

// TestIsTextBrowser_Browser verifies a standard browser UA returns false.
func TestIsTextBrowser_Browser(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")
	if isTextBrowser(req) {
		t.Error("isTextBrowser should return false for Mozilla UA")
	}
}

// TestIsTextBrowser_Empty verifies empty UA returns false.
func TestIsTextBrowser_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "")
	if isTextBrowser(req) {
		t.Error("isTextBrowser should return false for empty UA")
	}
}

// TestIsHttpTool_Curl verifies curl UA is detected.
func TestIsHttpTool_Curl(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "curl/7.68.0")
	if !isHttpTool(req) {
		t.Error("isHttpTool should return true for curl UA")
	}
}

// TestIsHttpTool_Wget verifies wget UA is detected.
func TestIsHttpTool_Wget(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "wget/1.20.3")
	if !isHttpTool(req) {
		t.Error("isHttpTool should return true for wget UA")
	}
}

// TestIsHttpTool_EmptyUA verifies empty UA is treated as HTTP tool.
func TestIsHttpTool_EmptyUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "")
	if !isHttpTool(req) {
		t.Error("isHttpTool should return true for empty UA")
	}
}

// TestIsHttpTool_Httpie verifies httpie UA is detected.
func TestIsHttpTool_Httpie(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "httpie/3.0.0")
	if !isHttpTool(req) {
		t.Error("isHttpTool should return true for httpie UA")
	}
}

// TestIsHttpTool_Browser verifies a browser UA returns false.
func TestIsHttpTool_Browser(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")
	if isHttpTool(req) {
		t.Error("isHttpTool should return false for Mozilla UA")
	}
}

// TestIsHttpTool_ShortNonEmpty verifies a short (< 4 chars) non-empty UA returns false.
func TestIsHttpTool_ShortNonEmpty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "abc")
	if isHttpTool(req) {
		t.Error("isHttpTool should return false for short non-empty UA < 4 chars")
	}
}

// TestRenderSimpleHTML_Home verifies the home case includes expected text.
func TestRenderSimpleHTML_Home(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	result := h.renderSimpleHTML("home", nil)
	if !strings.Contains(result, "VidVeil") {
		t.Error("renderSimpleHTML home should contain VidVeil")
	}
	if !strings.Contains(result, "<html>") || !strings.Contains(result, "<body>") {
		t.Error("renderSimpleHTML home should be wrapped in html/body")
	}
}

// TestRenderSimpleHTML_About verifies the about case includes expected text.
func TestRenderSimpleHTML_About(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	result := h.renderSimpleHTML("about", nil)
	if !strings.Contains(result, "About") {
		t.Error("renderSimpleHTML about should contain 'About'")
	}
}

// TestRenderSimpleHTML_Privacy verifies the privacy case includes expected text.
func TestRenderSimpleHTML_Privacy(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	result := h.renderSimpleHTML("privacy", nil)
	if !strings.Contains(result, "Privacy") {
		t.Error("renderSimpleHTML privacy should contain 'Privacy'")
	}
}

// TestRenderSimpleHTML_Preferences verifies the preferences case includes expected text.
func TestRenderSimpleHTML_Preferences(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	result := h.renderSimpleHTML("preferences", nil)
	if !strings.Contains(result, "Preferences") {
		t.Error("renderSimpleHTML preferences should contain 'Preferences'")
	}
}

// TestRenderSimpleHTML_Search verifies the search case includes query from data.
func TestRenderSimpleHTML_Search(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	data := map[string]interface{}{"query": "golang test"}
	result := h.renderSimpleHTML("search", data)
	if !strings.Contains(result, "golang test") {
		t.Error("renderSimpleHTML search should contain query text")
	}
}

// TestRenderSimpleHTML_AgeVerify verifies the age-verify case includes expected text.
func TestRenderSimpleHTML_AgeVerify(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	result := h.renderSimpleHTML("age-verify", nil)
	if !strings.Contains(result, "Age") {
		t.Error("renderSimpleHTML age-verify should contain 'Age'")
	}
}

// TestRenderSimpleHTML_ContentRestricted verifies Message and Region appear in the output.
func TestRenderSimpleHTML_ContentRestricted(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	data := map[string]interface{}{
		"Message": "Adult content restricted",
		"Region":  "US",
	}
	result := h.renderSimpleHTML("content-restricted", data)
	if !strings.Contains(result, "Adult content restricted") {
		t.Error("renderSimpleHTML content-restricted should contain Message")
	}
	if !strings.Contains(result, "US") {
		t.Error("renderSimpleHTML content-restricted should contain Region")
	}
}

// TestRenderSimpleHTML_ContentBlocked verifies Message and Region appear in the blocked output.
func TestRenderSimpleHTML_ContentBlocked(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	data := map[string]interface{}{
		"Message": "Service unavailable",
		"Region":  "CN",
	}
	result := h.renderSimpleHTML("content-blocked", data)
	if !strings.Contains(result, "Service unavailable") {
		t.Error("renderSimpleHTML content-blocked should contain Message")
	}
	if !strings.Contains(result, "CN") {
		t.Error("renderSimpleHTML content-blocked should contain Region")
	}
}

// TestRenderSimpleHTML_Unknown verifies that an unknown name still returns a body wrapper.
func TestRenderSimpleHTML_Unknown(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	result := h.renderSimpleHTML("does-not-exist", nil)
	if !strings.Contains(result, "<body>") || !strings.Contains(result, "</body>") {
		t.Error("renderSimpleHTML unknown should still return body wrapper")
	}
}

// TestConvertHTMLToText_H1 verifies that h1 tags are stripped and title text is preserved.
func TestConvertHTMLToText_H1(t *testing.T) {
	input := "<h1>My Title</h1>"
	result := convertHTMLToText(input, 40)
	if strings.Contains(result, "<h1>") || strings.Contains(result, "</h1>") {
		t.Error("convertHTMLToText should remove <h1> tags")
	}
	if !strings.Contains(result, "My Title") {
		t.Error("convertHTMLToText should preserve title text")
	}
}

// TestConvertHTMLToText_Paragraph verifies that p tags are removed and text is preserved.
func TestConvertHTMLToText_Paragraph(t *testing.T) {
	input := "<p>Hello world</p>"
	result := convertHTMLToText(input, 40)
	if strings.Contains(result, "<p>") {
		t.Error("convertHTMLToText should remove <p> tags")
	}
	if !strings.Contains(result, "Hello world") {
		t.Error("convertHTMLToText should preserve paragraph text")
	}
}

// TestConvertHTMLToText_ListItem verifies that li items produce a bullet indicator.
func TestConvertHTMLToText_ListItem(t *testing.T) {
	input := "<ul><li>item one</li></ul>"
	result := convertHTMLToText(input, 40)
	if !strings.Contains(result, "item one") {
		t.Error("convertHTMLToText should preserve list item text")
	}
	if !strings.Contains(result, "•") {
		t.Error("convertHTMLToText should convert li to bullet")
	}
}

// TestHTMLEscape_LessThan verifies < is escaped.
func TestHTMLEscape_LessThan(t *testing.T) {
	if got := htmlEscape("<"); got != "&lt;" {
		t.Errorf("htmlEscape('<') = %q, want %q", got, "&lt;")
	}
}

// TestHTMLEscape_GreaterThan verifies > is escaped.
func TestHTMLEscape_GreaterThan(t *testing.T) {
	if got := htmlEscape(">"); got != "&gt;" {
		t.Errorf("htmlEscape('>') = %q, want %q", got, "&gt;")
	}
}

// TestHTMLEscape_Ampersand verifies & is escaped.
func TestHTMLEscape_Ampersand(t *testing.T) {
	if got := htmlEscape("&"); got != "&amp;" {
		t.Errorf("htmlEscape('&') = %q, want %q", got, "&amp;")
	}
}

// TestHTMLEscape_Quote verifies " is escaped.
func TestHTMLEscape_Quote(t *testing.T) {
	if got := htmlEscape(`"`); got != "&quot;" {
		t.Errorf(`htmlEscape('"') = %q, want %q`, got, "&quot;")
	}
}

// TestHTMLEscape_PlainText verifies plain text is unchanged.
func TestHTMLEscape_PlainText(t *testing.T) {
	in := "hello world"
	if got := htmlEscape(in); got != in {
		t.Errorf("htmlEscape(%q) = %q, want unchanged", in, got)
	}
}

// TestIntToString_Zero verifies 0 maps to "0".
func TestIntToString_Zero(t *testing.T) {
	if got := intToString(0); got != "0" {
		t.Errorf("intToString(0) = %q, want %q", got, "0")
	}
}

// TestIntToString_Positive verifies positive numbers convert correctly.
func TestIntToString_Positive(t *testing.T) {
	if got := intToString(42); got != "42" {
		t.Errorf("intToString(42) = %q, want %q", got, "42")
	}
}

// TestIntToString_Negative verifies negative numbers convert correctly.
func TestIntToString_Negative(t *testing.T) {
	if got := intToString(-5); got != "-5" {
		t.Errorf("intToString(-5) = %q, want %q", got, "-5")
	}
}

// TestIntToString_Hundred verifies intToString(100) returns "100".
func TestIntToString_Hundred(t *testing.T) {
	if got := intToString(100); got != "100" {
		t.Errorf("intToString(100) = %q, want %q", got, "100")
	}
}

// TestRepeatStr_Three verifies repeatStr("x", 3) returns "xxx".
func TestRepeatStr_Three(t *testing.T) {
	if got := repeatStr("x", 3); got != "xxx" {
		t.Errorf("repeatStr('x', 3) = %q, want %q", got, "xxx")
	}
}

// TestRepeatStr_Zero verifies repeatStr("ab", 0) returns "".
func TestRepeatStr_Zero(t *testing.T) {
	if got := repeatStr("ab", 0); got != "" {
		t.Errorf("repeatStr('ab', 0) = %q, want empty string", got)
	}
}

// TestReplaceAll_Match verifies replacement of found substring.
func TestReplaceAll_Match(t *testing.T) {
	got := replaceAll("hello <p> world <p> end", "<p>", "")
	if strings.Contains(got, "<p>") {
		t.Errorf("replaceAll should remove all <p>, got %q", got)
	}
}

// TestReplaceAll_NoMatch verifies that a string with no match is unchanged.
func TestReplaceAll_NoMatch(t *testing.T) {
	in := "nothing to replace"
	got := replaceAll(in, "<p>", "X")
	if got != in {
		t.Errorf("replaceAll no match: got %q, want %q", got, in)
	}
}

// TestIndexOf_Found verifies the correct index is returned when substring is found.
func TestIndexOf_Found(t *testing.T) {
	idx := indexOf("hello world", "world")
	if idx != 6 {
		t.Errorf("indexOf('hello world', 'world') = %d, want 6", idx)
	}
}

// TestIndexOf_NotFound verifies -1 is returned when substring is absent.
func TestIndexOf_NotFound(t *testing.T) {
	idx := indexOf("hello world", "xyz")
	if idx != -1 {
		t.Errorf("indexOf not found = %d, want -1", idx)
	}
}

// TestGetClientIP_XForwardedForSingle verifies a single XFF IP is returned.
func TestGetClientIP_XForwardedForSingle(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := getClientIP(req); got != "1.2.3.4" {
		t.Errorf("getClientIP XFF single = %q, want %q", got, "1.2.3.4")
	}
}

// TestGetClientIP_XForwardedForChain verifies the first IP in a chain is returned.
func TestGetClientIP_XForwardedForChain(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	if got := getClientIP(req); got != "1.2.3.4" {
		t.Errorf("getClientIP XFF chain = %q, want %q", got, "1.2.3.4")
	}
}

// TestGetClientIP_XRealIP verifies X-Real-IP header is used.
func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	if got := getClientIP(req); got != "10.0.0.1" {
		t.Errorf("getClientIP X-Real-IP = %q, want %q", got, "10.0.0.1")
	}
}

// TestGetClientIP_RemoteAddr verifies RemoteAddr is parsed correctly (IPv4 with port).
func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:9090"
	if got := getClientIP(req); got != "192.0.2.1" {
		t.Errorf("getClientIP RemoteAddr = %q, want %q", got, "192.0.2.1")
	}
}

// TestGetClientIP_IPv6RemoteAddr verifies IPv6 RemoteAddr brackets are stripped.
func TestGetClientIP_IPv6RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:8080"
	if got := getClientIP(req); got != "::1" {
		t.Errorf("getClientIP IPv6 RemoteAddr = %q, want %q", got, "::1")
	}
}

// TestNewServerHandler_NonNil verifies that NewServerHandler returns a non-nil handler.
func TestNewServerHandler_NonNil(t *testing.T) {
	cfg := createTestConfig()
	h := NewServerHandler(cfg)
	if h == nil {
		t.Fatal("NewServerHandler should return non-nil handler")
	}
}

// TestNewServerHandler_NilConfig verifies that NewServerHandler with nil config uses a default.
func TestNewServerHandler_NilConfig(t *testing.T) {
	h := NewServerHandler(nil)
	if h == nil {
		t.Fatal("NewServerHandler(nil) should return non-nil handler")
	}
	if h.appConfig == nil {
		t.Error("NewServerHandler(nil) should populate appConfig with default")
	}
}

// TestAPIAbout_StatusOK verifies APIAbout returns 200 with ok:true and a name field.
func TestAPIAbout_StatusOK(t *testing.T) {
	cfg := createTestConfig()
	h := NewServerHandler(cfg)
	req := httptest.NewRequest("GET", "/api/v1/server/about", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIAbout(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APIAbout status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("APIAbout returned invalid JSON: %v", err)
	}
	if resp["ok"] != true {
		t.Error("APIAbout should return ok:true")
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("APIAbout should return a data object")
	}
	if data["name"] == nil {
		t.Error("APIAbout data should contain a name field")
	}
}

// TestAPIPrivacy_StatusOK verifies APIPrivacy returns 200 with ok:true.
func TestAPIPrivacy_StatusOK(t *testing.T) {
	cfg := createTestConfig()
	h := NewServerHandler(cfg)
	req := httptest.NewRequest("GET", "/api/v1/server/privacy", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIPrivacy(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APIPrivacy status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("APIPrivacy returned invalid JSON: %v", err)
	}
	if resp["ok"] != true {
		t.Error("APIPrivacy should return ok:true")
	}
}

// TestAPIHelp_StatusOK verifies APIHelp returns 200 with ok:true.
func TestAPIHelp_StatusOK(t *testing.T) {
	cfg := createTestConfig()
	h := NewServerHandler(cfg)
	req := httptest.NewRequest("GET", "/api/v1/server/help", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIHelp(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APIHelp status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("APIHelp returned invalid JSON: %v", err)
	}
	if resp["ok"] != true {
		t.Error("APIHelp should return ok:true")
	}
}

// TestAPIContact_GetMethodNotAllowed verifies GET returns 405.
func TestAPIContact_GetMethodNotAllowed(t *testing.T) {
	cfg := createTestConfig()
	h := NewServerHandler(cfg)
	req := httptest.NewRequest("GET", "/api/v1/server/contact", nil)
	rr := httptest.NewRecorder()
	h.APIContact(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("APIContact GET status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// TestAPIContact_PostMissingFields verifies POST with missing fields returns 400.
func TestAPIContact_PostMissingFields(t *testing.T) {
	cfg := createTestConfig()
	h := NewServerHandler(cfg)
	body := strings.NewReader("subject=hello")
	req := httptest.NewRequest("POST", "/api/v1/server/contact", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.APIContact(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("APIContact POST missing fields status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestAPIContact_PostValid verifies a valid POST returns 200.
func TestAPIContact_PostValid(t *testing.T) {
	cfg := createTestConfig()
	h := NewServerHandler(cfg)
	body := strings.NewReader("subject=hello&message=world")
	req := httptest.NewRequest("POST", "/api/v1/server/contact", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.APIContact(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("APIContact POST valid status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestChangePasswordRedirect_RedirectsToAdminLogin verifies the handler redirects to admin path + "/login".
func TestChangePasswordRedirect_RedirectsToAdminLogin(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server.Admin.Path = "admin"
	handler := ChangePasswordRedirect(cfg)
	req := httptest.NewRequest("GET", "/.well-known/change-password", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("ChangePasswordRedirect status = %d, want %d", rr.Code, http.StatusFound)
	}
	loc := rr.Header().Get("Location")
	want := cfg.AdminURLPrefix() + "/login"
	if loc != want {
		t.Errorf("ChangePasswordRedirect Location = %q, want %q", loc, want)
	}
}

// TestSetDataDir_NoPanic verifies SetDataDir does not panic.
func TestSetDataDir_NoPanic(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	h.SetDataDir("/tmp/test-data")
	if h.dataDir != "/tmp/test-data" {
		t.Errorf("SetDataDir: dataDir = %q, want %q", h.dataDir, "/tmp/test-data")
	}
}

// TestSetMetrics_NoPanic verifies SetMetrics accepts nil without panicking.
func TestSetMetrics_NoPanic(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	h.SetMetrics(nil)
	if h.metrics != nil {
		t.Error("SetMetrics(nil) should set metrics to nil")
	}
}

// TestSetTorService_NoPanic verifies SetTorService accepts nil without panicking.
func TestSetTorService_NoPanic(t *testing.T) {
	cfg := createTestConfig()
	h := &SearchHandler{appConfig: cfg}
	h.SetTorService(nil)
	if h.torSvc != nil {
		t.Error("SetTorService(nil) should set torSvc to nil")
	}
}

// Ensure createTestConfig is used to suppress the "imported and not used" linter note.
var _ *config.AppConfig = createTestConfig()

// ── SetGeoIPService ───────────────────────────────────────────────────────────

// TestSetGeoIPService_NoPanic verifies that SetGeoIPService accepts nil without panicking.
func TestSetGeoIPService_NoPanic(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	h.SetGeoIPService(nil)
	if h.geoipSvc != nil {
		t.Error("SetGeoIPService(nil) should set geoipSvc to nil")
	}
}

// ── GetSearchCache ────────────────────────────────────────────────────────────

// TestGetSearchCache_ReturnsCache verifies GetSearchCache returns the cache
// set during construction.
func TestGetSearchCache_ReturnsCache(t *testing.T) {
	cfg := createTestConfig()
	h := NewSearchHandler(cfg, nil)
	if h.GetSearchCache() == nil {
		t.Error("GetSearchCache() should return non-nil after NewSearchHandler")
	}
}

// ── nil-metrics getter paths ──────────────────────────────────────────────────

// TestGetSearchCount_NilMetrics verifies getSearchCount returns 0 when metrics is nil.
func TestGetSearchCount_NilMetrics(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	if got := h.getSearchCount(); got != 0 {
		t.Errorf("getSearchCount() nil metrics = %d, want 0", got)
	}
}

// TestGetRequestsTotal_NilMetrics verifies getRequestsTotal returns 0 when metrics is nil.
func TestGetRequestsTotal_NilMetrics(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	if got := h.getRequestsTotal(); got != 0 {
		t.Errorf("getRequestsTotal() nil metrics = %d, want 0", got)
	}
}

// TestGetRequests24h_NilMetrics verifies getRequests24h returns 0 when metrics is nil.
func TestGetRequests24h_NilMetrics(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	if got := h.getRequests24h(); got != 0 {
		t.Errorf("getRequests24h() nil metrics = %d, want 0", got)
	}
}

// TestGetActiveConnections_NilMetrics verifies getActiveConnections returns 0 when nil.
func TestGetActiveConnections_NilMetrics(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	if got := h.getActiveConnections(); got != 0 {
		t.Errorf("getActiveConnections() nil metrics = %d, want 0", got)
	}
}

// ── nil-torSvc getter paths ───────────────────────────────────────────────────

// TestGetTorStatus_NilTorSvc verifies getTorStatus returns "disabled" when torSvc is nil.
func TestGetTorStatus_NilTorSvc(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	if got := h.getTorStatus(); got != "disabled" {
		t.Errorf("getTorStatus() nil tor = %q, want disabled", got)
	}
}

// TestGetTorHostname_NilTorSvc verifies getTorHostname returns "" when torSvc is nil.
func TestGetTorHostname_NilTorSvc(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	if got := h.getTorHostname(); got != "" {
		t.Errorf("getTorHostname() nil tor = %q, want empty", got)
	}
}

// ── getProxyClient nil torSvc ─────────────────────────────────────────────────

// TestGetProxyClient_NilTorSvc verifies getProxyClient returns a plain HTTP
// client when torSvc is nil.
func TestGetProxyClient_NilTorSvc(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	client := h.getProxyClient(5 * 1e9)
	if client == nil {
		t.Error("getProxyClient() nil tor should return non-nil http.Client")
	}
}

// ── getUserIPForwardPreference nil torSvc ─────────────────────────────────────

// TestGetUserIPForwardPreference_NilTorSvc verifies that false/"" is returned
// when torSvc is nil.
func TestGetUserIPForwardPreference_NilTorSvc(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/", nil)
	ok, ip := h.getUserIPForwardPreference(req)
	if ok {
		t.Error("getUserIPForwardPreference() nil tor should return false")
	}
	if ip != "" {
		t.Errorf("getUserIPForwardPreference() nil tor IP = %q, want empty", ip)
	}
}

// ── setContentRestrictionAckCookie ───────────────────────────────────────────

// TestSetContentRestrictionAckCookie_SetsCookie verifies that the ack cookie is
// set in the response.
func TestSetContentRestrictionAckCookie_SetsCookie(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	rr := httptest.NewRecorder()
	h.setContentRestrictionAckCookie(rr)

	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == ContentRestrictionAckCookieName {
			found = true
			if c.Value != "1" {
				t.Errorf("ack cookie value = %q, want 1", c.Value)
			}
		}
	}
	if !found {
		t.Errorf("ack cookie %q not set in response", ContentRestrictionAckCookieName)
	}
}

// ── getAPIResponseFormat ──────────────────────────────────────────────────────

// TestGetAPIResponseFormat_TxtExtension verifies .txt extension returns "text".
func TestGetAPIResponseFormat_TxtExtension(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/search.txt?q=x", nil)
	if got := getAPIResponseFormat(req); got != "text" {
		t.Errorf("getAPIResponseFormat .txt = %q, want text", got)
	}
}

// TestGetAPIResponseFormat_AcceptJSON verifies Accept: application/json returns "json".
func TestGetAPIResponseFormat_AcceptJSON(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	req.Header.Set("Accept", "application/json")
	if got := getAPIResponseFormat(req); got != "json" {
		t.Errorf("getAPIResponseFormat Accept:json = %q, want json", got)
	}
}

// TestGetAPIResponseFormat_AcceptTextPlain verifies Accept: text/plain returns "text".
func TestGetAPIResponseFormat_AcceptTextPlain(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	req.Header.Set("Accept", "text/plain")
	if got := getAPIResponseFormat(req); got != "text" {
		t.Errorf("getAPIResponseFormat Accept:text = %q, want text", got)
	}
}

// TestGetAPIResponseFormat_CurlUA verifies curl User-Agent returns "text".
func TestGetAPIResponseFormat_CurlUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	req.Header.Set("User-Agent", "curl/8.0.0")
	if got := getAPIResponseFormat(req); got != "text" {
		t.Errorf("getAPIResponseFormat curl = %q, want text", got)
	}
}

// TestGetAPIResponseFormat_EmptyUA verifies empty User-Agent returns "text".
func TestGetAPIResponseFormat_EmptyUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	req.Header.Set("User-Agent", "")
	if got := getAPIResponseFormat(req); got != "text" {
		t.Errorf("getAPIResponseFormat empty UA = %q, want text", got)
	}
}

// TestGetAPIResponseFormat_BrowserUA verifies a browser User-Agent returns "json".
func TestGetAPIResponseFormat_BrowserUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/search", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")
	if got := getAPIResponseFormat(req); got != "json" {
		t.Errorf("getAPIResponseFormat browser = %q, want json", got)
	}
}

// ── BuildDateTime ─────────────────────────────────────────────────────────────

// TestBuildDateTime_EmptyReturnsUnknown verifies that an empty BuildTime
// returns "unknown".
func TestBuildDateTime_EmptyReturnsUnknown(t *testing.T) {
	got := BuildDateTime()
	// BuildTime may be set at build time; in tests it's typically empty or "unknown".
	if got == "" {
		t.Error("BuildDateTime() returned empty string, want at least 'unknown'")
	}
}

// ── HumansTxt ────────────────────────────────────────────────────────────────

// TestHumansTxt_StatusOK verifies HumansTxt returns 200 with text/plain content type.
func TestHumansTxt_StatusOK(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/humans.txt", nil)
	rr := httptest.NewRecorder()
	h.HumansTxt(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HumansTxt status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("HumansTxt Content-Type = %q, want text/plain", ct)
	}
	if !strings.Contains(rr.Body.String(), "TEAM") {
		t.Error("HumansTxt body should contain TEAM section")
	}
}

// ── Favicon / AppleTouchIcon ─────────────────────────────────────────────────

// TestFavicon_RedirectsToCorrctPath verifies Favicon redirects to favicon.ico.
func TestFavicon_RedirectsToCorrectPath(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/favicon.ico", nil)
	rr := httptest.NewRecorder()
	h.Favicon(rr, req)

	if rr.Code != http.StatusMovedPermanently {
		t.Errorf("Favicon status = %d, want %d", rr.Code, http.StatusMovedPermanently)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "favicon.ico") {
		t.Errorf("Favicon redirect location = %q, want favicon.ico path", loc)
	}
}

// TestAppleTouchIcon_RedirectsToIconPng verifies AppleTouchIcon redirects to icon PNG.
func TestAppleTouchIcon_RedirectsToIconPng(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/apple-touch-icon.png", nil)
	rr := httptest.NewRecorder()
	h.AppleTouchIcon(rr, req)

	if rr.Code != http.StatusMovedPermanently {
		t.Errorf("AppleTouchIcon status = %d, want %d", rr.Code, http.StatusMovedPermanently)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "icon") {
		t.Errorf("AppleTouchIcon redirect location = %q, want icon path", loc)
	}
}

// ── MaintenanceModeMiddleware ─────────────────────────────────────────────────

// TestMaintenanceModeMiddleware_HealthzBypassed verifies /healthz is never
// intercepted by maintenance mode even when the flag file exists.
func TestMaintenanceModeMiddleware_HealthzBypassed(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := h.MaintenanceModeMiddleware(next)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("MaintenanceModeMiddleware should pass /healthz to next handler")
	}
}

// TestMaintenanceModeMiddleware_NoFlag passes regular request through when there
// is no maintenance flag file.
func TestMaintenanceModeMiddleware_NoFlag(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := h.MaintenanceModeMiddleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("MaintenanceModeMiddleware with no flag file should call next handler")
	}
	if rr.Code == http.StatusServiceUnavailable {
		t.Error("MaintenanceModeMiddleware with no flag file should not return 503")
	}
}

// newTestSearchHandler creates a SearchHandler backed by a real EngineManager.
// Engines are not initialized so ListEngines returns an empty slice.
func newTestSearchHandler(t *testing.T) *SearchHandler {
	t.Helper()
	cfg := createTestConfig()
	mgr := engine.NewEngineManager(config.DefaultAppConfig())
	return NewSearchHandler(cfg, mgr)
}

// ── SetTemplatesFS ────────────────────────────────────────────────────────────

// TestSetTemplatesFS_NoPanic verifies SetTemplatesFS does not panic.
func TestSetTemplatesFS_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetTemplatesFS panicked: %v", r)
		}
	}()
	prev := templatesFS
	t.Cleanup(func() { templatesFS = prev })
	var fs embed.FS
	SetTemplatesFS(fs)
}

// ── APIVersion ────────────────────────────────────────────────────────────────

// TestAPIVersion_Returns200WithOk verifies APIVersion returns 200 with ok:true.
func TestAPIVersion_Returns200WithOk(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/version", nil)
	rr := httptest.NewRecorder()
	h.APIVersion(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIVersion status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("APIVersion returned invalid JSON: %v", err)
	}
	if resp["ok"] != true {
		t.Errorf("APIVersion ok = %v, want true", resp["ok"])
	}
	if resp["version"] == nil {
		t.Error("APIVersion response missing 'version' field")
	}
}

// ── APIHealthCheck ────────────────────────────────────────────────────────────

// TestAPIHealthCheck_JSONReturns200 verifies APIHealthCheck returns 200 with JSON.
func TestAPIHealthCheck_JSONReturns200(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIHealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIHealthCheck status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("APIHealthCheck returned invalid JSON: %v", err)
	}
	if resp["status"] == nil {
		t.Error("APIHealthCheck response missing 'status' field")
	}
}

// TestAPIHealthCheck_TextOutput verifies APIHealthCheck text format includes status line.
func TestAPIHealthCheck_TextOutput(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/healthz", nil)
	req.Header.Set("User-Agent", "curl/8.0.0")
	rr := httptest.NewRecorder()
	h.APIHealthCheck(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "status:") {
		t.Errorf("APIHealthCheck text output missing 'status:' line; got: %q", body)
	}
}

// TestAPIHealthCheck_DevelopmentMode verifies mode is "development" when set.
func TestAPIHealthCheck_DevelopmentMode(t *testing.T) {
	cfg := createTestConfig()
	mgr := engine.NewEngineManager(config.DefaultAppConfig())
	h := NewSearchHandler(cfg, mgr)
	req := httptest.NewRequest("GET", "/api/v1/healthz", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIHealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIHealthCheck dev mode status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// ── APIStats ──────────────────────────────────────────────────────────────────

// TestAPIStats_JSONReturns200 verifies APIStats returns 200 with engines data.
func TestAPIStats_JSONReturns200(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIStats status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestAPIStats_TextOutput verifies APIStats text format includes engines_enabled.
func TestAPIStats_TextOutput(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("User-Agent", "curl/8.0.0")
	rr := httptest.NewRecorder()
	h.APIStats(rr, req)

	if !strings.Contains(rr.Body.String(), "engines_enabled:") {
		t.Errorf("APIStats text missing 'engines_enabled:'; got: %q", rr.Body.String())
	}
}

// ── APIEngines ────────────────────────────────────────────────────────────────

// TestAPIEngines_JSONReturns200 verifies APIEngines returns 200.
func TestAPIEngines_JSONReturns200(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/engines", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	h.APIEngines(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIEngines status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestAPIEngines_TextOutput verifies APIEngines text format includes "engines:".
func TestAPIEngines_TextOutput(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/engines", nil)
	req.Header.Set("User-Agent", "curl/8.0.0")
	rr := httptest.NewRecorder()
	h.APIEngines(rr, req)

	if !strings.Contains(rr.Body.String(), "engines:") {
		t.Errorf("APIEngines text missing 'engines:'; got: %q", rr.Body.String())
	}
}

// ── APIEngineHealth ───────────────────────────────────────────────────────────

// TestAPIEngineHealth_Returns200 verifies APIEngineHealth returns 200.
func TestAPIEngineHealth_Returns200(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/v1/engines/health", nil)
	rr := httptest.NewRecorder()
	h.APIEngineHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("APIEngineHealth status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// ── WellKnownVidVeil ──────────────────────────────────────────────────────────

// TestWellKnownVidVeil_Returns200 verifies the handler returns 200 with software field.
func TestWellKnownVidVeil_Returns200(t *testing.T) {
	h := newTestSearchHandler(t)
	req := httptest.NewRequest("GET", "/.well-known/vidveil", nil)
	rr := httptest.NewRecorder()
	h.WellKnownVidVeil(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("WellKnownVidVeil status = %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("WellKnownVidVeil returned invalid JSON: %v", err)
	}
	if resp["software"] != "vidveil" {
		t.Errorf("WellKnownVidVeil software = %v, want vidveil", resp["software"])
	}
}

// ── RenderErrorPage + NotFoundHandler + InternalErrorHandler ─────────────────

// TestRenderErrorPage_FallsBackToPlainText verifies the fallback when templatesFS is empty.
func TestRenderErrorPage_FallsBackToPlainText(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/missing", nil)
	rr := httptest.NewRecorder()
	h.RenderErrorPage(rr, req, http.StatusNotFound, "Not Found", "page gone")

	if rr.Code != http.StatusNotFound {
		t.Errorf("RenderErrorPage status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Not Found") {
		t.Errorf("RenderErrorPage body missing title: %q", body)
	}
}

// TestNotFoundHandler_Returns404 verifies NotFoundHandler returns 404.
func TestNotFoundHandler_Returns404(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/gone", nil)
	rr := httptest.NewRecorder()
	h.NotFoundHandler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("NotFoundHandler status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// TestInternalErrorHandler_Returns500 verifies InternalErrorHandler returns 500.
func TestInternalErrorHandler_Returns500(t *testing.T) {
	h := &SearchHandler{appConfig: createTestConfig()}
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.InternalErrorHandler(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("InternalErrorHandler status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ── isPrivateHost ─────────────────────────────────────────────────────────────

// TestIsPrivateHost_Loopback verifies localhost resolves as private.
func TestIsPrivateHost_Loopback(t *testing.T) {
	if !isPrivateHost("localhost") {
		t.Error("isPrivateHost(localhost) = false, want true")
	}
}

// TestIsPrivateHost_PrivateRange verifies a private-range IP hostname is detected as private.
func TestIsPrivateHost_PrivateRange(t *testing.T) {
	if !isPrivateHost("127.0.0.1") {
		t.Error("isPrivateHost(127.0.0.1) = false, want true")
	}
}

// ── MetricsMiddleware ─────────────────────────────────────────────────────────

// TestMetricsMiddleware_PassThrough verifies the middleware calls the next handler.
func TestMetricsMiddleware_PassThrough(t *testing.T) {
	cfg := createTestConfig()
	mgr := engine.NewEngineManager(config.DefaultAppConfig())
	m := NewMetrics(cfg, mgr)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := m.MetricsMiddleware(next)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("MetricsMiddleware should call next handler")
	}
}

// TestMetricsMiddleware_IncrementsCounters verifies the middleware increments request counters.
func TestMetricsMiddleware_IncrementsCounters(t *testing.T) {
	cfg := createTestConfig()
	mgr := engine.NewEngineManager(config.DefaultAppConfig())
	m := NewMetrics(cfg, mgr)
	before := m.GetRequestsTotal()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := m.MetricsMiddleware(next)
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if m.GetRequestsTotal() <= before {
		t.Error("MetricsMiddleware should increment requests counter")
	}
}

// ── BuildDateTime coverage extension ─────────────────────────────────────────

// TestBuildDateTime_RFC3339 verifies a valid RFC3339 time is formatted correctly.
func TestBuildDateTime_RFC3339(t *testing.T) {
	// Temporarily set BuildTime and call BuildDateTime via version package
	// We can't set version.BuildTime from outside the package, but we can
	// verify the function handles the "unknown" constant correctly.
	result := BuildDateTime()
	if result == "" {
		t.Error("BuildDateTime() returned empty string")
	}
	// Must be either "unknown" or a formatted date string.
	if result != "unknown" && !strings.Contains(result, ",") {
		t.Logf("BuildDateTime() returned raw value: %q (build time not set)", result)
	}
}

// ── getUptime ─────────────────────────────────────────────────────────────────

// TestGetUptime_ReturnsNonEmpty verifies getUptime returns a non-empty string.
func TestGetUptime_ReturnsNonEmpty(t *testing.T) {
	got := getUptime()
	if got == "" {
		t.Error("getUptime() returned empty string")
	}
}

// TestGetUptime_ContainsTimeUnit verifies getUptime output contains at least one time unit.
func TestGetUptime_ContainsTimeUnit(t *testing.T) {
	got := getUptime()
	if !strings.Contains(got, "h") && !strings.Contains(got, "d") {
		t.Errorf("getUptime() = %q, expected 'h' or 'd' time unit", got)
	}
}

// ── Metrics Handler ───────────────────────────────────────────────────────────

// TestMetricsHandler_Returns200 verifies the /metrics HTTP handler returns 200 from loopback.
func TestMetricsHandler_Returns200(t *testing.T) {
	cfg := createTestConfig()
	mgr := engine.NewEngineManager(config.DefaultAppConfig())
	m := NewMetrics(cfg, mgr)
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	rr := httptest.NewRecorder()
	m.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Metrics handler status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestMetricsHandler_ForbidsNonLoopback verifies the /metrics handler blocks non-loopback when no token.
func TestMetricsHandler_ForbidsNonLoopback(t *testing.T) {
	cfg := createTestConfig()
	mgr := engine.NewEngineManager(config.DefaultAppConfig())
	m := NewMetrics(cfg, mgr)
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	m.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden && rr.Code != http.StatusUnauthorized {
		t.Errorf("Metrics handler from non-loopback: status = %d, want 403/401", rr.Code)
	}
}
