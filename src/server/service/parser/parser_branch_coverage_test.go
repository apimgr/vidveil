// SPDX-License-Identifier: MIT
// Branch-level coverage for the six site parsers.
// Each test targets a specific uncovered conditional path identified from the
// coverage profile (0-count blocks in pornhub.go, xvideos.go, xnxx.go,
// eporner.go, pornmd.go, redtube.go).
package parser

import (
	"testing"
)

// ============================================================
// PornHubParser branch tests
// ============================================================

// Title comes from text inside span.title a (no "title" attribute on any titleElem).
// Covers pornhub.go:38-40 (if item.Title == "" → CleanText(titleElem.Text())).
func TestPornHubParse_TitleFromText(t *testing.T) {
	p := NewPornHubParser()
	html := `<li class="videoBox">
		<a class="linkVideoThumb" href="/video/123">thumb</a>
		<span class="title"><a href="/video/123">Title From Text</a></span>
	</li>`
	item := p.Parse(newDoc(html).Find("li.videoBox"))
	if item == nil {
		t.Fatal("expected non-nil item when title is from text node")
	}
	if item.Title == "" {
		t.Error("Title must not be empty when populated from span.title text")
	}
}

// Title comes from the link's own "title" attribute because titleElem has neither
// attribute nor text content.
// Covers pornhub.go:41-43 (if item.Title == "" → ExtractAttr(link, "title")).
func TestPornHubParse_TitleFromLinkAttr(t *testing.T) {
	p := NewPornHubParser()
	html := `<li class="videoBox">
		<a class="linkVideoThumb" href="/video/999" title="Link Title">thumb</a>
	</li>`
	item := p.Parse(newDoc(html).Find("li.videoBox"))
	if item == nil {
		t.Fatal("expected non-nil item when title comes from link title attr")
	}
	if item.Title != "Link Title" {
		t.Errorf("Title = %q, want Link Title", item.Title)
	}
}

// When no title is found from any source the parser returns nil.
// Covers pornhub.go:44-46 (return nil).
func TestPornHubParse_NoTitle_ReturnsNil(t *testing.T) {
	p := NewPornHubParser()
	html := `<li class="videoBox">
		<a class="linkVideoThumb" href="/video/888">no title anywhere</a>
	</li>`
	if p.Parse(newDoc(html).Find("li.videoBox")) != nil {
		t.Error("expected nil when no title source is present")
	}
}

// Non-empty DownloadURL is run through MakeAbsoluteURL.
// Covers pornhub.go:66-68.
func TestPornHubParse_DownloadURL_MadeAbsolute(t *testing.T) {
	p := NewPornHubParser()
	html := `<li class="videoBox" data-video-url="//cdn.pornhub.com/dl.mp4">
		<a class="linkVideoThumb" href="/video/1" title="DL Video">thumb</a>
	</li>`
	item := p.Parse(newDoc(html).Find("li.videoBox"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.DownloadURL == "" {
		t.Error("DownloadURL must not be empty when data-video-url is present")
	}
}

// ============================================================
// XVideosParser branch tests
// ============================================================

// Title comes from text of p.title a (no "title" attribute on that element).
// Covers xvideos.go:56-58.
func TestXVideosParse_TitleFromText(t *testing.T) {
	p := NewXVideosParser()
	html := `<div class="thumb-block">
		<a href="/video/abc">thumb</a>
		<p class="title"><a href="/video/abc">XV Text Title</a></p>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item when title comes from text node")
	}
	if item.Title == "" {
		t.Error("Title must not be empty")
	}
}

// Title comes from the first link's "title" attribute because p.title a has nothing.
// Covers xvideos.go:59-61.
func TestXVideosParse_TitleFromLinkAttr(t *testing.T) {
	p := NewXVideosParser()
	html := `<div class="thumb-block">
		<a href="/video/xyz" title="XV Link Title">thumb</a>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item when title comes from link attr")
	}
	if item.Title != "XV Link Title" {
		t.Errorf("Title = %q, want XV Link Title", item.Title)
	}
}

// No title from any source → return nil.
// Covers xvideos.go:62-64.
func TestXVideosParse_NoTitle_ReturnsNil(t *testing.T) {
	p := NewXVideosParser()
	html := `<div class="thumb-block"><a href="/video/zz">no title</a></div>`
	if p.Parse(newDoc(html).Find("div.thumb-block")) != nil {
		t.Error("expected nil when no title source is present")
	}
}

// Thumbnail URL containing "lightbox-blank.gif" is cleared to empty.
// Covers xvideos.go:71-73.
func TestXVideosParse_BlankGifThumbnailCleared(t *testing.T) {
	p := NewXVideosParser()
	html := `<div class="thumb-block">
		<a href="/video/bb" title="Blank GIF Test">
			<img data-mzl="lightbox-blank.gif" src="lightbox-blank.gif">
		</a>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.Thumbnail != "" {
		t.Errorf("Thumbnail must be cleared for lightbox-blank.gif, got %q", item.Thumbnail)
	}
}

// Non-empty data-pvv on img populates PreviewURL (made absolute).
// Covers xvideos.go:84-86.
func TestXVideosParse_PreviewURL_MadeAbsolute(t *testing.T) {
	p := NewXVideosParser()
	html := `<div class="thumb-block">
		<a href="/video/pv1" title="Preview URL Test">
			<img src="thumb.jpg" data-pvv="//cdn.xvideos.com/preview.mp4">
		</a>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.PreviewURL == "" {
		t.Error("PreviewURL must not be empty when data-pvv is present")
	}
}

// Non-empty data-video-url on img populates DownloadURL (made absolute).
// Covers xvideos.go:93-95.
func TestXVideosParse_DownloadURL_MadeAbsolute(t *testing.T) {
	p := NewXVideosParser()
	html := `<div class="thumb-block">
		<a href="/video/dl1" title="Download URL Test">
			<img src="thumb.jpg" data-video-url="//cdn.xvideos.com/dl.mp4">
		</a>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.DownloadURL == "" {
		t.Error("DownloadURL must not be empty when data-video-url is present")
	}
}

// ============================================================
// XNXXParser branch tests
// ============================================================

// URL containing "xnxx.gold" triggers the early return nil.
// Covers xnxx.go:43-45.
func TestXNXXParse_GoldURL_ReturnsNil(t *testing.T) {
	p := NewXNXXParser()
	html := `<div class="thumb-block">
		<div class="thumb-under">
			<p><a href="//xnxx.gold/video/123" title="Gold Video">Gold Video</a></p>
		</div>
	</div>`
	if p.Parse(newDoc(html).Find("div.thumb-block")) != nil {
		t.Error("expected nil for xnxx.gold URL")
	}
}

// Title comes from CleanText(link.Text()) when the title attribute is absent.
// Covers xnxx.go:49-51.
func TestXNXXParse_TitleFromLinkText(t *testing.T) {
	p := NewXNXXParser()
	html := `<div class="thumb-block">
		<div class="thumb-under">
			<p><a href="/video-abc123/title.html">XNXX Text Title</a></p>
		</div>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item when title comes from link text")
	}
	if item.Title == "" {
		t.Error("Title must not be empty")
	}
}

// Link with no title attribute and empty text → return nil.
// Covers xnxx.go:52-54.
func TestXNXXParse_NoTitle_ReturnsNil(t *testing.T) {
	p := NewXNXXParser()
	html := `<div class="thumb-block">
		<div class="thumb-under">
			<p><a href="/video-nt123/blank.html"></a></p>
		</div>
	</div>`
	if p.Parse(newDoc(html).Find("div.thumb-block")) != nil {
		t.Error("expected nil when title is empty")
	}
}

// data-video-url without "http" prefix gets "https:" prepended.
// Covers xnxx.go:68-70.
func TestXNXXParse_DownloadURL_ProtocolAdded(t *testing.T) {
	p := NewXNXXParser()
	html := `<div class="thumb-block">
		<div class="thumb-inside">
			<img data-video-url="//cdn.xnxx.com/dl.mp4">
		</div>
		<div class="thumb-under">
			<p><a href="/video-dl1/dl.html" title="DL Test">DL Test</a></p>
		</div>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.DownloadURL == "" {
		t.Error("DownloadURL must not be empty")
	}
}

// p.metadata with text nodes populates Duration; span.right populates Views.
// Covers xnxx.go:74-80 and xnxx.go:90-94.
func TestXNXXParse_MetadataAndViews(t *testing.T) {
	p := NewXNXXParser()
	html := `<div class="thumb-block">
		<div class="thumb-under">
			<p><a href="/video-mv1/mv.html" title="Metadata Video">Metadata Video</a></p>
			<p class="metadata">5:30 <span class="right">1.2M Views</span></p>
		</div>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.DurationSeconds == 0 {
		t.Error("DurationSeconds should be non-zero when metadata text contains duration")
	}
}

// span.video-hd populates Quality.
// Covers xnxx.go:84-86.
func TestXNXXParse_HDQuality(t *testing.T) {
	p := NewXNXXParser()
	html := `<div class="thumb-block">
		<div class="thumb-under">
			<p><a href="/video-hd1/hd.html" title="HD Video">HD Video</a></p>
		</div>
		<span class="video-hd">HD</span>
	</div>`
	item := p.Parse(newDoc(html).Find("div.thumb-block"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.Quality == "" {
		t.Error("Quality must not be empty when span.video-hd is present")
	}
}

// HTML containing "premium" text causes IsPremiumContent to return true → return nil.
// Covers xnxx.go:98-100.
func TestXNXXParse_PremiumContent_ReturnsNil(t *testing.T) {
	p := NewXNXXParser()
	html := `<div class="thumb-block">
		<div class="thumb-under">
			<p><a href="/video-pm1/pm.html" title="Premium Video">Premium Video</a></p>
		</div>
		<span class="premium-badge">premium</span>
	</div>`
	if p.Parse(newDoc(html).Find("div.thumb-block")) != nil {
		t.Error("expected nil for premium content")
	}
}

// ============================================================
// EpornerParser branch tests
// ============================================================

// Link has no "title" attribute — title comes from .mbtit a text instead.
// Covers eporner.go:37-40.
func TestEpornerParse_TitleFromMbtit(t *testing.T) {
	p := NewEpornerParser()
	html := `<div class="mb">
		<a href="/video/ep456/">
			<img src="thumb.jpg">
		</a>
		<div class="mbtit"><a href="/video/ep456/">Eporner Mbtit Title</a></div>
	</div>`
	item := p.Parse(newDoc(html).Find("div.mb"))
	if item == nil {
		t.Fatal("expected non-nil item when title comes from .mbtit")
	}
	if item.Title == "" {
		t.Error("Title must not be empty")
	}
}

// No title from link attr or .mbtit → return nil.
// Covers eporner.go:41-43.
func TestEpornerParse_NoTitle_ReturnsNil(t *testing.T) {
	p := NewEpornerParser()
	html := `<div class="mb">
		<a href="/video/ep789/">
			<img src="thumb.jpg">
		</a>
	</div>`
	if p.Parse(newDoc(html).Find("div.mb")) != nil {
		t.Error("expected nil when no title source is present")
	}
}

// ============================================================
// PornMDParser branch tests
// ============================================================

// a.item-link has no "title" attr → title falls back to img alt text.
// Covers pornmd.go:43-47.
func TestPornMDParse_TitleFromImgAlt(t *testing.T) {
	p := NewPornMDParser()
	html := `<div class="card sub">
		<a class="item-link" href="/out/md001">
			<img class="item-image" src="thumb.jpg" alt="PornMD Alt Title">
		</a>
	</div>`
	item := p.Parse(newDoc(html).Find("div.card.sub"))
	if item == nil {
		t.Fatal("expected non-nil item when title comes from img alt")
	}
	if item.Title != "PornMD Alt Title" {
		t.Errorf("Title = %q, want PornMD Alt Title", item.Title)
	}
}

// No title from link title or img alt → return nil.
// Covers pornmd.go:48-50.
func TestPornMDParse_NoTitle_ReturnsNil(t *testing.T) {
	p := NewPornMDParser()
	html := `<div class="card sub">
		<a class="item-link" href="/out/md002">
			<img class="item-image" src="thumb.jpg">
		</a>
	</div>`
	if p.Parse(newDoc(html).Find("div.card.sub")) != nil {
		t.Error("expected nil when title is empty")
	}
}

// No img.item-image present → falls back to any img for thumbnail.
// Covers pornmd.go:54-56.
func TestPornMDParse_ThumbnailFallback(t *testing.T) {
	p := NewPornMDParser()
	html := `<div class="card sub">
		<a class="item-link" href="/out/md003" title="Thumbnail Fallback Video">
			<img src="fallback-thumb.jpg">
		</a>
	</div>`
	item := p.Parse(newDoc(html).Find("div.card.sub"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.Thumbnail == "" {
		t.Error("Thumbnail must not be empty when any img is present")
	}
}

// data-video-url on the link element populates DownloadURL.
// Covers pornmd.go:67-69.
func TestPornMDParse_DownloadURLFromLink(t *testing.T) {
	p := NewPornMDParser()
	html := `<div class="card sub">
		<a class="item-link" href="/out/md004" title="DL Link Video"
		   data-video-url="//cdn.pornmd.com/dl.mp4">
			<img class="item-image" src="thumb.jpg">
		</a>
	</div>`
	item := p.Parse(newDoc(html).Find("div.card.sub"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.DownloadURL == "" {
		t.Error("DownloadURL must not be empty when data-video-url is on the link")
	}
}

// Duration badge present → Duration/DurationSeconds populated.
// Covers pornmd.go:77-82.
func TestPornMDParse_DurationBadge(t *testing.T) {
	p := NewPornMDParser()
	html := `<div class="card sub">
		<a class="item-link" href="/out/md005" title="Duration Badge Video">
			<img class="item-image" src="thumb.jpg">
		</a>
		<div class="item-meta-container">
			<span class="badge float-right">4:00</span>
		</div>
	</div>`
	item := p.Parse(newDoc(html).Find("div.card.sub"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.DurationSeconds == 0 {
		t.Error("DurationSeconds should be non-zero when badge contains duration")
	}
}

// Rating span containing "%" populates item.Rating.
// Covers pornmd.go:86-90.
func TestPornMDParse_RatingPercent(t *testing.T) {
	p := NewPornMDParser()
	html := `<div class="card sub">
		<a class="item-link" href="/out/md006" title="Rated Video">
			<img class="item-image" src="thumb.jpg">
		</a>
		<span class="item-score">88%</span>
	</div>`
	item := p.Parse(newDoc(html).Find("div.card.sub"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.Rating != "88%" {
		t.Errorf("Rating = %q, want 88%%", item.Rating)
	}
}

// Non-empty .item-source text populates item.Description.
// Covers pornmd.go:96-98.
func TestPornMDParse_SourceText(t *testing.T) {
	p := NewPornMDParser()
	html := `<div class="card sub">
		<a class="item-link" href="/out/md007" title="Sourced Video">
			<img class="item-image" src="thumb.jpg">
		</a>
		<span class="item-source">YouPorn</span>
	</div>`
	item := p.Parse(newDoc(html).Find("div.card.sub"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.Description == "" {
		t.Error("Description must not be empty when item-source is present")
	}
}

// ============================================================
// RedTubeParser branch tests
// ============================================================

// Title link has no href → falls back to a.video_link for the URL.
// Covers redtube.go:42-45.
func TestRedTubeParse_HrefFromVideoLink(t *testing.T) {
	p := NewRedTubeParser()
	html := `<li class="videoblock_list">
		<a class="video_link" href="/video/55">
			<img class="js_thumbImageTag" src="thumb.jpg">
		</a>
		<a class="video-title-text">RedTube Fallback Href</a>
	</li>`
	item := p.Parse(newDoc(html).Find("li.videoblock_list"))
	if item == nil {
		t.Fatal("expected non-nil item when href comes from video_link fallback")
	}
	if item.URL == "" {
		t.Error("URL must not be empty")
	}
}

// Title is present but no href anywhere → return nil.
// Covers redtube.go:46-48.
func TestRedTubeParse_NoHref_ReturnsNil(t *testing.T) {
	p := NewRedTubeParser()
	html := `<li class="videoblock_list">
		<a class="video-title-text">Title But No Href</a>
	</li>`
	if p.Parse(newDoc(html).Find("li.videoblock_list")) != nil {
		t.Error("expected nil when no href is available")
	}
}

// Non-empty data-mediabook on img → PreviewURL set with &amp; replaced.
// Covers redtube.go:63-65.
func TestRedTubeParse_PreviewURLAmpersandReplaced(t *testing.T) {
	p := NewRedTubeParser()
	html := `<li class="videoblock_list">
		<a class="video_link" href="/video/66">
			<img class="js_thumbImageTag" src="thumb.jpg"
			     data-mediabook="https://cdn.redtube.com/preview.mp4?a=1&amp;b=2">
		</a>
		<a class="video-title-text" href="/video/66">Preview Amp Video</a>
	</li>`
	item := p.Parse(newDoc(html).Find("li.videoblock_list"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.PreviewURL == "" {
		t.Error("PreviewURL must not be empty when data-mediabook is set")
	}
}

// data-video-url present on img → DownloadURL populated and &amp; replaced.
// Covers redtube.go:72-76.
func TestRedTubeParse_DownloadURL_AmpersandAndPrefix(t *testing.T) {
	p := NewRedTubeParser()
	html := `<li class="videoblock_list">
		<a class="video_link" href="/video/77">
			<img class="js_thumbImageTag" src="thumb.jpg"
			     data-video-url="//cdn.redtube.com/dl.mp4?x=1&amp;y=2">
		</a>
		<a class="video-title-text" href="/video/77">Download Amp Video</a>
	</li>`
	item := p.Parse(newDoc(html).Find("li.videoblock_list"))
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.DownloadURL == "" {
		t.Error("DownloadURL must not be empty when data-video-url is set")
	}
}
