package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"snoopg/internal/db"
)

func TestSuggestionsRender(t *testing.T) {
	// A zero-value client is enough: this test never touches the database.
	a := NewApp(&db.Client{}, []Module{&stubModule{title: "stub"}}).(*app)
	a.width, a.height = 170, 44

	for i, k := range "sel" {
		_, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}, Alt: false})
		if i >= 1 && len(a.input.MatchedSuggestions()) == 0 {
			t.Fatalf("keystroke %d (%q): matched suggestions empty", i, string(k))
		}
	}
	view := a.View()
	lines := strings.Split(view, "\n")
	if !strings.Contains(lines[len(lines)-1], "ECT") {
		t.Errorf("ghost completion not rendered; last line %q", lines[len(lines)-1])
	}
}

func TestViewFitsTerminal(t *testing.T) {
	a := NewApp(&db.Client{}, []Module{&stubModule{title: "stub"}}).(*app)
	a.width, a.height = 170, 44

	view := a.View()
	if n := strings.Count(view, "\n") + 1; n != 43 {
		t.Fatalf("view renders %d lines, want 43 (height-1)", n)
	}
	if !strings.HasPrefix(view, "snoopg") {
		t.Fatalf("header missing from first line: %q", strings.Split(view, "\n")[0])
	}
}

func TestModuleTicksContinueWhileResultsOpen(t *testing.T) {
	a := NewApp(&db.Client{}, []Module{&stubModule{title: "stub"}}).(*app)
	mod := a.mods[0].(*stubModule)
	a.showResults = true

	// A module tick message arriving while the results modal is open must
	// still reach the active module, or its poll loop dies silently.
	_, _ = a.Update(execResultMsg{})
	_, _ = a.Update(struct{}{})
	if len(mod.msgs) == 0 {
		t.Fatal("module did not receive messages while results modal was open")
	}
}
