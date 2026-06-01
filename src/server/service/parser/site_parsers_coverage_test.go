// SPDX-License-Identifier: MIT
// Tests for Parse() method on each of the 6 site-specific parsers, plus
// extractViewsFromText from xvideos.go.
package parser

import (
	"testing"
)

// ---- PornHubParser.Parse ----

// No <a> tag → no href → Parse returns nil (early return guard).
func TestPornHubParse_NoLink(t *testing.T) {
	p := NewPornHubParser()
	sel := newDoc(`<li class="videoBox"><span class="title">Video</span></li>`).Find("li.videoBox")
	if p.Parse(sel) != nil {
		t.Error("PornHubParser.Parse: expected nil when no href present")
	}
}

// Valid minimal HTML yields a non-nil VideoItem with URL and Title populated.
func TestPornHubParse_Valid(t *testing.T) {
	p := NewPornHubParser()
	html := `<li class="videoBox">
		<a class="linkVideoThumb" href="/video/123" title="My PH Video">
			<img src="thumb.jpg">
		</a>
		<span class="title"><a title="My PH Video" href="/video/123">My PH Video</a></span>
		<var class="duration">5:30</var>
	</li>`
	sel := newDoc(html).Find("li.videoBox")
	item := p.Parse(sel)
	if item == nil {
		t.Fatal("PornHubParser.Parse: expected non-nil VideoItem for valid HTML")
	}
	if item.URL == "" {
		t.Error("PornHubParser.Parse: URL must not be empty")
	}
	if item.Title == "" {
		t.Error("PornHubParser.Parse: Title must not be empty")
	}
}

// ---- XVideosParser.Parse ----

// No <a> tag → no href → Parse returns nil.
func TestXVideosParse_NoLink(t *testing.T) {
	p := NewXVideosParser()
	sel := newDoc(`<div class="thumb-block"><p class="title">Video</p></div>`).Find("div.thumb-block")
	if p.Parse(sel) != nil {
		t.Error("XVideosParser.Parse: expected nil when no href present")
	}
}

// Valid minimal HTML with href and title attribute yields a non-nil VideoItem.
func TestXVideosParse_Valid(t *testing.T) {
	p := NewXVideosParser()
	html := `<div class="thumb-block">
		<a href="/video/abc123" title="My XV Video">
			<img data-mzl="thumb.jpg" src="blank.gif">
		</a>
		<p class="title"><a href="/video/abc123" title="My XV Video">My XV Video</a></p>
	</div>`
	sel := newDoc(html).Find("div.thumb-block")
	item := p.Parse(sel)
	if item == nil {
		t.Fatal("XVideosParser.Parse: expected non-nil VideoItem for valid HTML")
	}
	if item.URL == "" {
		t.Error("XVideosParser.Parse: URL must not be empty")
	}
	if item.Title == "" {
		t.Error("XVideosParser.Parse: Title must not be empty")
	}
}

// ---- XNXXParser.Parse ----

// No <a href> anywhere → Parse returns nil.
func TestXNXXParse_NoLink(t *testing.T) {
	p := NewXNXXParser()
	sel := newDoc(`<div class="thumb-block"><p>no link here</p></div>`).Find("div.thumb-block")
	if p.Parse(sel) != nil {
		t.Error("XNXXParser.Parse: expected nil when no href present")
	}
}

// Valid minimal HTML using the primary selector yields a non-nil VideoItem.
func TestXNXXParse_Valid(t *testing.T) {
	p := NewXNXXParser()
	html := `<div class="thumb-block">
		<div class="thumb-inside">
			<img data-src="//cdn.xnxx.com/thumb.jpg">
		</div>
		<div class="thumb-under">
			<p><a href="/video-xyz123/title.html" title="My XNXX Video">My XNXX Video</a></p>
		</div>
	</div>`
	sel := newDoc(html).Find("div.thumb-block")
	item := p.Parse(sel)
	if item == nil {
		t.Fatal("XNXXParser.Parse: expected non-nil VideoItem for valid HTML")
	}
	if item.URL == "" {
		t.Error("XNXXParser.Parse: URL must not be empty")
	}
	if item.Title == "" {
		t.Error("XNXXParser.Parse: Title must not be empty")
	}
}

// ---- RedTubeParser.Parse ----

// No title element → no title → Parse returns nil.
func TestRedTubeParse_NoTitle(t *testing.T) {
	p := NewRedTubeParser()
	sel := newDoc(`<li class="videoblock_list"><a href="/video/1">thumb</a></li>`).Find("li.videoblock_list")
	if p.Parse(sel) != nil {
		t.Error("RedTubeParser.Parse: expected nil when no title element present")
	}
}

// Valid minimal HTML with a.video-title-text yields a non-nil VideoItem.
func TestRedTubeParse_Valid(t *testing.T) {
	p := NewRedTubeParser()
	html := `<li class="videoblock_list">
		<a class="video_link" href="/video/42">
			<img class="js_thumbImageTag" data-src="//cdn.redtube.com/thumb.jpg">
		</a>
		<a class="video-title-text" href="/video/42">My RedTube Video</a>
	</li>`
	sel := newDoc(html).Find("li.videoblock_list")
	item := p.Parse(sel)
	if item == nil {
		t.Fatal("RedTubeParser.Parse: expected non-nil VideoItem for valid HTML")
	}
	if item.URL == "" {
		t.Error("RedTubeParser.Parse: URL must not be empty")
	}
	if item.Title == "" {
		t.Error("RedTubeParser.Parse: Title must not be empty")
	}
}

// ---- EpornerParser.Parse ----

// No <a> tag → no href → Parse returns nil.
func TestEpornerParse_NoLink(t *testing.T) {
	p := NewEpornerParser()
	sel := newDoc(`<div class="mb"><span>nothing</span></div>`).Find("div.mb")
	if p.Parse(sel) != nil {
		t.Error("EpornerParser.Parse: expected nil when no href present")
	}
}

// Valid minimal HTML with an anchor that has title and href yields a non-nil VideoItem.
func TestEpornerParse_Valid(t *testing.T) {
	p := NewEpornerParser()
	html := `<div class="mb">
		<a href="/video/ep123/" title="My Eporner Video">
			<img data-src="thumb.jpg">
		</a>
	</div>`
	sel := newDoc(html).Find("div.mb")
	item := p.Parse(sel)
	if item == nil {
		t.Fatal("EpornerParser.Parse: expected non-nil VideoItem for valid HTML")
	}
	if item.URL == "" {
		t.Error("EpornerParser.Parse: URL must not be empty")
	}
	if item.Title == "" {
		t.Error("EpornerParser.Parse: Title must not be empty")
	}
}

// ---- PornMDParser.Parse ----

// No <a> tag → no href → Parse returns nil.
func TestPornMDParse_NoLink(t *testing.T) {
	p := NewPornMDParser()
	sel := newDoc(`<div class="card sub"><span>nothing</span></div>`).Find("div.card.sub")
	if p.Parse(sel) != nil {
		t.Error("PornMDParser.Parse: expected nil when no href present")
	}
}

// Valid minimal HTML with a.item-link having title and href yields a non-nil VideoItem.
func TestPornMDParse_Valid(t *testing.T) {
	p := NewPornMDParser()
	html := `<div class="card sub">
		<a class="item-link" href="/out/pornmd123" title="My PornMD Video">
			<img class="item-image" src="thumb.jpg" alt="My PornMD Video">
		</a>
	</div>`
	sel := newDoc(html).Find("div.card.sub")
	item := p.Parse(sel)
	if item == nil {
		t.Fatal("PornMDParser.Parse: expected non-nil VideoItem for valid HTML")
	}
	if item.URL == "" {
		t.Error("PornMDParser.Parse: URL must not be empty")
	}
	if item.Title == "" {
		t.Error("PornMDParser.Parse: Title must not be empty")
	}
}

// ---- extractViewsFromText (xvideos.go, unexported) ----

// "6.4M Views" → viewStr "6.4M views", count ~6400000.
func TestExtractViewsFromText_Millions(t *testing.T) {
	viewStr, count := extractViewsFromText("10 min Gabiconkey - 6.4M Views -")
	if viewStr != "6.4M views" {
		t.Errorf("extractViewsFromText millions viewStr = %q, want %q", viewStr, "6.4M views")
	}
	if count < 6000000 || count > 7000000 {
		t.Errorf("extractViewsFromText millions count = %d, want ~6400000", count)
	}
}

// "500K Views" → count ~500000.
func TestExtractViewsFromText_Thousands(t *testing.T) {
	viewStr, count := extractViewsFromText("8 min SomeModel - 500K Views -")
	if viewStr != "500K views" {
		t.Errorf("extractViewsFromText thousands viewStr = %q, want %q", viewStr, "500K views")
	}
	if count < 450000 || count > 550000 {
		t.Errorf("extractViewsFromText thousands count = %d, want ~500000", count)
	}
}

// No match → returns ("", 0).
func TestExtractViewsFromText_NoMatch(t *testing.T) {
	viewStr, count := extractViewsFromText("no view count here")
	if viewStr != "" {
		t.Errorf("extractViewsFromText no match viewStr = %q, want empty string", viewStr)
	}
	if count != 0 {
		t.Errorf("extractViewsFromText no match count = %d, want 0", count)
	}
}
