package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type app struct {
	ready bool
}

func NewApp() tea.Model {
	return &app{}
}

func (a *app) Init() tea.Cmd {
	return nil
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.ready = true
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a *app) View() string {
	if !a.ready {
		return "loading..."
	}
	title := titleStyle.Render("pgspy — postgres heap page inspector")
	hint := hintStyle.Render("q: quit")
	return fmt.Sprintf("%s\n\n%s", title, hint)
}
