package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"snoopg/internal/db"
)

type RefreshMsg struct{}

type ProfileChangedMsg struct {
	Label string
}

type ExplainResultMsg struct {
	Query    string
	Plan     *db.ExplainPlan
	Elapsed  time.Duration
	Analyzed bool
	Err      error
}

type Module interface {
	Title() string
	Init() tea.Cmd
	Update(msg tea.Msg) (Module, tea.Cmd)
	View(width, height int) string
}

type KeySink interface {
	WantsAllKeys() bool
}
