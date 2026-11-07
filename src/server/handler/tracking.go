package handler

import (
	"encoding/json"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

func trackingJSString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func trackingConfigured(cfg config.TrackingConfig) bool {
	t := strings.ToLower(strings.TrimSpace(cfg.Type))
	return t != "" && t != "none"
}

func (h *ServerHandler) privacyConfigForDisplay() config.PrivacyConfig {
	privacy := h.appConfig.Server.Privacy
	privacy.Cookies.Analytics.Enabled = trackingConfigured(h.appConfig.Server.Tracking)
	if privacy.Cookies.Analytics.Enabled {
		found := false
		for _, service := range privacy.ThirdParty.Services {
			if strings.EqualFold(service.Name, trackingTypeName(h.appConfig.Server.Tracking.Type)) {
				found = true
				break
			}
		}
		if !found {
			privacy.ThirdParty.Services = append(privacy.ThirdParty.Services, config.ThirdPartyService{
				Name:     trackingTypeName(h.appConfig.Server.Tracking.Type),
				Purpose:  "Usage analytics",
				DataSent: "Page views and browser information, only after consent",
			})
		}
	}
	return privacy
}

func trackingAllowed(cfg config.TrackingConfig, r *http.Request) bool {
	return trackingConfigured(cfg) && consentAnalyticsEnabled(r) && !urlvar.GlobalResolver().IsOverlayRequest(r)
}

func trackingScriptMarkup(cfg config.TrackingConfig, r *http.Request) template.HTML {
	if !trackingAllowed(cfg, r) {
		return ""
	}
	t := strings.ToLower(cfg.Type)
	id := html.EscapeString(cfg.ID)
	base := strings.TrimRight(cfg.URL, "/")
	var tag string
	switch t {
	case "google":
		if strings.HasPrefix(cfg.ID, "G-") {
			tag = `<script async src="https://www.googletagmanager.com/gtag/js?id=` + id + `"></script>`
		} else {
			tag = `<script async src="https://www.google-analytics.com/analytics.js"></script>`
		}
	case "matomo", "piwik", "owa":
		// These providers load their external tracker from the generated same-origin
		// script body so their configuration is queued before the tracker starts.
	case "fathom":
		scriptURL := "https://cdn.usefathom.com/script.js"
		if base != "" {
			scriptURL = base + "/tracker.js"
		}
		tag = `<script defer src="` + html.EscapeString(scriptURL) + `" data-site="` + id + `"></script>`
	case "plausible":
		scriptURL := "https://plausible.io/js/script.js"
		if base != "" {
			scriptURL = base + "/js/script.js"
		}
		tag = `<script defer data-domain="` + id + `" src="` + html.EscapeString(scriptURL) + `"></script>`
	case "umami":
		tag = `<script defer src="` + html.EscapeString(base+"/script.js") + `" data-website-id="` + id + `"></script>`
	case "simple":
		tag = `<script async defer src="https://scripts.simpleanalyticscdn.com/latest.js"></script>`
	case "cloudflare":
		tag = `<script defer src="https://static.cloudflareinsights.com/beacon.min.js" data-cf-beacon='{"token":"` + id + `"}'></script>`
	default:
		return ""
	}
	if tag == "" {
		localURL := html.EscapeString(urlvar.BuildURL(r, "/server/tracking.js"))
		tag = `<script async src="` + localURL + `"></script>`
		return template.HTML(tag)
	}
	localURL := html.EscapeString(urlvar.BuildURL(r, "/server/tracking.js"))
	if t := strings.ToLower(cfg.Type); t == "google" {
		return template.HTML(tag + `<script async src="` + localURL + `"></script>`)
	}
	return template.HTML(tag)
}

func trackingNoScriptMarkup(cfg config.TrackingConfig, r *http.Request) template.HTML {
	if !trackingAllowed(cfg, r) || strings.ToLower(cfg.Type) != "simple" {
		return ""
	}
	return `<noscript><img src="https://queue.simpleanalyticscdn.com/noscript.gif" alt="" referrerpolicy="no-referrer-when-downgrade"></noscript>`
}

func trackingJavaScript(cfg config.TrackingConfig) string {
	id := trackingJSString(cfg.ID)
	base := strings.TrimRight(cfg.URL, "/")
	baseJS := trackingJSString(base)
	switch strings.ToLower(cfg.Type) {
	case "google":
		if strings.HasPrefix(cfg.ID, "G-") {
			return "window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments)};gtag('js',new Date());gtag('config'," + id + ");\n"
		}
		return "window.ga=window.ga||function(){(ga.q=ga.q||[]).push(arguments)};ga.l=+new Date;ga('create'," + id + ",'auto');ga('send','pageview');\n"
	case "matomo", "piwik":
		return "window._paq=window._paq||[];_paq.push(['trackPageView']);_paq.push(['enableLinkTracking']);_paq.push(['setTrackerUrl'," + baseJS + "+'/matomo.php']);_paq.push(['setSiteId'," + id + "]);var g=document.createElement('script');g.async=true;g.src=" + baseJS + "+'/matomo.js';document.head.appendChild(g);\n"
	case "owa":
		return "window.owa_baseUrl=" + baseJS + "+'/';window.owa_cmds=window.owa_cmds||[];owa_cmds.push(['setSiteId'," + id + "]);owa_cmds.push(['trackPageView']);owa_cmds.push(['trackClicks']);var o=document.createElement('script');o.async=true;o.src=owa_baseUrl+'modules/base/js/owa.tracker-combined-min.js';document.head.appendChild(o);\n"
	case "fathom", "plausible", "umami", "simple", "cloudflare":
		return ""
	default:
		return ""
	}
}

func trackingCSPOrigins(cfg config.TrackingConfig) (scripts, connections []string) {
	if !trackingConfigured(cfg) {
		return nil, nil
	}
	add := func(origin string) {
		if origin == "" {
			return
		}
		for _, existing := range scripts {
			if existing == origin {
				return
			}
		}
		scripts = append(scripts, origin)
		connections = append(connections, origin)
	}
	providerOrigin := func(raw string) string {
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return ""
		}
		return parsed.Scheme + "://" + parsed.Host
	}
	switch strings.ToLower(cfg.Type) {
	case "google":
		add("https://www.googletagmanager.com")
		add("https://www.google-analytics.com")
	case "matomo", "piwik", "owa", "umami":
		add(providerOrigin(cfg.URL))
	case "fathom":
		if cfg.URL != "" {
			add(providerOrigin(cfg.URL))
		} else {
			add("https://cdn.usefathom.com")
			add("https://api.usefathom.com")
		}
	case "plausible":
		if cfg.URL != "" {
			add(providerOrigin(cfg.URL))
		} else {
			add("https://plausible.io")
		}
	case "simple":
		add("https://scripts.simpleanalyticscdn.com")
		add("https://queue.simpleanalyticscdn.com")
	case "cloudflare":
		add("https://static.cloudflareinsights.com")
		add("https://cloudflareinsights.com")
	}
	return scripts, connections
}

func TrackingCSPOrigins(cfg config.TrackingConfig) (scripts, connections []string) {
	return trackingCSPOrigins(cfg)
}

func trackingTypeName(t string) string {
	switch strings.ToLower(t) {
	case "google":
		return "Google Analytics"
	case "matomo":
		return "Matomo"
	case "piwik":
		return "Piwik"
	case "owa":
		return "Open Web Analytics"
	case "fathom":
		return "Fathom Analytics"
	case "plausible":
		return "Plausible Analytics"
	case "umami":
		return "Umami"
	case "simple":
		return "Simple Analytics"
	case "cloudflare":
		return "Cloudflare Web Analytics"
	default:
		return ""
	}
}

func (h *ServerHandler) TrackingJavaScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	if !trackingAllowed(h.appConfig.Server.Tracking, r) {
		http.NotFound(w, r)
		return
	}
	body := trackingJavaScript(h.appConfig.Server.Tracking)
	if body == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write([]byte(body))
}
