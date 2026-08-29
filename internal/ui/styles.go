package ui

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("209"))
	LogoStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("209"))
	LogoBlockStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("209")).Padding(0, 1)
	TabStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245")).Padding(0, 1)
	TabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("209")).Padding(0, 1)
	HintStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	PromptStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("209"))
	PaneStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	PaneTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	ErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	WatchStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
)

func Logo() string {
	return LogoStyle.Render("snoo") + LogoBlockStyle.Render("pg")
}

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
