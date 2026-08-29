package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type RefreshMsg struct{}

type ProfileChangedMsg struct {
	Label string
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
