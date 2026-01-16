// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - TUI Command
package cmd

import (
	"fmt"
	"strings"

	"github.com/apimgr/vidveil/src/common/terminal"
	"github.com/apimgr/vidveil/src/common/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TUIStyles holds lipgloss styles derived from theme.ColorPalette
// Per AI.md PART 36: TUI Styles from Palette
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
// Per AI.md PART 36: Responsive Layout
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

// CreateTUIStylesFromPalette creates TUIStyles from theme.ColorPalette
// Per AI.md PART 36: TUI Styles from Palette
// Per AI.md PART 1: Function names MUST reveal intent
func CreateTUIStylesFromPalette(palette theme.ColorPalette) TUIStyles {
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

// GetTUILayoutConfig returns layout config for a terminal.SizeMode
// Per AI.md PART 36: Responsive Layout
// Per AI.md PART 1: Function names MUST reveal intent
func GetTUILayoutConfig(sizeMode terminal.SizeMode) TUILayoutConfig {
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
			ShowBorders:    true,
			ShowHeader:     true,
			ShowFooter:     true,
			ShowSidebar:    true,
			SidebarWidth:   50,
			MaxColumns:     20,
			TruncateAt:     0, // No truncation
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
	searchQuery     string
	cursorPosition  int
	searchResults   []TUISearchResult
	selectedIndex   int
	lastError       error
	isLoading       bool
	terminalWidth   int
	terminalHeight  int
	isQuitting      bool
	sizeMode        terminal.SizeMode
	layoutConfig    TUILayoutConfig
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

// CreateInitialTUIModel creates the initial TUI model
// Per AI.md PART 1: Function names MUST reveal intent - "initialModel" is ambiguous
func CreateInitialTUIModel() TUIModel {
	// Initialize styles from theme palette
	// Per AI.md PART 36: TUI uses theme.ColorPalette from src/common/theme
	themeMode := "dark"
	if cliConfig != nil && cliConfig.TUI.Theme != "" {
		themeMode = cliConfig.TUI.Theme
	}
	tuiStyles = CreateTUIStylesFromPalette(theme.GetColorPalette(themeMode))

	// Get initial terminal size and layout config
	termSize := terminal.GetTerminalSize()
	layoutConfig := GetTUILayoutConfig(termSize.Mode)

	return TUIModel{
		searchQuery:   "",
		searchResults: nil,
		terminalWidth: termSize.Cols,
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
			if m.searchQuery != "" && !m.isLoading {
				m.isLoading = true
				return m, ExecuteTUISearch(m.searchQuery)
			}
			if len(m.searchResults) > 0 && m.selectedIndex < len(m.searchResults) {
				// Open selected result
				fmt.Printf("\nOpening: %s\n", m.searchResults[m.selectedIndex].URL)
				return m, tea.Quit
			}

		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}

		case "down", "j":
			if m.selectedIndex < len(m.searchResults)-1 {
				m.selectedIndex++
			}

		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			}

		case "esc":
			m.searchQuery = ""
			m.searchResults = nil
			m.lastError = nil

		default:
			if len(msg.String()) == 1 {
				m.searchQuery += msg.String()
			}
		}

	case tea.WindowSizeMsg:
		// Per AI.md PART 36: Window Resize Handling
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height

		// Recalculate size mode and layout config
		m.sizeMode = terminal.GetTerminalSize().Mode
		m.layoutConfig = GetTUILayoutConfig(m.sizeMode)

	case TUISearchDoneMsg:
		m.isLoading = false
		m.searchResults = msg.results
		m.lastError = msg.err
		m.selectedIndex = 0
	}

	return m, nil
}

// View implements tea.Model
// Per AI.md PART 36: Responsive Layout based on SizeMode
func (m TUIModel) View() string {
	if m.isQuitting {
		return ""
	}

	var viewBuilder strings.Builder
	layout := m.layoutConfig

	// Title - show based on layout config
	if layout.ShowHeader {
		viewBuilder.WriteString(tuiStyles.Title.Render("VidVeil TUI") + "\n\n")
	}

	// Search input
	viewBuilder.WriteString("Search: ")
	viewBuilder.WriteString(tuiStyles.Input.Render(m.searchQuery + "_"))
	viewBuilder.WriteString("\n\n")

	// Status
	if m.isLoading {
		viewBuilder.WriteString(tuiStyles.Status.Render("Searching...") + "\n\n")
	} else if m.lastError != nil {
		viewBuilder.WriteString(tuiStyles.Error.Render("Error: "+m.lastError.Error()) + "\n\n")
	}

	// Results
	if len(m.searchResults) > 0 {
		viewBuilder.WriteString(fmt.Sprintf("Results (%d):\n", len(m.searchResults)))
		if layout.ShowBorders {
			viewBuilder.WriteString(strings.Repeat("-", 50) + "\n")
		}

		// Determine how many results to show based on terminal height
		maxResults := 10
		if m.terminalHeight > 0 {
			maxResults = m.terminalHeight - 10 // Reserve space for header/footer
			if maxResults < 3 {
				maxResults = 3
			}
			if maxResults > len(m.searchResults) {
				maxResults = len(m.searchResults)
			}
		}

		for i, result := range m.searchResults {
			if i >= maxResults {
				viewBuilder.WriteString(tuiStyles.Muted.Render(fmt.Sprintf("  ... and %d more\n", len(m.searchResults)-maxResults)))
				break
			}

			// Truncate based on layout config
			truncateAt := layout.TruncateAt
			if truncateAt == 0 || truncateAt > m.terminalWidth-10 {
				truncateAt = m.terminalWidth - 10
			}
			if truncateAt < 30 {
				truncateAt = 30
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
		viewBuilder.WriteString("\n")
	}

	// Help - show based on layout config
	if layout.ShowFooter {
		helpText := "q: quit | enter: search/open | esc: clear | j/k: navigate"
		if layout.UseAbbrev {
			helpText = "q:quit | ↵:search | esc:clear | j/k:nav"
		}
		viewBuilder.WriteString(tuiStyles.Help.Render(helpText))
	}

	return viewBuilder.String()
}

// ExecuteTUISearch performs a search from TUI
// Per AI.md PART 1: Function names MUST reveal intent - "doSearch" is ambiguous
func ExecuteTUISearch(searchQuery string) tea.Cmd {
	return func() tea.Msg {
		resp, err := apiClient.Search(searchQuery, 0, 20, nil, false)
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

// RunInteractiveTUI runs the interactive TUI
// Per AI.md PART 1: Function names MUST reveal intent - "runTUI" is ambiguous
func RunInteractiveTUI() error {
	tuiProgram := tea.NewProgram(CreateInitialTUIModel(), tea.WithAltScreen())
	_, err := tuiProgram.Run()
	return err
}
