// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI Client - TUI Command
package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/apimgr/vidveil/src/client/api"
	"github.com/apimgr/vidveil/src/client/browser"
	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/common/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TUI display constants
// Per AI.md PART 1: No magic numbers - use named constants
const (
	TUIDefaultMaxResults  = 10
	TUIReservedSpaceForUI = 10
	TUIMinDisplayResults  = 3
	TUITruncationPadding  = 10
	TUIMinTruncationWidth = 30
	TUIDefaultTheme       = "dark"
	TUICommandSearch      = "search"
	TUICommandHealth      = "health"
	TUICommandEngines     = "engines"
	TUICommandBangs       = "bangs"
	TUICommandFavorites   = "favorites"
	TUICommandProbe       = "probe"
	TUICommandLogin       = "login"
	TUICommandUpdate      = "update"
	TUICommandSetup       = "setup"
)

var tuiCommandOrder = []string{TUICommandSearch, TUICommandEngines, TUICommandBangs, TUICommandProbe, TUICommandHealth, TUICommandFavorites, TUICommandLogin, TUICommandUpdate, TUICommandSetup}

func tuiTranslate(locale, key string) string {
	return i18n.Translate(locale, "tui."+key)
}

// TUIStyles holds lipgloss styles derived from theme.ColorPalette
// Per AI.md PART 32: TUI Styles from Palette
// Per AI.md PART 1: Type names MUST be specific
type TUIStyles struct {
	Base     lipgloss.Style
	Title    lipgloss.Style
	Input    lipgloss.Style
	Result   lipgloss.Style
	Selected lipgloss.Style
	Help     lipgloss.Style
	Status   lipgloss.Style
	Error    lipgloss.Style
	Warning  lipgloss.Style
	Muted    lipgloss.Style
	Border   lipgloss.Style
}

// TUILayoutConfig provides TUI-specific layout settings based on SizeMode
// Per AI.md PART 32: Responsive Layout
// Per AI.md PART 1: Type names MUST be specific
type TUILayoutConfig struct {
	ShowBorders    bool
	ShowHeader     bool
	ShowFooter     bool
	ShowSidebar    bool
	SidebarWidth   int
	MaxColumns     int
	TruncateAt     int
	UseAbbrev      bool
	VerticalScroll bool
}

// createTUIStylesFromPalette creates TUIStyles from theme.ColorPalette
// Per AI.md PART 32: TUI Styles from Palette
// Per AI.md PART 1: Function names MUST reveal intent
func createTUIStylesFromPalette(palette theme.ColorPalette) TUIStyles {
	return TUIStyles{
		Base: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Foreground)).
			Background(lipgloss.Color(palette.Background)),
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Primary)).
			Bold(true).
			Padding(0, 1),
		Input: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Foreground)).
			Background(lipgloss.Color(palette.Surface)).
			Padding(0, 1),
		Result: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Foreground)),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Info)).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Muted)),
		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Success)),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Error)),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Warning)),
		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Muted)),
		Border: lipgloss.NewStyle().
			BorderForeground(lipgloss.Color(palette.Border)),
	}
}

// createTUIStylesFromTerminalPalette creates TUIStyles from the ANSI-mapped
// theme.TerminalPalette — see AI.md PART 16 "CLI/TUI Color Mapping". This is
// the REQUIRED baseline for CLI/TUI output; createTUIStylesFromPalette
// (literal hex) is an opt-in enhancement only, used when the terminal
// reports true-color support.
func createTUIStylesFromTerminalPalette(palette theme.TerminalPalette) TUIStyles {
	return TUIStyles{
		Base: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Foreground)),
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Primary)).
			Bold(true).
			Padding(0, 1),
		Input: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Foreground)).
			Padding(0, 1),
		Result: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Foreground)),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Info)).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Muted)),
		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Success)),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Error)),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Warning)),
		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Muted)),
		Border: lipgloss.NewStyle().
			BorderForeground(lipgloss.Color(palette.Border)),
	}
}

// getTUILayoutConfig returns layout config for a terminal.SizeMode
// Per AI.md PART 32: Responsive Layout
// Per AI.md PART 1: Function names MUST reveal intent
func getTUILayoutConfig(sizeMode terminal.SizeMode) TUILayoutConfig {
	configs := map[terminal.SizeMode]TUILayoutConfig{
		terminal.SizeModeMicro: {
			ShowBorders:    false,
			ShowHeader:     false,
			ShowFooter:     false,
			ShowSidebar:    false,
			MaxColumns:     2,
			TruncateAt:     30,
			UseAbbrev:      true,
			VerticalScroll: true,
		},
		terminal.SizeModeMinimal: {
			ShowBorders:    false,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    false,
			MaxColumns:     3,
			TruncateAt:     40,
			UseAbbrev:      true,
			VerticalScroll: true,
		},
		terminal.SizeModeCompact: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    false,
			MaxColumns:     4,
			TruncateAt:     60,
			UseAbbrev:      false,
			VerticalScroll: true,
		},
		terminal.SizeModeStandard: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    false,
			MaxColumns:     6,
			TruncateAt:     80,
			UseAbbrev:      false,
			VerticalScroll: true,
		},
		terminal.SizeModeWide: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    true,
			SidebarWidth:   30,
			MaxColumns:     8,
			TruncateAt:     120,
			UseAbbrev:      false,
			VerticalScroll: true,
		},
		terminal.SizeModeUltrawide: {
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    true,
			SidebarWidth:   40,
			MaxColumns:     12,
			TruncateAt:     200,
			UseAbbrev:      false,
			VerticalScroll: false,
		},
		terminal.SizeModeMassive: {
			ShowBorders:  true,
			ShowHeader:   true,
			ShowFooter:   true,
			ShowSidebar:  true,
			SidebarWidth: 50,
			MaxColumns:   20,
			// No truncation
			TruncateAt:     0,
			UseAbbrev:      false,
			VerticalScroll: false,
		},
	}
	if config, ok := configs[sizeMode]; ok {
		return config
	}
	return configs[terminal.SizeModeStandard]
}

// Global TUI styles - initialized on startup
// Per AI.md PART 1: Variable names must be specific
var tuiStyles TUIStyles

// TUIModel represents the TUI application state
// Per AI.md PART 1: Type names MUST be specific - "model" is ambiguous
type TUIModel struct {
	language          string
	activeView        string
	menuIndex         int
	viewRows          []string
	viewScroll        int
	loginServerURL    string
	loginToken        string
	loginField        int
	probeEngines      string
	probeQuery        string
	probeVerbose      bool
	engineFilter      string
	bangFilter        string
	viewLoading       bool
	showHelp          bool
	searchQuery       string
	searchField       int
	searchScroll      int
	searchPage        string
	searchLimit       string
	searchEngines     string
	searchResults     []TUISearchResult
	selectedIndex     int
	lastError         error
	isLoading         bool
	terminalWidth     int
	terminalHeight    int
	isQuitting        bool
	sizeMode          terminal.SizeMode
	layoutConfig      TUILayoutConfig
	statusMessage     string
	openedURL         string
	showShortcutsHelp bool
	setupRequested    bool
	utilityMode       string
	utilityField      int
	utilityValues     [4]string
}

// TUISearchResult represents a single search result in TUI
// Per AI.md PART 1: Type names MUST be specific - "searchResult" is ambiguous
type TUISearchResult struct {
	Title    string
	URL      string
	Duration string
	Engine   string
}

// TUISearchDoneMsg is sent when search completes
// Per AI.md PART 1: Type names MUST be specific - "searchDoneMsg" is ambiguous
type TUISearchDoneMsg struct {
	results []TUISearchResult
	err     error
}

type TUIViewDoneMsg struct {
	view string
	rows []string
	err  error
}

// createInitialTUIModel creates the initial TUI model
// Per AI.md PART 1: Function names MUST reveal intent - "initialModel" is ambiguous
func createInitialTUIModel() TUIModel {
	// Initialize styles from theme palette
	// Per AI.md PART 16 "CLI/TUI Color Mapping": ANSI TerminalPalette is the
	// required baseline; literal hex ColorPalette is an opt-in enhancement
	// only for terminals that report true-color support.
	themeMode := TUIDefaultTheme
	if cliConfig != nil && cliConfig.TUI.Theme != "" {
		themeMode = cliConfig.TUI.Theme
	}
	if theme.SupportsTrueColor() {
		tuiStyles = createTUIStylesFromPalette(theme.GetColorPalette(themeMode))
	} else {
		tuiStyles = createTUIStylesFromTerminalPalette(theme.GetTerminalPalette(themeMode))
	}

	// Get initial terminal size and layout config
	termSize := terminal.GetTerminalSize()
	layoutConfig := getTUILayoutConfig(termSize.Mode)

	return TUIModel{
		language:     resolveCLILanguage(),
		activeView:   "menu",
		engineFilter: "all",
		loginServerURL: func() string {
			if cliConfig != nil {
				return cliConfig.Server.Address
			}
			return ""
		}(),
		probeQuery:     ProbeDefaultTestQuery,
		probeEngines:   "*",
		searchQuery:    "",
		searchResults:  nil,
		terminalWidth:  termSize.Cols,
		terminalHeight: termSize.Rows,
		sizeMode:       termSize.Mode,
		layoutConfig:   layoutConfig,
	}
}

// Init implements tea.Model
func (m TUIModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "?" {
			if m.activeView == TUICommandSearch {
				m.showShortcutsHelp = !m.showShortcutsHelp
			} else {
				m.showHelp = !m.showHelp
			}
			return m, nil
		}
		if msg.String() == "esc" && m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if msg.String() == "esc" && m.showShortcutsHelp {
			m.showShortcutsHelp = false
			return m, nil
		}
		if m.activeView != "menu" && m.activeView != TUICommandProbe && m.activeView != TUICommandLogin && (msg.String() == "ctrl+c" || (msg.String() == "q" && m.activeView != TUICommandSearch)) {
			m.isQuitting = true
			return m, tea.Quit
		}
		if m.activeView == "menu" {
			switch msg.String() {
			case "ctrl+c", "q":
				m.isQuitting = true
				return m, tea.Quit
			case "up", "k":
				if m.menuIndex > 0 {
					m.menuIndex--
				}
			case "down", "j":
				if m.menuIndex < len(tuiCommandOrder)-1 {
					m.menuIndex++
				}
			case "enter":
				m.activeView = tuiCommandOrder[m.menuIndex]
				m.viewRows = nil
				m.lastError = nil
				m.statusMessage = ""
				if m.activeView == TUICommandSetup {
					m.setupRequested = true
					return m, tea.Quit
				}
				if m.activeView == TUICommandEngines || m.activeView == TUICommandBangs || m.activeView == TUICommandHealth || m.activeView == TUICommandFavorites || m.activeView == TUICommandUpdate {
					m.viewLoading = true
					return m, loadTUIView(m.activeView, m.language)
				}
			}
			return m, nil
		}
		if msg.String() == "esc" && m.activeView == TUICommandSearch {
			if m.showShortcutsHelp {
				m.showShortcutsHelp = false
				return m, nil
			}
			if m.searchQuery != "" || len(m.searchResults) > 0 {
				m.searchQuery = ""
				m.searchResults = nil
				m.lastError = nil
				m.statusMessage = ""
				m.openedURL = ""
				return m, nil
			}
		}
		if msg.String() == "esc" {
			m.activeView = "menu"
			m.viewLoading = false
			m.lastError = nil
			m.statusMessage = ""
			return m, nil
		}
		if m.activeView != TUICommandSearch {
			return m.updateCommandViewKey(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			if m.searchQuery == "" || len(m.searchResults) > 0 {
				m.isQuitting = true
				return m, tea.Quit
			}
			// Clear query if not empty
			m.searchQuery = ""
			m.searchResults = nil
			return m, nil

		case "enter":
			if m.searchQuery != "" && !m.isLoading && len(m.searchResults) == 0 {
				// No results yet, perform search
				m.isLoading = true
				m.statusMessage = ""
				page, _ := strconv.Atoi(m.searchPage)
				limit, _ := strconv.Atoi(m.searchLimit)
				var engines []string
				if strings.TrimSpace(m.searchEngines) != "" {
					engines = strings.Split(m.searchEngines, ",")
				}
				return m, executeTUISearchWithOptions(m.searchQuery, page, limit, engines)
			}
			if len(m.searchResults) > 0 && m.selectedIndex < len(m.searchResults) {
				// Open selected result in browser
				selectedURL := m.searchResults[m.selectedIndex].URL
				if browser.CanOpenBrowser() {
					if err := browser.OpenURL(selectedURL); err != nil {
						m.lastError = err
					} else {
						m.openedURL = selectedURL
						m.statusMessage = tuiTranslate(m.language, "opened_browser")
					}
				} else {
					// Can't open browser, show URL
					m.statusMessage = i18n.TranslateFormat(m.language, "tui.url", "url", selectedURL)
				}
				return m, nil
			}

		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			ensureTUISearchSelectionVisible(&m)

		case "down", "j":
			if m.selectedIndex < len(m.searchResults)-1 {
				m.selectedIndex++
			}
			ensureTUISearchSelectionVisible(&m)

		case "backspace":
			switch m.searchField {
			case 0:
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				}
			case 1:
				if len(m.searchPage) > 0 {
					m.searchPage = m.searchPage[:len(m.searchPage)-1]
				}
			case 2:
				if len(m.searchLimit) > 0 {
					m.searchLimit = m.searchLimit[:len(m.searchLimit)-1]
				}
			case 3:
				if len(m.searchEngines) > 0 {
					m.searchEngines = m.searchEngines[:len(m.searchEngines)-1]
				}
			}
		case "tab":
			m.searchField = (m.searchField + 1) % 4
		case "g":
			if len(m.searchResults) > 0 {
				m.selectedIndex = 0
				m.searchScroll = 0
			}
		case "G":
			if len(m.searchResults) > 0 {
				m.selectedIndex = len(m.searchResults) - 1
				ensureTUISearchSelectionVisible(&m)
			}
		case "home":
			m.selectedIndex = 0
			m.searchScroll = 0
		case "end":
			m.selectedIndex = len(m.searchResults) - 1
			ensureTUISearchSelectionVisible(&m)
		case "pageup":
			m.selectedIndex -= 10
			if m.selectedIndex < 0 {
				m.selectedIndex = 0
			}
			ensureTUISearchSelectionVisible(&m)
		case "pagedown":
			m.selectedIndex += 10
			if m.selectedIndex >= len(m.searchResults) {
				m.selectedIndex = len(m.searchResults) - 1
			}
			ensureTUISearchSelectionVisible(&m)
		case "esc":
			if m.showShortcutsHelp {
				m.showShortcutsHelp = false
				return m, nil
			}
			m.searchQuery = ""
			m.searchResults = nil
			m.lastError = nil
			m.statusMessage = ""
			m.openedURL = ""
		case "/":
			// New search - clear results and focus on query
			m.searchResults = nil
			m.selectedIndex = 0
			m.searchField = 0
			m.statusMessage = ""
			m.openedURL = ""
			return m, nil
		case "?":
			m.showShortcutsHelp = !m.showShortcutsHelp
			return m, nil
		case "o":
			// Open current result without navigating
			if len(m.searchResults) > 0 && m.selectedIndex < len(m.searchResults) {
				selectedURL := m.searchResults[m.selectedIndex].URL
				if browser.CanOpenBrowser() {
					if err := browser.OpenURL(selectedURL); err != nil {
						m.lastError = err
					} else {
						m.statusMessage = tuiTranslate(m.language, "opened_browser")
					}
				}
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				switch m.searchField {
				case 0:
					m.searchQuery += msg.String()
				case 1:
					if msg.String() >= "0" && msg.String() <= "9" {
						m.searchPage += msg.String()
					}
				case 2:
					if msg.String() >= "0" && msg.String() <= "9" {
						m.searchLimit += msg.String()
					}
				case 3:
					m.searchEngines += msg.String()
				}
			}
		}

	case tea.WindowSizeMsg:
		// Per AI.md PART 32: Window Resize Handling
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height

		// Recalculate size mode and layout config
		m.sizeMode = terminal.GetSizeMode(msg.Width, msg.Height)
		m.layoutConfig = getTUILayoutConfig(m.sizeMode)
		ensureTUISearchSelectionVisible(&m)

	case TUISearchDoneMsg:
		m.isLoading = false
		m.searchResults = msg.results
		m.lastError = msg.err
		m.selectedIndex = 0
	case TUIViewDoneMsg:
		if msg.view != m.activeView {
			return m, nil
		}
		m.viewLoading = false
		m.viewRows = msg.rows
		m.lastError = msg.err
		if msg.err == nil {
			m.statusMessage = tuiTranslate(m.language, "completed")
		}
	}

	return m, nil
}

// View implements tea.Model
// Per AI.md PART 32: Responsive Layout based on SizeMode
func (m TUIModel) View() string {
	if m.isQuitting {
		return ""
	}

	var viewBuilder strings.Builder
	layout := m.layoutConfig
	if m.activeView == "menu" {
		viewBuilder.WriteString(tuiStyles.Title.Render("VidVeil — "+tuiTranslate(m.language, "main_menu")) + "\n\n")
		for index, command := range tuiCommandOrder {
			line := command
			if index == m.menuIndex {
				line = "> " + line
			} else {
				line = "  " + line
			}
			viewBuilder.WriteString(line + "\n")
		}
		viewBuilder.WriteString("\n" + tuiTranslate(m.language, "menu_help"))
		if m.showHelp {
			viewBuilder.WriteString("\n" + tuiTranslate(m.language, "menu_help_detail"))
		}
		return viewBuilder.String()
	}
	viewTitle := tuiTranslate(m.language, "screen."+m.activeView)
	viewBuilder.WriteString(tuiStyles.Title.Render("VidVeil — "+viewTitle) + "\n\n")
	if m.activeView != TUICommandSearch {
		if m.viewLoading {
			viewBuilder.WriteString(tuiStyles.Status.Render(tuiTranslate(m.language, "loading")) + "\n")
		}
		if m.lastError != nil {
			viewBuilder.WriteString(tuiStyles.Error.Render(m.lastError.Error()) + "\n")
		}
		for rowIndex, row := range m.viewRows {
			if rowIndex < m.viewScroll {
				continue
			}
			if m.terminalHeight > 0 && rowIndex >= m.viewScroll+m.terminalHeight-TUIReservedSpaceForUI {
				break
			}
			if m.activeView == TUICommandEngines && m.engineFilter == "enabled" && !strings.Contains(row, "enabled=true") {
				continue
			}
			if m.activeView == TUICommandEngines && m.engineFilter == "disabled" && !strings.Contains(row, "enabled=false") {
				continue
			}
			if m.activeView == TUICommandBangs && m.bangFilter != "" && !strings.Contains(strings.ToLower(row), strings.ToLower(m.bangFilter)) {
				continue
			}
			viewBuilder.WriteString(row + "\n")
		}
		switch m.activeView {
		case TUICommandProbe:
			verbose := tuiTranslate(m.language, "off")
			if m.probeVerbose {
				verbose = tuiTranslate(m.language, "on")
			}
			viewBuilder.WriteString(tuiTranslate(m.language, "engines") + ": " + m.probeEngines + "\n" + tuiTranslate(m.language, "query") + ": " + m.probeQuery + "\n" + tuiTranslate(m.language, "verbose") + ": " + verbose + "\n" + tuiTranslate(m.language, "probe_instructions"))
		case TUICommandLogin:
			serverPrefix, tokenPrefix := "  ", "  "
			if m.loginField == 0 {
				serverPrefix = "> "
			} else {
				tokenPrefix = "> "
			}
			viewBuilder.WriteString(serverPrefix + tuiTranslate(m.language, "server_url") + ": " + m.loginServerURL + "\n")
			viewBuilder.WriteString(tokenPrefix + tuiTranslate(m.language, "api_token") + ": " + strings.Repeat("•", len(m.loginToken)) + "\n" + tuiTranslate(m.language, "login_instructions"))
		case TUICommandHealth:
			viewBuilder.WriteString(tuiTranslate(m.language, "health_instructions"))
		case TUICommandFavorites:
			viewBuilder.WriteString(tuiTranslate(m.language, "favorites_instructions") + "\n")
			switch m.utilityMode {
			case "add":
				labels := []string{"favorite_url", "favorite_title", "favorite_thumbnail", "favorite_source"}
				for index, label := range labels {
					prefix := "  "
					value := m.utilityValues[index]
					if index == m.utilityField {
						prefix = "> "
						value += "_"
					}
					viewBuilder.WriteString(prefix + tuiTranslate(m.language, label) + ": " + value + "\n")
				}
			case "remove":
				viewBuilder.WriteString(tuiTranslate(m.language, "favorite_id") + ": " + m.utilityValues[0] + "_\n")
			}
		case TUICommandUpdate:
			viewBuilder.WriteString(tuiTranslate(m.language, "update_instructions"))
		case TUICommandSetup:
			viewBuilder.WriteString(tuiTranslate(m.language, "setup_instructions"))
		default:
			if m.activeView == TUICommandEngines {
				viewBuilder.WriteString(tuiTranslate(m.language, "engines_instructions") + "\n")
			}
			if m.activeView == TUICommandBangs {
				viewBuilder.WriteString(tuiTranslate(m.language, "filter") + ": " + m.bangFilter + " · " + tuiTranslate(m.language, "filter_instructions") + "\n")
			}
			if m.statusMessage != "" {
				viewBuilder.WriteString(tuiStyles.Status.Render(m.statusMessage) + "\n")
			}
		}
		viewBuilder.WriteString("\n" + tuiTranslate(m.language, "view_footer"))
		if m.showHelp {
			viewBuilder.WriteString("\n" + tuiTranslate(m.language, "view_help_detail"))
		}
		return viewBuilder.String()
	}

	// Title - show based on layout config
	if layout.ShowHeader {
		viewBuilder.WriteString(tuiStyles.Title.Render("VidVeil "+tuiTranslate(m.language, "tui_title")) + "\n\n")
	}

	// Search input
	searchFields := []struct{ label, value string }{
		{tuiTranslate(m.language, "query"), m.searchQuery}, {tuiTranslate(m.language, "page"), m.searchPage}, {tuiTranslate(m.language, "limit"), m.searchLimit}, {tuiTranslate(m.language, "engines"), m.searchEngines},
	}
	for index, field := range searchFields {
		prefix := "  "
		if m.searchField == index {
			prefix = "> "
		}
		value := field.value
		if m.searchField == index {
			value += "_"
		}
		viewBuilder.WriteString(prefix + field.label + ": " + tuiStyles.Input.Render(value) + "\n")
	}
	viewBuilder.WriteString(tuiTranslate(m.language, "search_instructions") + "\n\n")

	// Status
	if m.isLoading {
		viewBuilder.WriteString(tuiStyles.Status.Render(tuiTranslate(m.language, "searching")) + "\n\n")
	} else if m.lastError != nil {
		viewBuilder.WriteString(tuiStyles.Error.Render(tuiTranslate(m.language, "error_prefix")+": "+m.lastError.Error()) + "\n\n")
	} else if m.statusMessage != "" {
		viewBuilder.WriteString(tuiStyles.Status.Render(m.statusMessage) + "\n\n")
	}

	if m.showShortcutsHelp {
		helpLines := []string{
			tuiTranslate(m.language, "shortcuts"),
			"  enter  " + tuiTranslate(m.language, "shortcut_search_open"),
			"  /      " + tuiTranslate(m.language, "shortcut_new_search"),
			"  j/k    " + tuiTranslate(m.language, "shortcut_navigate"),
			"  o      " + tuiTranslate(m.language, "shortcut_browser"),
			"  esc    " + tuiTranslate(m.language, "shortcut_escape"),
			"  q      " + tuiTranslate(m.language, "shortcut_quit"),
			"  ?      " + tuiTranslate(m.language, "shortcut_toggle_help"),
		}
		viewBuilder.WriteString(tuiStyles.Help.Render(strings.Join(helpLines, "\n")) + "\n\n")
	}

	// Results
	if len(m.searchResults) > 0 {
		fmt.Fprintf(&viewBuilder, "%s\n", i18n.TranslateFormat(m.language, "tui.results", "count", len(m.searchResults)))
		if layout.ShowBorders {
			viewBuilder.WriteString(strings.Repeat("-", 50) + "\n")
		}

		// Determine how many results to show based on terminal height
		// Per AI.md PART 1: No magic numbers - use named constants
		maxResults := TUIDefaultMaxResults
		if m.terminalHeight > 0 {
			maxResults = m.terminalHeight - TUIReservedSpaceForUI
			if maxResults < TUIMinDisplayResults {
				maxResults = TUIMinDisplayResults
			}
			if maxResults > len(m.searchResults) {
				maxResults = len(m.searchResults)
			}
		}

		if m.searchScroll > 0 {
			viewBuilder.WriteString(tuiStyles.Muted.Render(i18n.TranslateFormat(m.language, "tui.results_above", "count", m.searchScroll) + "\n"))
		}
		end := m.searchScroll + maxResults
		if end > len(m.searchResults) {
			end = len(m.searchResults)
		}
		for i := m.searchScroll; i < end; i++ {
			result := m.searchResults[i]

			// Truncate based on layout config
			// Per AI.md PART 1: No magic numbers - use named constants
			truncateAt := layout.TruncateAt
			if truncateAt == 0 || truncateAt > m.terminalWidth-TUITruncationPadding {
				truncateAt = m.terminalWidth - TUITruncationPadding
			}
			if truncateAt < TUIMinTruncationWidth {
				truncateAt = TUIMinTruncationWidth
			}

			line := fmt.Sprintf("  %s [%s] - %s", result.Title, result.Duration, result.Engine)
			if len(line) > truncateAt {
				line = line[:truncateAt-3] + "..."
			}

			if i == m.selectedIndex {
				viewBuilder.WriteString(tuiStyles.Selected.Render("> "+line) + "\n")
			} else {
				viewBuilder.WriteString(tuiStyles.Result.Render("  "+line) + "\n")
			}
		}
		if end < len(m.searchResults) {
			viewBuilder.WriteString(tuiStyles.Muted.Render(i18n.TranslateFormat(m.language, "tui.results_below", "count", len(m.searchResults)-end) + "\n"))
		}
		viewBuilder.WriteString("\n")
	}

	// Help - show based on layout config
	if layout.ShowFooter {
		var helpText string
		if len(m.searchResults) > 0 {
			helpText = tuiTranslate(m.language, "footer_results")
			if layout.UseAbbrev {
				helpText = tuiTranslate(m.language, "footer_results_short")
			}
		} else {
			helpText = tuiTranslate(m.language, "footer_empty")
			if layout.UseAbbrev {
				helpText = tuiTranslate(m.language, "footer_empty_short")
			}
		}
		viewBuilder.WriteString(tuiStyles.Help.Render(helpText))
	}

	return viewBuilder.String()
}

func visibleTUISearchResultCount(model TUIModel) int {
	if model.terminalHeight <= 0 {
		return TUIDefaultMaxResults
	}
	visible := model.terminalHeight - TUIReservedSpaceForUI
	if visible < TUIMinDisplayResults {
		visible = TUIMinDisplayResults
	}
	if visible > len(model.searchResults) {
		visible = len(model.searchResults)
	}
	return visible
}

func ensureTUISearchSelectionVisible(model *TUIModel) {
	if len(model.searchResults) == 0 {
		model.searchScroll = 0
		return
	}
	visible := visibleTUISearchResultCount(*model)
	if visible < 1 {
		visible = 1
	}
	if model.selectedIndex < model.searchScroll {
		model.searchScroll = model.selectedIndex
	}
	if model.selectedIndex >= model.searchScroll+visible {
		model.searchScroll = model.selectedIndex - visible + 1
	}
	maxScroll := len(model.searchResults) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if model.searchScroll > maxScroll {
		model.searchScroll = maxScroll
	}
}

func (m TUIModel) updateCommandViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.activeView {
	case TUICommandEngines, TUICommandBangs:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.isQuitting = true
			return m, tea.Quit
		}
		if msg.String() == "r" && !m.viewLoading {
			m.viewLoading = true
			return m, loadTUIView(m.activeView, m.language)
		}
		visibleRows := m.terminalHeight - TUIReservedSpaceForUI
		if visibleRows < 1 {
			visibleRows = 1
		}
		switch msg.String() {
		case "up", "k":
			if m.viewScroll > 0 {
				m.viewScroll--
			}
			return m, nil
		case "down", "j":
			maxScroll := len(m.viewRows) - visibleRows
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.viewScroll < maxScroll {
				m.viewScroll++
			}
			return m, nil
		case "pageup":
			m.viewScroll -= visibleRows
			if m.viewScroll < 0 {
				m.viewScroll = 0
			}
			return m, nil
		case "pagedown":
			m.viewScroll += visibleRows
			maxScroll := len(m.viewRows) - visibleRows
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.viewScroll > maxScroll {
				m.viewScroll = maxScroll
			}
			return m, nil
		}
		if m.activeView == TUICommandEngines {
			switch msg.String() {
			case "e":
				m.engineFilter = "enabled"
			case "d":
				m.engineFilter = "disabled"
			case "a":
				m.engineFilter = "all"
			}
		} else {
			switch msg.String() {
			case "backspace":
				if len(m.bangFilter) > 0 {
					m.bangFilter = m.bangFilter[:len(m.bangFilter)-1]
				}
			case "enter":
				m.bangFilter = ""
			default:
				if len(msg.String()) == 1 {
					m.bangFilter += msg.String()
				}
			}
		}
	case TUICommandProbe:
		switch msg.String() {
		case "tab":
			m.loginField = (m.loginField + 1) % 2
		case "enter":
			m.viewLoading = true
			return m, runTUIProbe(m.probeEngines, m.probeQuery, m.probeVerbose, m.language)
		case "v":
			m.probeVerbose = !m.probeVerbose
		case "backspace":
			if m.loginField == 0 && len(m.probeEngines) > 0 {
				m.probeEngines = m.probeEngines[:len(m.probeEngines)-1]
			}
			if m.loginField == 1 && len(m.probeQuery) > 0 {
				m.probeQuery = m.probeQuery[:len(m.probeQuery)-1]
			}
		default:
			if len(msg.String()) == 1 {
				if m.loginField == 0 {
					m.probeEngines += msg.String()
				} else {
					m.probeQuery += msg.String()
				}
			}
		}
	case TUICommandLogin:
		switch msg.String() {
		case "tab":
			m.loginField = (m.loginField + 1) % 2
		case "enter":
			if strings.TrimSpace(m.loginServerURL) == "" || strings.TrimSpace(m.loginToken) == "" {
				m.lastError = fmt.Errorf("%s", tuiTranslate(m.language, "error.login_required"))
				return m, nil
			}
			if err := saveCLILogin(m.loginServerURL, m.loginToken); err != nil {
				m.lastError = err
			} else {
				if cliConfig == nil {
					cliConfig = &CLIConfig{}
				}
				cliConfig.Server.Address = m.loginServerURL
				cliConfig.Server.Token = m.loginToken
				initAPIClient()
				m.loginToken = ""
				m.statusMessage = tuiTranslate(m.language, "login_saved")
			}
		case "backspace":
			if m.loginField == 0 && len(m.loginServerURL) > 0 {
				m.loginServerURL = m.loginServerURL[:len(m.loginServerURL)-1]
			}
			if m.loginField == 1 && len(m.loginToken) > 0 {
				m.loginToken = m.loginToken[:len(m.loginToken)-1]
			}
		default:
			if len(msg.String()) == 1 {
				if m.loginField == 0 {
					m.loginServerURL += msg.String()
				} else {
					m.loginToken += msg.String()
				}
			}
		}
	case TUICommandHealth:
		if msg.String() == "r" {
			m.viewLoading = true
			return m, loadTUIView(m.activeView, m.language)
		}
	case TUICommandUpdate:
		switch msg.String() {
		case "c":
			m.viewLoading = true
			return m, loadTUIView(m.activeView, m.language)
		case "y":
			if err := RunCLIUpdateCommand([]string{"yes"}); err != nil {
				m.lastError = err
			} else {
				m.statusMessage = tuiTranslate(m.language, "update_started")
			}
		}
	case TUICommandFavorites:
		if m.statusMessage == tuiTranslate(m.language, "favorites_clear_confirm") && msg.String() != "y" {
			m.statusMessage = ""
			return m, nil
		}
		if m.utilityMode == "add" || m.utilityMode == "remove" {
			switch msg.String() {
			case "tab":
				fieldCount := 1
				if m.utilityMode == "add" {
					fieldCount = 4
				}
				m.utilityField = (m.utilityField + 1) % fieldCount
			case "ctrl+s":
				if apiClient == nil {
					m.lastError = fmt.Errorf("%s", tuiTranslate(m.language, "error.server_unconfigured"))
					break
				}
				if m.utilityMode == "add" {
					err := apiClient.AddFavorite(api.Favorite{URL: strings.TrimSpace(m.utilityValues[0]), Title: strings.TrimSpace(m.utilityValues[1]), Thumbnail: strings.TrimSpace(m.utilityValues[2]), Source: strings.TrimSpace(m.utilityValues[3])})
					if err != nil {
						m.lastError = err
					} else {
						m.utilityMode = ""
						m.utilityValues = [4]string{}
						m.statusMessage = tuiTranslate(m.language, "favorite_added")
						return m, loadTUIView(m.activeView, m.language)
					}
				} else {
					id, err := strconv.ParseInt(strings.TrimSpace(m.utilityValues[0]), 10, 64)
					if err != nil || id <= 0 {
						m.lastError = fmt.Errorf("%s", tuiTranslate(m.language, "error.favorite_id_invalid"))
						break
					}
					if err := apiClient.RemoveFavorite(id); err != nil {
						m.lastError = err
					} else {
						m.utilityMode = ""
						m.utilityValues = [4]string{}
						m.statusMessage = tuiTranslate(m.language, "favorite_removed")
						return m, loadTUIView(m.activeView, m.language)
					}
				}
			case "backspace":
				if len(m.utilityValues[m.utilityField]) > 0 {
					m.utilityValues[m.utilityField] = m.utilityValues[m.utilityField][:len(m.utilityValues[m.utilityField])-1]
				}
			default:
				if len(msg.String()) == 1 {
					m.utilityValues[m.utilityField] += msg.String()
				}
			}
		} else {
			switch msg.String() {
			case "a":
				m.utilityMode = "add"
				m.utilityField = 0
				m.utilityValues = [4]string{}
			case "r":
				m.utilityMode = "remove"
				m.utilityField = 0
				m.utilityValues = [4]string{}
			case "l":
				return m, loadTUIView(m.activeView, m.language)
			case "c":
				m.statusMessage = tuiTranslate(m.language, "favorites_clear_confirm")
			case "y":
				if m.statusMessage == tuiTranslate(m.language, "favorites_clear_confirm") {
					if apiClient == nil {
						m.lastError = fmt.Errorf("%s", tuiTranslate(m.language, "error.server_unconfigured"))
						break
					}
					if err := apiClient.ClearFavorites(); err != nil {
						m.lastError = err
					} else {
						m.statusMessage = tuiTranslate(m.language, "favorites_cleared")
						return m, loadTUIView(m.activeView, m.language)
					}
				}
			}
		}
	}
	return m, nil
}

func loadTUIView(view, language string) tea.Cmd {
	return func() tea.Msg {
		var rows []string
		switch view {
		case TUICommandHealth:
			if apiClient == nil {
				return TUIViewDoneMsg{view: view, err: fmt.Errorf("%s", tuiTranslate(language, "error.server_unconfigured"))}
			}
			healthy, err := apiClient.Health()
			if err != nil {
				return TUIViewDoneMsg{view: view, err: err}
			}
			if !healthy {
				return TUIViewDoneMsg{view: view, err: fmt.Errorf("%s", tuiTranslate(language, "error.health_failed"))}
			}
			return TUIViewDoneMsg{view: view, rows: []string{tuiTranslate(language, "health_healthy")}}
		case TUICommandFavorites:
			if apiClient == nil {
				return TUIViewDoneMsg{view: view, err: fmt.Errorf("%s", tuiTranslate(language, "error.server_unconfigured"))}
			}
			favorites, err := apiClient.ListFavorites()
			if err != nil {
				return TUIViewDoneMsg{view: view, err: err}
			}
			for _, favorite := range favorites {
				rows = append(rows, i18n.TranslateFormat(language, "tui.favorite_row", "id", favorite.ID, "title", favorite.Title, "url", favorite.URL))
			}
			return TUIViewDoneMsg{view: view, rows: rows}
		case TUICommandUpdate:
			info, err := CheckCLIUpdate()
			if err != nil {
				return TUIViewDoneMsg{view: view, err: err}
			}
			if info == nil || !info.UpdateAvailable {
				return TUIViewDoneMsg{view: view, rows: []string{tuiTranslate(language, "update_current")}}
			}
			return TUIViewDoneMsg{view: view, rows: []string{i18n.TranslateFormat(language, "tui.update_available", "version", info.LatestVersion)}}
		case TUICommandEngines:
			data, err := FetchEnginesList()
			if err != nil {
				return TUIViewDoneMsg{view: view, err: err}
			}
			for _, item := range data.Engines {
				rows = append(rows, i18n.TranslateFormat(language, "tui.engine_summary", "display_name", item.DisplayName, "tier", item.Tier, "enabled", item.Enabled, "available", item.Available, "capabilities", item.Capabilities))
			}
		case TUICommandBangs:
			data, err := fetchBangsList()
			if err != nil {
				return TUIViewDoneMsg{view: view, err: err}
			}
			for _, item := range data.Bangs {
				rows = append(rows, i18n.TranslateFormat(language, "tui.bang_summary", "bang", item.Bang, "display_name", item.DisplayName, "engine", item.EngineName))
			}
		}
		return TUIViewDoneMsg{view: view, rows: rows}
	}
}

func runTUIProbe(engineList, query string, verbose bool, language string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(engineList) == "" {
			return TUIViewDoneMsg{view: TUICommandProbe, err: fmt.Errorf("%s", tuiTranslate(language, "error.engines_required"))}
		}
		if strings.TrimSpace(query) == "" {
			return TUIViewDoneMsg{view: TUICommandProbe, err: fmt.Errorf("%s", tuiTranslate(language, "error.probe_query_required"))}
		}
		engineNames := strings.Split(engineList, ",")
		if strings.TrimSpace(engineList) == "*" {
			var err error
			engineNames, err = fetchEngineList()
			if err != nil {
				return TUIViewDoneMsg{view: TUICommandProbe, err: err}
			}
		}
		var rows []string
		for _, name := range engineNames {
			result := probeEngineByName(strings.TrimSpace(name), query)
			state := "unavailable"
			if result.Available {
				state = "available"
			}
			if result.Error != "" {
				rows = append(rows, i18n.TranslateFormat(language, "tui.probe_error", "name", result.Name, "state", tuiTranslate(language, state), "error", result.Error))
			} else {
				rows = append(rows, i18n.TranslateFormat(language, "tui.probe_summary", "name", result.Name, "state", tuiTranslate(language, state), "count", result.ResultCount, "milliseconds", result.ResponseTimeMS))
			}
			if verbose && result.Capabilities != nil {
				rows = append(rows, i18n.TranslateFormat(language, "tui.probe_details", "capabilities", result.Capabilities, "fields", result.FieldStats))
			}
		}
		return TUIViewDoneMsg{view: TUICommandProbe, rows: rows}
	}
}

func executeTUISearchWithOptions(searchQuery string, page, limit int, engines []string) tea.Cmd {
	return func() tea.Msg {
		resp, err := apiClient.Search(searchQuery, page, limit, engines)
		if err != nil {
			return TUISearchDoneMsg{err: err}
		}

		var searchResults []TUISearchResult
		for _, result := range resp.Results {
			searchResults = append(searchResults, TUISearchResult{
				Title:    result.Title,
				URL:      result.URL,
				Duration: result.Duration,
				Engine:   result.Engine,
			})
		}

		return TUISearchDoneMsg{results: searchResults}
	}
}

// runInteractiveTUI runs the interactive TUI
// Per AI.md PART 1: Function names MUST reveal intent - "runTUI" is ambiguous
func runInteractiveTUI() error {
	for {
		tuiProgram := tea.NewProgram(createInitialTUIModel(), tea.WithAltScreen())
		finalModel, err := tuiProgram.Run()
		if err != nil {
			return err
		}
		model, ok := finalModel.(TUIModel)
		if !ok || !model.setupRequested {
			return nil
		}
		if err := runSetupWizard(); err != nil {
			return err
		}
		if err := LoadCLIConfigFromFile(); err != nil {
			return err
		}
		if err := initializeCLILogging(); err != nil {
			return err
		}
		initAPIClient()
	}
}
