package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/apimgr/vidveil/src/server/model"
)

func TestRequestEngineNamesUsesSavedSelection(t *testing.T) {
	engines := []model.EngineInfo{{Name: "alpha"}, {Name: "beta"}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: "vidveil_pref_engine_alpha", Value: "1"})

	if got := requestEngineNames(req, nil, engines); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("requestEngineNames() = %v, want [alpha]", got)
	}
}

func TestRequestEngineSelectionUsesPackedCookie(t *testing.T) {
	engines := []model.EngineInfo{{Name: "alpha"}, {Name: "beta"}, {Name: "gamma"}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: engineSelectionValueCookieName, Value: "alpha.gamma"})

	selected, configured := requestEngineSelection(req, engines)
	if !configured || !selected["alpha"] || selected["beta"] || !selected["gamma"] {
		t.Fatalf("packed engine selection = (%v, %v), want alpha and gamma", selected, configured)
	}
}

func TestRequestEngineSelectionPackedNone(t *testing.T) {
	engines := []model.EngineInfo{{Name: "alpha"}, {Name: "beta"}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: engineSelectionValueCookieName, Value: "none"})

	selected, configured := requestEngineSelection(req, engines)
	if !configured || len(selected) != 0 {
		t.Fatalf("packed empty selection = (%v, %v), want empty configured selection", selected, configured)
	}
}

func TestRequestEngineSelectionLegacyEmptyFallsBackToDefault(t *testing.T) {
	engines := []model.EngineInfo{{Name: "alpha"}, {Name: "beta"}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: "vidveil_pref_engine_alpha", Value: "0"})
	req.AddCookie(&http.Cookie{Name: "vidveil_pref_engine_beta", Value: "0"})

	if selected, configured := requestEngineSelection(req, engines); configured || selected != nil {
		t.Fatalf("legacy empty selection = (%v, %v), want default engine selection", selected, configured)
	}
}

func TestRequestEngineSelectionEmptyPackedValueDefaultsToAll(t *testing.T) {
	engines := []model.EngineInfo{{Name: "alpha"}, {Name: "beta"}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: engineSelectionValueCookieName, Value: ""})

	if selected, configured := requestEngineSelection(req, engines); configured || selected != nil {
		t.Fatalf("empty packed selection = (%v, %v), want default selection", selected, configured)
	}
}

func TestRequestEngineNamesDefaultsToAll(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	if got := requestEngineNames(req, nil, []model.EngineInfo{{Name: "alpha"}}); got != nil {
		t.Fatalf("requestEngineNames() = %v, want nil (all engines)", got)
	}
}

func TestRequestEngineNamesSavedEmptySelectionDoesNotSelectAll(t *testing.T) {
	engines := []model.EngineInfo{{Name: "alpha"}, {Name: "beta"}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: engineSelectionValueCookieName, Value: "none"})

	if got := requestEngineNames(req, nil, engines); !reflect.DeepEqual(got, []string{"__none__"}) {
		t.Fatalf("requestEngineNames() = %v, want [__none__]", got)
	}
}

func TestRequestEngineNamesQueryOverridesSavedSelection(t *testing.T) {
	engines := []model.EngineInfo{{Name: "alpha"}, {Name: "beta"}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=test&engines=beta", nil)
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: "vidveil_pref_engine_alpha", Value: "1"})

	if got := requestEngineNames(req, nil, engines); !reflect.DeepEqual(got, []string{"beta"}) {
		t.Fatalf("requestEngineNames() = %v, want [beta]", got)
	}
}

func TestRequestEngineNamesBangOverridesSavedSelection(t *testing.T) {
	engines := []model.EngineInfo{{Name: "alpha"}, {Name: "beta"}}
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	req.AddCookie(&http.Cookie{Name: engineSelectionCookieName, Value: "1"})
	req.AddCookie(&http.Cookie{Name: "vidveil_pref_engine_alpha", Value: "1"})

	if got := requestEngineNames(req, []string{"beta"}, engines); !reflect.DeepEqual(got, []string{"beta"}) {
		t.Fatalf("requestEngineNames() = %v, want [beta]", got)
	}
}

func TestSearchPaginationURLPreservesSearchAndFiltersWithoutSession(t *testing.T) {
	req := httptest.NewRequest("GET", "/search?q=%21ph+cat&duration=short&sort=views&page=1&session=old", nil)
	got := searchPaginationURL(req, 2)
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("searchPaginationURL() returned invalid URL %q: %v", got, err)
	}
	query := parsed.Query()
	if query.Get("q") != "!ph cat" || query.Get("duration") != "short" || query.Get("sort") != "views" {
		t.Fatalf("searchPaginationURL() lost search filters: %s", got)
	}
	if query.Get("page") != "2" || query.Has("session") {
		t.Fatalf("searchPaginationURL() page/session = %q/%q, want 2/empty", query.Get("page"), query.Get("session"))
	}
}
