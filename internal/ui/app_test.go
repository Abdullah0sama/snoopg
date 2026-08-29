package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"snoopg/internal/db"
)

const testLabDSN = "postgres://postgres:postgres@localhost:5433/snoopg_lab?sslmode=disable"

type stubModule struct {
	title string
	msgs  []tea.Msg
	sink  bool
}

func (s *stubModule) Title() string { return s.title }
func (s *stubModule) Init() tea.Cmd { return nil }
func (s *stubModule) Update(msg tea.Msg) (Module, tea.Cmd) {
	s.msgs = append(s.msgs, msg)
	return s, nil
}
func (s *stubModule) View(width, height int) string { return "" }
func (s *stubModule) WantsAllKeys() bool            { return s.sink }

func mustApp(t *testing.T, readOnly bool) *app {
	t.Helper()
	client, err := db.NewWithMode(context.Background(), testLabDSN, readOnly)
	if err != nil {
		t.Skipf("skipping: lab postgres unreachable: %v", err)
	}
	t.Cleanup(client.Close)
	mod := &stubModule{title: "stub"}
	a := NewApp(client, []Module{mod}).(*app)
	a.width, a.height = 120, 40
	return a
}

func key(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func TestProfileChangedMsgUpdatesLabel(t *testing.T) {
	a := mustApp(t, false)
	_, _ = a.Update(ProfileChangedMsg{Label: "example.com:5432/app"})
	if a.connLabel != "example.com:5432/app" {
		t.Fatalf("connLabel = %q", a.connLabel)
	}
	if !strings.Contains(a.View(), "example.com:5432/app") {
		t.Error("header does not show new connection label")
	}
}

func TestViewShowsBadge(t *testing.T) {
	rw := mustApp(t, false)
	if !strings.Contains(rw.View(), "[RW]") {
		t.Error("view missing [RW] badge")
	}
	ro := mustApp(t, true)
	if !strings.Contains(ro.View(), "[RO]") {
		t.Error("view missing [RO] badge")
	}
}

func TestKeySinkReceivesAllKeys(t *testing.T) {
	a := mustApp(t, false)
	sink := &stubModule{title: "sink", sink: true}
	a.mods = []Module{sink}
	a.active = 0
	a.input.Blur()

	_, _ = a.Update(key("x"))
	_, _ = a.Update(key("5"))
	_, _ = a.Update(key("tab"))

	if len(sink.msgs) != 3 {
		t.Fatalf("sink module got %d msgs, want 3", len(sink.msgs))
	}
	for _, m := range sink.msgs {
		if _, ok := m.(tea.KeyMsg); !ok {
			t.Fatalf("sink got non-KeyMsg: %#v", m)
		}
	}
}

func TestKeySinkNotReceivingWhileTyping(t *testing.T) {
	a := mustApp(t, false)
	sink := &stubModule{title: "sink", sink: true}
	a.mods = []Module{sink}
	a.active = 0
	a.input.Focus()

	_, _ = a.Update(key("x"))
	if len(sink.msgs) != 0 {
		t.Fatal("module got keys while query bar focused")
	}
	if !strings.Contains(a.input.Value(), "x") {
		t.Fatal("keystroke did not reach the query bar")
	}
}

func TestWatchSelectExplainIssuesCmd(t *testing.T) {
	a := mustApp(t, false)
	a.input.Blur()
	a.extActs = []db.Activity{{PID: 1, Database: "snoopg_lab", Query: "SELECT 1"}}
	a.watchSelect = true
	a.watchSel = 0

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("no explain command issued for selected watched query")
	}
	if a.watchSelect {
		t.Fatal("watch selection should exit after explaining")
	}
}

func TestExplainResultRoutesToPlansModule(t *testing.T) {
	a := mustApp(t, false)
	plans := &stubModule{title: "plans"}
	a.mods = []Module{&stubModule{title: "stub"}, plans}
	a.active = 0

	_, _ = a.Update(ExplainResultMsg{Query: "SELECT 1"})
	if a.active != 1 {
		t.Fatalf("active module = %d, want plans at index 1", a.active)
	}
	if len(plans.msgs) != 1 {
		t.Fatal("plans module did not receive the explain result")
	}
}
