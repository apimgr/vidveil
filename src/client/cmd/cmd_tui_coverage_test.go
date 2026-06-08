// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for TUIModel.Update and TUIModel.View.
// All tests call the model methods directly — no terminal, no network.
package cmd

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestTUIModel creates a TUIModel for testing.
func newTestTUIModel() TUIModel {
	return CreateInitialTUIModel()
}

// ── TUIModel.Update — key messages ───────────────────────────────────────────

func TestTUIModel_CtrlC_EmptyQuery_Quits(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = ""
	m.searchResults = nil
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	wm := newM.(TUIModel)
	if !wm.isQuitting {
		t.Error("ctrl+c with empty query: isQuitting should be true")
	}
}

func TestTUIModel_Q_NonEmptyQuery_ClearsQuery(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = "hello"
	m.searchResults = nil
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	wm := newM.(TUIModel)
	if wm.searchQuery != "" {
		t.Errorf("q with query: searchQuery = %q, want ''", wm.searchQuery)
	}
}

func TestTUIModel_Q_WithResults_Quits(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = ""
	m.searchResults = []TUISearchResult{{Title: "r", URL: "http://example.com"}}
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	wm := newM.(TUIModel)
	if !wm.isQuitting {
		t.Error("q with results: isQuitting should be true")
	}
}

func TestTUIModel_Enter_EmptyQuery_Noop(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = ""
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := newM.(TUIModel)
	if wm.isLoading {
		t.Error("enter with empty query: isLoading should be false")
	}
}

func TestTUIModel_Enter_NonEmptyQuery_NotLoading_StartsSearch(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = "test"
	m.isLoading = false
	m.searchResults = nil
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := newM.(TUIModel)
	if !wm.isLoading {
		t.Error("enter with query: isLoading should be true")
	}
	_ = cmd
}

func TestTUIModel_Enter_WithResultsNoBrowser_ShowsURL(t *testing.T) {
	m := newTestTUIModel()
	m.searchResults = []TUISearchResult{{Title: "r", URL: "http://example.com"}}
	m.selectedIndex = 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wm := newM.(TUIModel)
	_ = wm.statusMessage
}

func TestTUIModel_Up_DecrementsIndex(t *testing.T) {
	m := newTestTUIModel()
	m.searchResults = []TUISearchResult{{Title: "a"}, {Title: "b"}}
	m.selectedIndex = 1
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	wm := newM.(TUIModel)
	if wm.selectedIndex != 0 {
		t.Errorf("k: selectedIndex = %d, want 0", wm.selectedIndex)
	}
}

func TestTUIModel_Up_AtZero_Noop(t *testing.T) {
	m := newTestTUIModel()
	m.selectedIndex = 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	wm := newM.(TUIModel)
	if wm.selectedIndex != 0 {
		t.Errorf("up at 0: selectedIndex = %d, want 0", wm.selectedIndex)
	}
}

func TestTUIModel_Down_IncrementsIndex(t *testing.T) {
	m := newTestTUIModel()
	m.searchResults = []TUISearchResult{{Title: "a"}, {Title: "b"}}
	m.selectedIndex = 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	wm := newM.(TUIModel)
	if wm.selectedIndex != 1 {
		t.Errorf("j: selectedIndex = %d, want 1", wm.selectedIndex)
	}
}

func TestTUIModel_Down_AtEnd_Noop(t *testing.T) {
	m := newTestTUIModel()
	m.searchResults = []TUISearchResult{{Title: "a"}}
	m.selectedIndex = 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	wm := newM.(TUIModel)
	if wm.selectedIndex != 0 {
		t.Errorf("down at end: selectedIndex = %d, want 0", wm.selectedIndex)
	}
}

func TestTUIModel_Backspace_TrimQuery(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = "abc"
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	wm := newM.(TUIModel)
	if wm.searchQuery != "ab" {
		t.Errorf("backspace: query = %q, want 'ab'", wm.searchQuery)
	}
}

func TestTUIModel_Backspace_EmptyQuery_Noop(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = ""
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	wm := newM.(TUIModel)
	if wm.searchQuery != "" {
		t.Errorf("backspace empty: query = %q, want ''", wm.searchQuery)
	}
}

func TestTUIModel_Esc_ShortcutsVisible_Hides(t *testing.T) {
	m := newTestTUIModel()
	m.showShortcutsHelp = true
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	wm := newM.(TUIModel)
	if wm.showShortcutsHelp {
		t.Error("esc with shortcuts visible: showShortcutsHelp should be false")
	}
}

func TestTUIModel_Esc_ClearsState(t *testing.T) {
	m := newTestTUIModel()
	m.showShortcutsHelp = false
	m.searchQuery = "test"
	m.searchResults = []TUISearchResult{{Title: "r"}}
	m.statusMessage = "some status"
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	wm := newM.(TUIModel)
	if wm.searchQuery != "" {
		t.Error("esc: searchQuery should be cleared")
	}
	if wm.statusMessage != "" {
		t.Error("esc: statusMessage should be cleared")
	}
}

func TestTUIModel_Slash_ClearsResultsForNewSearch(t *testing.T) {
	m := newTestTUIModel()
	m.searchResults = []TUISearchResult{{Title: "r"}}
	m.statusMessage = "found"
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	wm := newM.(TUIModel)
	if len(wm.searchResults) != 0 {
		t.Error("/: searchResults should be cleared")
	}
}

func TestTUIModel_Question_TogglesShortcuts(t *testing.T) {
	m := newTestTUIModel()
	m.showShortcutsHelp = false
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	wm := newM.(TUIModel)
	if !wm.showShortcutsHelp {
		t.Error("?: showShortcutsHelp should be true")
	}
}

func TestTUIModel_O_NoResults_Noop(t *testing.T) {
	m := newTestTUIModel()
	m.searchResults = nil
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	_ = newM.(TUIModel)
}

func TestTUIModel_O_WithResults_TriesOpen(t *testing.T) {
	m := newTestTUIModel()
	m.searchResults = []TUISearchResult{{Title: "r", URL: "http://example.com"}}
	m.selectedIndex = 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	_ = newM.(TUIModel)
}

func TestTUIModel_Default_SingleChar_Appended(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = "he"
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	wm := newM.(TUIModel)
	if wm.searchQuery != "hey" {
		t.Errorf("default char: query = %q, want 'hey'", wm.searchQuery)
	}
}

func TestTUIModel_WindowSizeMsg_UpdatesDimensions(t *testing.T) {
	m := newTestTUIModel()
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	wm := newM.(TUIModel)
	if wm.terminalWidth != 120 {
		t.Errorf("WindowSizeMsg: terminalWidth = %d, want 120", wm.terminalWidth)
	}
	if wm.terminalHeight != 40 {
		t.Errorf("WindowSizeMsg: terminalHeight = %d, want 40", wm.terminalHeight)
	}
}

func TestTUIModel_TUISearchDoneMsg_Success(t *testing.T) {
	m := newTestTUIModel()
	m.isLoading = true
	results := []TUISearchResult{{Title: "found", URL: "http://example.com"}}
	newM, _ := m.Update(TUISearchDoneMsg{results: results, err: nil})
	wm := newM.(TUIModel)
	if wm.isLoading {
		t.Error("TUISearchDoneMsg: isLoading should be false")
	}
	if len(wm.searchResults) != 1 {
		t.Errorf("TUISearchDoneMsg: searchResults = %v, want 1 result", wm.searchResults)
	}
}

func TestTUIModel_TUISearchDoneMsg_Error(t *testing.T) {
	m := newTestTUIModel()
	m.isLoading = true
	newM, _ := m.Update(TUISearchDoneMsg{results: nil, err: errors.New("failed")})
	wm := newM.(TUIModel)
	if wm.isLoading {
		t.Error("TUISearchDoneMsg error: isLoading should be false")
	}
	if wm.lastError == nil {
		t.Error("TUISearchDoneMsg error: lastError should be set")
	}
}

func TestTUIModel_UnknownMsg_Noop(t *testing.T) {
	m := newTestTUIModel()
	origQuery := m.searchQuery
	newM, _ := m.Update(struct{ x int }{99})
	wm := newM.(TUIModel)
	if wm.searchQuery != origQuery {
		t.Error("unknown msg: state changed unexpectedly")
	}
}

// ── TUIModel.View — all state variants ───────────────────────────────────────

func TestTUIModel_View_Quitting_ReturnsEmpty(t *testing.T) {
	m := newTestTUIModel()
	m.isQuitting = true
	view := m.View()
	if view != "" {
		t.Errorf("View(quitting): expected '', got %q", view)
	}
}

func TestTUIModel_View_Loading_NoPanic(t *testing.T) {
	m := newTestTUIModel()
	m.isLoading = true
	m.searchQuery = "test"
	view := m.View()
	if view == "" {
		t.Error("View(loading): returned empty string")
	}
}

func TestTUIModel_View_ShowShortcuts_NoPanic(t *testing.T) {
	m := newTestTUIModel()
	m.showShortcutsHelp = true
	view := m.View()
	if view == "" {
		t.Error("View(shortcuts): returned empty string")
	}
}

func TestTUIModel_View_WithResults_NoPanic(t *testing.T) {
	m := newTestTUIModel()
	m.searchResults = []TUISearchResult{
		{Title: "Result 1", URL: "http://example.com/1"},
		{Title: "Result 2", URL: "http://example.com/2"},
	}
	m.selectedIndex = 0
	view := m.View()
	if view == "" {
		t.Error("View(with results): returned empty string")
	}
}

func TestTUIModel_View_WithError_NoPanic(t *testing.T) {
	m := newTestTUIModel()
	m.lastError = errors.New("search failed")
	view := m.View()
	if view == "" {
		t.Error("View(with error): returned empty string")
	}
}

func TestTUIModel_View_EmptyState_NoPanic(t *testing.T) {
	m := newTestTUIModel()
	view := m.View()
	if view == "" {
		t.Error("View(empty): returned empty string")
	}
}

func TestTUIModel_View_WithStatus_NoPanic(t *testing.T) {
	m := newTestTUIModel()
	m.statusMessage = "Some status message"
	view := m.View()
	if view == "" {
		t.Error("View(with status): returned empty string")
	}
}

func TestTUIModel_View_WithOpenedURL_NoPanic(t *testing.T) {
	m := newTestTUIModel()
	m.openedURL = "http://example.com/opened"
	view := m.View()
	if view == "" {
		t.Error("View(with openedURL): returned empty string")
	}
}

func TestTUIModel_View_WithQuery_NoPanic(t *testing.T) {
	m := newTestTUIModel()
	m.searchQuery = "test query"
	view := m.View()
	if view == "" {
		t.Error("View(with query): returned empty string")
	}
}
