// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client - TUI Command
package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Dracula colors per AI.md PART 36
var (
	colorBackground = lipgloss.Color("#282a36")
	colorForeground = lipgloss.Color("#f8f8f2")
	colorSelection  = lipgloss.Color("#44475a")
	colorComment    = lipgloss.Color("#6272a4")
	colorCyan       = lipgloss.Color("#8be9fd")
	colorGreen      = lipgloss.Color("#50fa7b")
	colorOrange     = lipgloss.Color("#ffb86c")
	colorPink       = lipgloss.Color("#ff79c6")
	colorPurple     = lipgloss.Color("#bd93f9")
	colorRed        = lipgloss.Color("#ff5555")
	colorYellow     = lipgloss.Color("#f1fa8c")
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(colorPurple).
			Bold(true).
			Padding(0, 1)

	inputStyle = lipgloss.NewStyle().
			Foreground(colorForeground).
			Background(colorSelection).
			Padding(0, 1)

	resultStyle = lipgloss.NewStyle().
			Foreground(colorForeground)

	selectedStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorComment)

	statusStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed)
)

type model struct {
	query    string
	cursor   int
	results  []searchResult
	selected int
	err      error
	loading  bool
	width    int
	height   int
	quitting bool
}

type searchResult struct {
	Title    string
	URL      string
	Duration string
	Engine   string
}

type searchDoneMsg struct {
	results []searchResult
	err     error
}

func initialModel() model {
	return model{
		query:   "",
		results: nil,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.query == "" || len(m.results) > 0 {
				m.quitting = true
				return m, tea.Quit
			}
			// Clear query if not empty
			m.query = ""
			m.results = nil
			return m, nil

		case "enter":
			if m.query != "" && !m.loading {
				m.loading = true
				return m, doSearch(m.query)
			}
			if len(m.results) > 0 && m.selected < len(m.results) {
				// Open selected result
				fmt.Printf("\nOpening: %s\n", m.results[m.selected].URL)
				return m, tea.Quit
			}

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if m.selected < len(m.results)-1 {
				m.selected++
			}

		case "backspace":
			if len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
			}

		case "esc":
			m.query = ""
			m.results = nil
			m.err = nil

		default:
			if len(msg.String()) == 1 {
				m.query += msg.String()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case searchDoneMsg:
		m.loading = false
		m.results = msg.results
		m.err = msg.err
		m.selected = 0
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("VidVeil TUI") + "\n\n")

	// Search input
	b.WriteString("Search: ")
	b.WriteString(inputStyle.Render(m.query + "_"))
	b.WriteString("\n\n")

	// Status
	if m.loading {
		b.WriteString(statusStyle.Render("Searching...") + "\n\n")
	} else if m.err != nil {
		b.WriteString(errorStyle.Render("Error: "+m.err.Error()) + "\n\n")
	}

	// Results
	if len(m.results) > 0 {
		b.WriteString(fmt.Sprintf("Results (%d):\n", len(m.results)))
		b.WriteString(strings.Repeat("-", 50) + "\n")

		for i, r := range m.results {
			if i >= 10 {
				b.WriteString(fmt.Sprintf("  ... and %d more\n", len(m.results)-10))
				break
			}

			line := fmt.Sprintf("  %s [%s] - %s", r.Title, r.Duration, r.Engine)
			if len(line) > 70 {
				line = line[:67] + "..."
			}

			if i == m.selected {
				b.WriteString(selectedStyle.Render("> "+line) + "\n")
			} else {
				b.WriteString(resultStyle.Render("  "+line) + "\n")
			}
		}
		b.WriteString("\n")
	}

	// Help
	help := "q: quit | enter: search/open | esc: clear | j/k: navigate"
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Search(query, 0, 20, nil, false)
		if err != nil {
			return searchDoneMsg{err: err}
		}

		var results []searchResult
		for _, r := range resp.Results {
			results = append(results, searchResult{
				Title:    r.Title,
				URL:      r.URL,
				Duration: r.Duration,
				Engine:   r.Engine,
			})
		}

		return searchDoneMsg{results: results}
	}
}

func runTUI() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
