// SPDX-License-Identifier: MIT
// AI.md PART 32: native GUI command screens.
//go:build gui && !freebsd && !netbsd && !openbsd

package gui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/client/api"
	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/core/dialog"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/theme/material3"
	"github.com/gogpu/ui/widget"
)

var guiCommandScreens = []string{"search", "engines", "bangs", "probe", "health", "favorites", "login", "update", "setup"}

type guiSession struct {
	cfg    *Config
	app    *app.App
	theme  *material3.Theme
	api    *api.APIClient
	screen string
	values map[string]string
	output []string
	error  string
}

func (session *guiSession) translate(key string) string {
	return i18n.Translate(session.cfg.Language, "gui."+key)
}

func (session *guiSession) buildMainWindow() *primitives.BoxWidget {
	version := session.cfg.Version
	if version == "" {
		version = "dev"
	}
	server := session.cfg.ServerURL
	if server == "" {
		server = session.translate("server_unconfigured")
	}
	children := []widget.Widget{
		primitives.Text(session.cfg.BinaryName).FontSize(24).Bold(),
		primitives.Text(session.translate("version") + " " + version).FontSize(14),
		primitives.Text(session.translate("server") + ": " + server).FontSize(14),
		primitives.Text(session.translate("keyboard_help")).FontSize(13),
	}
	for _, command := range guiCommandScreens {
		name := command
		children = append(children, session.newButton(session.translate("screen."+name), func() { session.openScreen(name) }))
	}
	return primitives.Box(children...).Padding(24).Gap(12)
}

func (session *guiSession) openScreen(name string) {
	session.screen = name
	session.values = make(map[string]string)
	session.output = nil
	session.error = ""
	session.app.SetRoot(session.buildCommandScreen())
}

func (session *guiSession) buildCommandScreen() *primitives.BoxWidget {
	children := []widget.Widget{
		primitives.Text(session.translate("screen." + session.screen)).FontSize(24).Bold(),
		session.newButton(session.translate("main_menu"), func() {
			session.screen = ""
			session.app.SetRoot(session.buildMainWindow())
		}),
	}
	fields := make(map[string]*textfield.Widget)
	addField := func(key, label, placeholder, initial string, inputType textfield.InputType) {
		children = append(children, primitives.Text(label).FontSize(13))
		if value, ok := session.values[key]; ok {
			initial = value
		}
		field := textfield.New(
			textfield.InitialValue(initial),
			textfield.Placeholder(placeholder),
			textfield.InputTypeOpt(inputType),
			textfield.A11yLabel(label),
			textfield.MaxLength(2048),
			textfield.PainterOpt(material3.TextFieldPainter{Theme: session.theme}),
			textfield.OnChange(func(value string) { session.values[key] = value }),
		)
		fields[key] = field
		children = append(children, field)
	}
	addAction := func(label string, action func()) {
		children = append(children, session.newButton(label, action))
	}
	switch session.screen {
	case "setup":
		children = append(children, primitives.Text(session.translate("welcome")).FontSize(16))
		addField("server", session.translate("server_url"), "https://", session.cfg.ServerURL, textfield.TypeText)
		addField("token", session.translate("api_token"), session.translate("api_token_placeholder"), session.cfg.Token, textfield.TypePassword)
		saveConfig := session.values["save_config"] != "false"
		saveLabel := "[ ] " + session.translate("save_configuration")
		if saveConfig {
			saveLabel = "[x] " + session.translate("save_configuration")
		}
		addAction(saveLabel, func() {
			session.values["save_config"] = strconv.FormatBool(!saveConfig)
			session.app.SetRoot(session.buildCommandScreen())
		})
		setDefault := session.values["set_default"] == "true"
		defaultLabel := "[ ] " + session.translate("set_default_server")
		if setDefault {
			defaultLabel = "[x] " + session.translate("set_default_server")
		}
		addAction(defaultLabel, func() {
			session.values["set_default"] = strconv.FormatBool(!setDefault)
			session.app.SetRoot(session.buildCommandScreen())
		})
		if session.cfg.AutostartEnabled != nil && session.cfg.SetAutostart != nil {
			autostart := session.cfg.AutostartEnabled()
			autostartLabel := "[ ] " + session.translate("start_automatically")
			if autostart {
				autostartLabel = "[x] " + session.translate("start_automatically")
			}
			addAction(autostartLabel, func() {
				if err := session.cfg.SetAutostart(!autostart); err != nil {
					session.showError(err.Error())
					return
				}
				session.app.SetRoot(session.buildCommandScreen())
			})
		}
		addAction(session.translate("test_connection"), func() {
			_, serverVersion, err := session.validateServerConnection(fields["server"].Text(), fields["token"].Text())
			if err != nil {
				session.showError(err.Error())
				return
			}
			session.values["server"] = strings.TrimSpace(fields["server"].Text())
			session.values["token"] = fields["token"].Text()
			session.showResults([]string{i18n.TranslateFormat(session.cfg.Language, "gui.connection_success", "version", serverVersion.Version)})
		})
		addAction(session.translate("connect"), func() {
			serverURL := strings.TrimSpace(fields["server"].Text())
			token := fields["token"].Text()
			apiClient, _, err := session.validateServerConnection(serverURL, token)
			if err != nil {
				session.showError(err.Error())
				return
			}
			saveConfig := session.values["save_config"] != "false"
			setDefault := session.values["set_default"] == "true"
			if saveConfig {
				if session.cfg.SaveSetup != nil {
					if err := session.cfg.SaveSetup(serverURL, token, true, setDefault); err != nil {
						session.showError(err.Error())
						return
					}
				} else if session.cfg.SaveServer != nil {
					if err := session.cfg.SaveServer(serverURL, token); err != nil {
						session.showError(err.Error())
						return
					}
				} else {
					session.showError(session.translate("error.setup_storage_unavailable"))
					return
				}
			}
			session.cfg.ServerURL = serverURL
			session.cfg.Token = token
			session.cfg.FirstRun = false
			session.api = apiClient
			session.screen = ""
			session.values = nil
			session.output = nil
			session.error = ""
			session.app.SetRoot(session.buildMainWindow())
		})
	case "search":
		addField("query", session.translate("search_query"), session.translate("search_query_placeholder"), "", textfield.TypeSearch)
		addField("page", session.translate("page"), "1", "1", textfield.TypeNumber)
		addField("limit", session.translate("results_per_page"), "20", "20", textfield.TypeNumber)
		addField("engines", session.translate("engines_comma_separated"), session.translate("all_engines"), "", textfield.TypeText)
		addAction(session.translate("search"), func() {
			query := strings.TrimSpace(fields["query"].Text())
			if query == "" {
				session.showError(session.translate("error.search_query_required"))
				return
			}
			var engines []string
			for _, name := range strings.Split(fields["engines"].Text(), ",") {
				if name = strings.TrimSpace(name); name != "" {
					engines = append(engines, name)
				}
			}
			response, err := session.api.Search(query, guiPositiveInt(fields["page"].Text(), 1), guiPositiveInt(fields["limit"].Text(), 20), engines)
			if err != nil {
				session.showError(err.Error())
				return
			}
			rows := []string{i18n.TranslateFormat(session.cfg.Language, "gui.search_summary", "count", response.Count, "milliseconds", response.SearchTimeMS)}
			for _, result := range response.Results {
				rows = append(rows, i18n.TranslateFormat(session.cfg.Language, "gui.search_result", "title", result.Title, "engine", result.Engine, "url", result.URL))
			}
			session.showResults(rows)
		})
	case "engines":
		addAction(session.translate("all_engines"), func() { session.loadEngineRows("all") })
		addAction(session.translate("enabled_engines"), func() { session.loadEngineRows("enabled") })
		addAction(session.translate("disabled_engines"), func() { session.loadEngineRows("disabled") })
		addAction(session.translate("refresh"), func() { session.loadEngineRows("all") })
	case "health":
		addAction(session.translate("check_health"), func() {
			if session.api == nil {
				session.showError(session.translate("error.server_unconfigured"))
				return
			}
			healthy, err := session.api.Health()
			if err != nil {
				session.showError(err.Error())
				return
			}
			if !healthy {
				session.showError(session.translate("error.health_failed"))
				return
			}
			session.showResults([]string{session.translate("health_healthy")})
		})
	case "bangs":
		addField("filter", session.translate("filter_shortcuts"), session.translate("filter_shortcuts_placeholder"), "", textfield.TypeSearch)
		addAction(session.translate("load_shortcuts"), func() { session.loadBangRows(fields["filter"].Text()) })
	case "probe":
		addField("engines", session.translate("engines"), session.translate("engines_placeholder"), "", textfield.TypeText)
		addField("query", session.translate("test_query"), "test", "test", textfield.TypeSearch)
		verbose := session.values["verbose"] == "true"
		verboseText := session.translate("verbose_off")
		if verbose {
			verboseText = session.translate("verbose_on")
		}
		addAction(verboseText, func() {
			session.values["verbose"] = strconv.FormatBool(!verbose)
			session.app.SetRoot(session.buildCommandScreen())
		})
		addAction(session.translate("probe_engines"), func() { session.probe(fields["engines"].Text(), fields["query"].Text(), verbose) })
	case "login":
		serverURL := session.cfg.ServerURL
		addField("server", session.translate("server_url"), "https://server.example", serverURL, textfield.TypeText)
		addField("token", session.translate("api_token"), session.translate("api_token_placeholder"), "", textfield.TypePassword)
		addAction(session.translate("save_login"), func() {
			serverURL := strings.TrimSpace(fields["server"].Text())
			token := strings.TrimSpace(fields["token"].Text())
			parsed, err := url.ParseRequestURI(serverURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				session.showError(session.translate("error.server_url_invalid"))
				return
			}
			if token == "" {
				session.showError(session.translate("error.api_token_required"))
				return
			}
			if session.cfg.SaveLogin == nil {
				session.showError(session.translate("error.login_storage_unavailable"))
				return
			}
			if err := session.cfg.SaveLogin(serverURL, token); err != nil {
				session.showError(err.Error())
				return
			}
			session.cfg.ServerURL = serverURL
			session.cfg.Token = token
			session.api = api.NewAPIClient(serverURL, token, session.cfg.Timeout, session.cfg.APIVersion)
			session.api.SetLanguage(session.cfg.Language)
			session.showResults([]string{session.translate("login_saved")})
		})
	case "favorites":
		addField("url", session.translate("favorite_url"), "https://", "", textfield.TypeText)
		addField("title", session.translate("favorite_title"), "", "", textfield.TypeText)
		addField("thumbnail", session.translate("favorite_thumbnail"), "https://", "", textfield.TypeText)
		addField("source", session.translate("favorite_source"), "", "", textfield.TypeText)
		addField("id", session.translate("favorite_id"), "1", "1", textfield.TypeNumber)
		addAction(session.translate("favorites_list"), func() {
			favorites, err := session.api.ListFavorites()
			if err != nil {
				session.showError(err.Error())
				return
			}
			rows := make([]string, 0, len(favorites))
			for _, favorite := range favorites {
				rows = append(rows, i18n.TranslateFormat(session.cfg.Language, "gui.favorite_row", "id", favorite.ID, "title", favorite.Title, "url", favorite.URL))
			}
			session.showResults(rows)
		})
		addAction(session.translate("favorites_add"), func() {
			if err := session.api.AddFavorite(api.Favorite{URL: strings.TrimSpace(fields["url"].Text()), Title: strings.TrimSpace(fields["title"].Text()), Thumbnail: strings.TrimSpace(fields["thumbnail"].Text()), Source: strings.TrimSpace(fields["source"].Text())}); err != nil {
				session.showError(err.Error())
				return
			}
			session.showResults([]string{session.translate("favorite_added")})
		})
		addAction(session.translate("favorites_remove"), func() {
			id, err := strconv.ParseInt(strings.TrimSpace(fields["id"].Text()), 10, 64)
			if err != nil || id <= 0 {
				session.showError(session.translate("error.favorite_id_invalid"))
				return
			}
			if err := session.api.RemoveFavorite(id); err != nil {
				session.showError(err.Error())
				return
			}
			session.showResults([]string{session.translate("favorite_removed")})
		})
		addAction(session.translate("favorites_clear"), func() {
			session.confirmClearFavorites()
		})
	case "update":
		addAction(session.translate("check_update"), func() {
			if session.cfg.CheckUpdate == nil {
				session.showError(session.translate("error.update_unavailable"))
				return
			}
			status, err := session.cfg.CheckUpdate()
			if err != nil {
				session.showError(err.Error())
				return
			}
			session.showResults([]string{status})
		})
		addAction(session.translate("apply_update"), func() {
			if session.cfg.ApplyUpdate == nil {
				session.showError(session.translate("error.update_unavailable"))
				return
			}
			if err := session.cfg.ApplyUpdate(); err != nil {
				session.showError(err.Error())
				return
			}
			session.showResults([]string{session.translate("update_started")})
		})
	default:
		session.error = session.translate("error.unknown_screen")
	}
	if session.error != "" {
		children = append(children, primitives.Text(session.error).FontSize(14))
	}
	for _, line := range session.output {
		children = append(children, primitives.Text(line).FontSize(13))
	}
	return primitives.Box(children...).Padding(24).Gap(10)
}

func (session *guiSession) confirmClearFavorites() {
	confirmation := dialog.New(
		dialog.Title(session.translate("favorites_clear")),
		dialog.Content(primitives.Text(session.translate("favorites_clear_confirm"))),
		dialog.Actions(
			dialog.Action{Label: session.translate("cancel"), Variant: dialog.VariantTextOnly},
			dialog.Action{Label: session.translate("favorites_clear"), Default: true, OnClick: func() {
				if err := session.api.ClearFavorites(); err != nil {
					session.showError(err.Error())
					return
				}
				session.showResults([]string{session.translate("favorites_cleared")})
			}},
		),
		dialog.PainterOpt(material3.DialogPainter{Theme: session.theme}),
	)
	confirmation.Show(session.app.Window().Context())
}

func (session *guiSession) newButton(label string, action func()) *button.Widget {
	return button.New(
		button.Text(label),
		button.A11yHint(label),
		button.PainterOpt(material3.ButtonPainter{Theme: session.theme}),
		button.OnClick(action),
	)
}

func (session *guiSession) loadEngineRows(filter string) {
	body, err := session.api.FetchURLResponseBytes(session.api.GetAPIBaseURL() + "/engines")
	if err != nil {
		session.showError(err.Error())
		return
	}
	var response struct {
		OK   bool `json:"ok"`
		Data []struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Enabled     bool   `json:"enabled"`
			Available   bool   `json:"available"`
			Tier        int    `json:"tier"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		session.showError(err.Error())
		return
	}
	if !response.OK {
		session.showError(response.Error)
		return
	}
	rows := make([]string, 0, len(response.Data))
	for _, engine := range response.Data {
		if filter == "enabled" && !engine.Enabled || filter == "disabled" && engine.Enabled {
			continue
		}
		rows = append(rows, i18n.TranslateFormat(session.cfg.Language, "gui.engine_summary", "display_name", engine.DisplayName, "name", engine.Name, "enabled", engine.Enabled, "available", engine.Available, "tier", engine.Tier))
	}
	session.showResults(rows)
}

func (session *guiSession) loadBangRows(filter string) {
	body, err := session.api.FetchURLResponseBytes(session.api.GetAPIBaseURL() + "/bangs")
	if err != nil {
		session.showError(err.Error())
		return
	}
	var response struct {
		OK   bool `json:"ok"`
		Data []struct {
			Bang        string `json:"bang"`
			EngineName  string `json:"engine_name"`
			DisplayName string `json:"display_name"`
			ShortCode   string `json:"short_code"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		session.showError(err.Error())
		return
	}
	if !response.OK {
		session.showError(response.Error)
		return
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	rows := make([]string, 0, len(response.Data))
	for _, bang := range response.Data {
		line := fmt.Sprintf("%s — %s — %s (%s)", bang.Bang, bang.DisplayName, bang.EngineName, bang.ShortCode)
		if filter == "" || strings.Contains(strings.ToLower(line), filter) {
			rows = append(rows, line)
		}
	}
	session.showResults(rows)
}

func (session *guiSession) probe(engineText, query string, verbose bool) {
	engineText = strings.TrimSpace(engineText)
	query = strings.TrimSpace(query)
	if query == "" {
		session.showError(session.translate("error.test_query_required"))
		return
	}
	var names []string
	if engineText == "*" {
		body, err := session.api.FetchURLResponseBytes(session.api.GetAPIBaseURL() + "/engines")
		if err != nil {
			session.showError(err.Error())
			return
		}
		var response struct {
			Data []struct {
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			session.showError(err.Error())
			return
		}
		for _, engine := range response.Data {
			names = append(names, engine.Name)
		}
	} else {
		for _, name := range strings.Split(engineText, ",") {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		session.showError(session.translate("error.engine_ids_required"))
		return
	}
	rows := make([]string, 0, len(names))
	for _, name := range names {
		started := time.Now()
		response, err := session.api.Search(query, 1, 20, []string{name})
		if err != nil {
			rows = append(rows, i18n.TranslateFormat(session.cfg.Language, "gui.probe_failed", "name", name, "error", err.Error()))
			continue
		}
		rows = append(rows, i18n.TranslateFormat(session.cfg.Language, "gui.probe_summary", "name", name, "count", response.Count, "milliseconds", time.Since(started).Milliseconds()))
		if verbose {
			stats := api.CountSearchResultFields(response.Results)
			rows = append(rows, i18n.TranslateFormat(session.cfg.Language, "gui.probe_fields",
				"thumbnail", stats[api.SearchFieldStatHasThumbnail], "preview", stats[api.SearchFieldStatHasPreviewURL],
				"download", stats[api.SearchFieldStatHasDownloadURL], "duration", stats[api.SearchFieldStatHasDuration],
				"views", stats[api.SearchFieldStatHasViews]))
			if body, infoErr := session.api.FetchURLResponseBytes(session.api.GetAPIBaseURL() + "/engines/" + url.PathEscape(name)); infoErr == nil {
				var details struct {
					Data struct {
						Capabilities map[string]interface{} `json:"capabilities"`
					} `json:"data"`
				}
				if json.Unmarshal(body, &details) == nil {
					rows = append(rows, session.translate("capabilities")+": "+fmt.Sprint(details.Data.Capabilities))
				}
			}
		}
	}
	session.showResults(rows)
}

func (session *guiSession) showResults(rows []string) {
	session.output = rows
	session.error = ""
	session.app.SetRoot(session.buildCommandScreen())
}

func (session *guiSession) validateServerConnection(serverURL, token string) (*api.APIClient, *api.VersionResponse, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(serverURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, nil, fmt.Errorf("%s", session.translate("error.server_url_invalid"))
	}
	client := api.NewAPIClient(serverURL, token, session.cfg.Timeout, session.cfg.APIVersion)
	client.SetLanguage(session.cfg.Language)
	healthy, err := client.Health()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", session.translate("error.connection_failed"), err)
	}
	if !healthy {
		return nil, nil, fmt.Errorf("%s", session.translate("error.server_not_healthy"))
	}
	serverVersion, err := client.GetVersion()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", session.translate("error.server_info_failed"), err)
	}
	if strings.TrimSpace(token) != "" {
		if _, err := client.CheckAuth(); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", session.translate("error.api_token_invalid"), err)
		}
	}
	return client, serverVersion, nil
}

func (session *guiSession) showError(message string) {
	session.error = message
	session.output = nil
	session.app.SetRoot(session.buildCommandScreen())
	alert := dialog.New(
		dialog.Title(session.translate("error.title")),
		dialog.Content(primitives.Text(message)),
		dialog.Actions(dialog.Action{Label: session.translate("ok"), Default: true}),
		dialog.PainterOpt(material3.DialogPainter{Theme: session.theme}),
	)
	alert.Show(session.app.Window().Context())
}

func guiPositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
