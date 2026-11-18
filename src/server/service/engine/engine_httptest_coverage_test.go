// SPDX-License-Identifier: MIT
// AI.md PART 28: httptest-based coverage tests for all engine Search() methods.
// Each engine's Search() is called against a local httptest.Server so no real
// network connections are made. Tests verify the function body executes without
// panicking; errors from HTML/JSON mismatch are expected and ignored.
package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// universalHTML is served for all paths and matches the CSS selectors used
// by every genericSearch-based engine, plus XVideos, RedTube, TubeGalore,
// TNAFlix, Motherless, and YouPorn-style selectors.
const universalHTML = `<!DOCTYPE html><html><body>
<ul>
  <li class="thumb"><a href="/v/1" title="Alpha Test"><img src="/t.jpg"></a><span class="duration">5:00</span><span class="views">1K</span></li>
  <li class="video-item"><a href="/v/16" title="YouJizz Test"><img src="/t.jpg"></a></li>
  <li class="videoblock_list"><a class="video-title-text" href="/v/20" title="RedTube Test"><img src="/t.jpg"></a></li>
</ul>
<div class="item"><a href="/v/2" title="Item Test"><img src="/t.jpg"></a></div>
<div class="video-item"><a href="/v/3" title="VideoItem Test"><img src="/t.jpg"></a></div>
<div class="card"><a href="/v/4" title="Card Test"><img src="/t.jpg"></a></div>
<div class="card sub"><a href="/v/5" title="CardSub Test"><img src="/t.jpg"></a></div>
<div class="video-thumb"><a href="/v/6" title="VideoThumb Test"><img src="/t.jpg"></a></div>
<div class="box"><a href="/v/7" title="Box Test"><img src="/t.jpg"></a></div>
<div class="scene"><a href="/v/8" title="Scene Test"><img src="/t.jpg"></a></div>
<a class="th video-thumb" href="/v/9" title="Nuvid Test"><img src="/t.jpg"></a>
<div class="video_container"><a href="/v/10" title="Container Test"><img src="/t.jpg"></a></div>
<a class="item drclass" href="/v/11" title="Sun Test"><img src="/t.jpg"></a>
<div class="thumb-item"><a href="/v/12" title="ThumbItem Test"><img src="/t.jpg"></a></div>
<a class="th ch-video" href="/v/13" title="DrTuber Test"><img src="/t.jpg"></a>
<div class="video-box thumbnail-card"><a class="video-box-image" href="/v/14" title="Tube8 Test"><img src="/t.jpg"></a><span class="video-title-text">Tube8 Test</span></div>
<div class="thumb"><a href="/v/15" title="Thumb Test"><img src="/t.jpg"></a></div>
<div data-public-id="testid123"><a href="/v/17" title="TubeGalore Test"><img src="/t.jpg"></a></div>
<div class="col-xs-6 col-md-4"><a href="/v/18" title="TNAFlix Test"><img src="/t.jpg"></a></div>
<div class="thumb-block"><a href="/v/19" title="XVideos Test"><img src="/t.jpg"></a></div>
<div class="thumb-container"><a href="/v/21" title="Motherless Test"><img src="/t.jpg"></a></div>
</body></html>`

// xhamsterHTML contains a window.initials JSON blob matching the real site structure.
const xhamsterHTML = `<!DOCTYPE html><html><head></head><body>
<script>window.initials={"searchResult":{"videoThumbProps":[{"id":1,"title":"XHamster Test","duration":300,"views":12345,"pageURL":"https://xhamster.com/videos/test-1","thumbURL":"https://thumb.xhamster.com/1.jpg","created":1699000000}]}};</script>
</body></html>`

// epornerJSON is a minimal Eporner API JSON response.
const epornerJSON = `{"count":1,"total_count":1,"videos":[{"id":"abc123","title":"Eporner Test","keywords":"test,video","views":1000,"rate":"70%","url":"https://www.eporner.com/video-abc123/test/","added":"2024-01-01","length_sec":300,"length_min":"5:00","default_thumb":{"src":"https://img.eporner.com/1.jpg"}}]}`

// newUniversalServer starts an httptest.Server that returns universalHTML for every request.
func newUniversalServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(universalHTML))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newXHamsterServer starts an httptest.Server that returns XHamster-style JSON-in-HTML.
func newXHamsterServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(xhamsterHTML))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newEpornerServer starts an httptest.Server that returns Eporner JSON API response.
func newEpornerServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(epornerJSON))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func defaultCfg() *config.AppConfig {
	return config.DefaultAppConfig()
}

// ── AlphaPorno ────────────────────────────────────────────────────────────────

func TestAlphaPornoEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newAlphaPornoEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── AnyPorn ───────────────────────────────────────────────────────────────────

func TestAnyPornEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newAnyPornEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── DrTuber ───────────────────────────────────────────────────────────────────

func TestDrTuberEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newDrTuberEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

func TestDrTuberEngine_UsesLiveSearchRoute(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Search</title></head><body><a class="th ch-video" href="/video/1" title="Video"><img src="https://cdn.example/thumb.jpg"></a></body></html>`))
	}))
	t.Cleanup(srv.Close)
	e := newDrTuberEngine(defaultCfg())
	e.baseURL = srv.URL
	results, err := e.Search(context.Background(), "lesbian", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/search/videos/lesbian/2" {
		t.Fatalf("search path = %q, want /search/videos/lesbian/2", gotPath)
	}
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want 1", len(results))
	}
}

// ── Eporner ───────────────────────────────────────────────────────────────────

func TestEpornerEngine_Search_HttpTest(t *testing.T) {
	srv := newEpornerServer(t)
	e := newEpornerEngine(defaultCfg())
	e.baseURL = srv.URL
	results, err := e.Search(context.Background(), "test", 1)
	if err != nil {
		t.Logf("EpornerEngine.Search error (acceptable): %v", err)
		return
	}
	if len(results) == 0 {
		t.Error("EpornerEngine.Search: expected at least one result from JSON API")
	}
}

// ── EmpFlix ───────────────────────────────────────────────────────────────────

func TestEMPFlixEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newEMPFlixEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── FlyFlv ────────────────────────────────────────────────────────────────────

func TestFlyflvEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newFlyflvEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── FourTube ──────────────────────────────────────────────────────────────────

func TestFourTubeEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newFourTubeEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── Fux ───────────────────────────────────────────────────────────────────────

func TestFuxEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newFuxEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── GotPorn ───────────────────────────────────────────────────────────────────

func TestGotPornEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newGotPornEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── HellPorno ─────────────────────────────────────────────────────────────────

func TestHellPornoEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newHellPornoEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── HQPorner ──────────────────────────────────────────────────────────────────

func TestHqpornerEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newHqpornerEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── LoveHomePorn ──────────────────────────────────────────────────────────────

func TestLoveHomePornEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newLoveHomePornEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── Motherless ────────────────────────────────────────────────────────────────

func TestMotherlessEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newMotherlessEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── NonkTube ──────────────────────────────────────────────────────────────────

func TestNonkTubeEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newNonkTubeEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── NubilesPorn ───────────────────────────────────────────────────────────────

func TestNubilesPornEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newNubilesPornEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── Nuvid ─────────────────────────────────────────────────────────────────────

func TestNuvidEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newNuvidEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornBox ───────────────────────────────────────────────────────────────────

func TestPornboxEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornboxEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornerBros ────────────────────────────────────────────────────────────────

func TestPornerBrosEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornerBrosEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornFlip ──────────────────────────────────────────────────────────────────

func TestPornFlipEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornFlipEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornTrex ──────────────────────────────────────────────────────────────────

func TestPornTrexEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornTrexEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornTube ──────────────────────────────────────────────────────────────────

func TestPornTubeEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornTubeEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── RedTube ───────────────────────────────────────────────────────────────────

func TestRedTubeEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newRedTubeEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── SunPorno ──────────────────────────────────────────────────────────────────

func TestSunPornoEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newSunPornoEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── ThreeMovs ─────────────────────────────────────────────────────────────────

func TestThreeMovsEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newThreeMovsEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── TNAFlix ───────────────────────────────────────────────────────────────────

func TestTNAFlixEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newTNAFlixEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── Tube8 ─────────────────────────────────────────────────────────────────────

func TestTube8Engine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newTube8Engine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── TubeGalore ────────────────────────────────────────────────────────────────

func TestTubeGaloreEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newTubeGaloreEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── Txxx ──────────────────────────────────────────────────────────────────────

func TestTxxxEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newTxxxEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── VJav ──────────────────────────────────────────────────────────────────────

func TestVJAVEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newVJAVEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── XBabe ─────────────────────────────────────────────────────────────────────

func TestXBabeEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newXBabeEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── XHamster ──────────────────────────────────────────────────────────────────

func TestXHamsterEngine_Search_HttpTest_Page1(t *testing.T) {
	srv := newXHamsterServer(t)
	e := newXHamsterEngine(defaultCfg())
	e.baseURL = srv.URL
	results, err := e.Search(context.Background(), "test", 1)
	if err != nil {
		t.Logf("XHamsterEngine.Search page 1 error (acceptable): %v", err)
		return
	}
	if len(results) == 0 {
		t.Error("XHamsterEngine.Search: expected at least one result from JSON extraction")
	}
	if results[0].Published.IsZero() {
		t.Error("XHamsterEngine.Search: expected Published to be set from the created timestamp, got zero time")
	}
	wantPublished := time.Unix(1699000000, 0).UTC()
	if !results[0].Published.Equal(wantPublished) {
		t.Errorf("XHamsterEngine.Search: Published = %v, want %v", results[0].Published, wantPublished)
	}
}

func TestXHamsterEngine_Search_HttpTest_Page2(t *testing.T) {
	srv := newXHamsterServer(t)
	e := newXHamsterEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 2)
}

// ── XNXX ──────────────────────────────────────────────────────────────────────

func TestXNXXEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newXNXXEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── XVideos ───────────────────────────────────────────────────────────────────

func TestXVideosEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newXVideosEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── XXXYMovies ────────────────────────────────────────────────────────────────

func TestXXXYMoviesEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newXXXYMoviesEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

func TestXXXYMoviesEngine_UsesCurrentSearchRoute(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("q")
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("search page = %q, want 2", r.URL.Query().Get("page"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(universalHTML))
	}))
	t.Cleanup(srv.Close)
	e := newXXXYMoviesEngine(defaultCfg())
	e.baseURL = srv.URL
	results, err := e.Search(context.Background(), "lesbian", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/search/" || gotQuery != "lesbian" {
		t.Fatalf("search URL = %s?q=%s, want /search/?q=lesbian", gotPath, gotQuery)
	}
	if len(results) == 0 {
		t.Fatal("Search returned no results from the current result-card markup")
	}
}

// ── YouJizz ───────────────────────────────────────────────────────────────────

func TestYouJizzEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newYouJizzEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── YouPorn ───────────────────────────────────────────────────────────────────

func TestYouPornEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newYouPornEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── MakeRequestWithMod ────────────────────────────────────────────────────────

func TestBaseEngine_MakeRequestWithMod_Success(t *testing.T) {
	srv := newUniversalServer(t)
	e := NewBaseEngine("test", "Test", srv.URL, 1, defaultCfg())
	resp, err := e.MakeRequest(context.Background(), srv.URL+"/test")
	if err != nil {
		t.Fatalf("MakeRequest: unexpected error: %v", err)
	}
	_ = resp.Body.Close()
}

func TestBaseEngine_MakeRequestWithMod_WithModifier(t *testing.T) {
	srv := newUniversalServer(t)
	e := NewBaseEngine("test", "Test", srv.URL, 1, defaultCfg())
	resp, err := e.MakeRequestWithMod(context.Background(), srv.URL+"/test", func(req *http.Request) {
		req.Header.Set("X-Test", "coverage")
	})
	if err != nil {
		t.Fatalf("MakeRequestWithMod: unexpected error: %v", err)
	}
	_ = resp.Body.Close()
}

func TestBaseEngine_MakeRequest_ServerError_RecordsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	e := NewBaseEngine("test", "Test", srv.URL, 1, defaultCfg())
	_, _ = e.MakeRequest(context.Background(), srv.URL+"/err")
}

func TestBaseEngine_MakeRequest_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	e := NewBaseEngine("test", "Test", srv.URL, 1, defaultCfg())
	_, _ = e.MakeRequest(context.Background(), srv.URL+"/limit")
}

func TestBaseEngine_MakeRequest_ClientErrorsRecordFailure(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusGone} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			t.Cleanup(srv.Close)
			e := NewBaseEngine("test", "Test", srv.URL, 1, defaultCfg())
			e.minRequestInterval = 0
			if resp, err := e.MakeRequest(context.Background(), srv.URL+"/search"); err == nil {
				if resp != nil {
					_ = resp.Body.Close()
				}
				t.Fatalf("MakeRequest returned no error for HTTP %d", status)
			}
			stats := e.GetStats()
			if stats.TotalFailures != 1 || stats.TotalSuccesses != 0 {
				t.Fatalf("health stats = failures %d, successes %d; want 1, 0", stats.TotalFailures, stats.TotalSuccesses)
			}
		})
	}
}

// ── genericSearch coverage ────────────────────────────────────────────────────

func TestGenericSearch_ReturnsResults(t *testing.T) {
	srv := newUniversalServer(t)
	e := NewBaseEngine("test", "Test", srv.URL, 1, defaultCfg())
	results, err := genericSearch(context.Background(), e, srv.URL+"/search", "li.thumb")
	if err != nil {
		t.Fatalf("genericSearch: unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Error("genericSearch: expected at least one result matching li.thumb")
	}
}

func TestGenericSearch_NoMatchingSelector_EmptyResults(t *testing.T) {
	srv := newUniversalServer(t)
	e := NewBaseEngine("test", "Test", srv.URL, 1, defaultCfg())
	results, err := genericSearch(context.Background(), e, srv.URL+"/search", "div.nonexistent-selector-xyz")
	if err != nil {
		t.Fatalf("genericSearch no match: unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("genericSearch no match: expected 0 results, got %d", len(results))
	}
}

func TestGenericSearch_RejectsUnrenderedAndCookieBlockedPages(t *testing.T) {
	for name, body := range map[string]string{
		"unrendered":       `...`,
		"cookies required": `<div class="cookie-error-box">Cookies Required</div>`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(srv.Close)
			e := NewBaseEngine("test", "Test", srv.URL, 1, defaultCfg())
			var err error
			if name == "unrendered" {
				_, err = genericSearchWithPageValidation(context.Background(), e, srv.URL, ".video-item", true)
			} else {
				_, err = genericSearch(context.Background(), e, srv.URL, ".video-item")
			}
			if err == nil {
				t.Fatal("genericSearch did not report the invalid source page")
			}
		})
	}
}

// ── PornHub ───────────────────────────────────────────────────────────────────

func TestPornHubEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := NewPornHubEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornHat ───────────────────────────────────────────────────────────────────

func TestPornHatEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornHatEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornHD ────────────────────────────────────────────────────────────────────

func TestPornHDEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornHDEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornMD ────────────────────────────────────────────────────────────────────

func TestPornMDEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornMDEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornOne ───────────────────────────────────────────────────────────────────

func TestPornOneEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornOneEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}

// ── PornTop ───────────────────────────────────────────────────────────────────

func TestPornTopEngine_Search_HttpTest(t *testing.T) {
	srv := newUniversalServer(t)
	e := newPornTopEngine(defaultCfg())
	e.baseURL = srv.URL
	_, _ = e.Search(context.Background(), "test", 1)
}
