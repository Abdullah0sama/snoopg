package ui

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	TabStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("241")).Padding(0, 1)
	TabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("205")).Padding(0, 1)
	HintStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	PromptStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	PaneStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	PaneTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	ErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	WatchStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
)

func FitWidth(s string, w int) string {
	if w <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= w {
		return s
	}
	return string(runes[:w])
}
