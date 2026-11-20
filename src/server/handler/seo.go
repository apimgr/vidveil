// SPDX-License-Identifier: MIT
package handler

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

func injectSEOData(r *http.Request, cfg *config.AppConfig, data map[string]interface{}) {
	if cfg == nil || data == nil {
		return
	}
	seo := cfg.Server.SEO
	title := cfg.Server.Branding.Title
	siteName := seo.OpenGraph.SiteName
	if siteName == "" {
		siteName = title
	}
	canonicalURL := ""
	if r != nil {
		canonicalURL = urlvar.BuildURL(r, r.URL.Path)
		if seo.Canonical != "" {
			base, err := url.Parse(seo.Canonical)
			if err == nil {
				base.Path = path.Join(strings.TrimSuffix(base.Path, "/"), r.URL.Path)
				if strings.HasSuffix(r.URL.Path, "/") && !strings.HasSuffix(base.Path, "/") {
					base.Path += "/"
				}
				canonicalURL = base.String()
			}
		}
	}
	robots := robotsDirective(r)
	if seo.NoIndex {
		robots = "noindex,nofollow"
	}
	data["Robots"] = robots
	data["CanonicalURL"] = canonicalURL
	data["TitleSeparator"] = seo.TitleSeparator
	data["SEOOpenGraphEnabled"] = seo.OpenGraph.Enabled
	data["SEOOpenGraphType"] = seo.OpenGraph.Type
	data["SEOOpenGraphSiteName"] = siteName
	data["SEOTwitterEnabled"] = seo.Twitter.Enabled
	data["SEOTwitterCard"] = seo.Twitter.Card
	data["SEOTwitterCreator"] = seo.Twitter.Creator
	data["SEOMetaTags"] = seo.MetaTags
}
