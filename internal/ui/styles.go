package ui

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	LogoStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	LogoBlockStyle = lipgloss.NewStyle().Bold(true).Italic(true).Foreground(lipgloss.Color("208"))
	TabStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("241")).Padding(0, 1)
	TabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("208")).Padding(0, 1)
	HintStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	PromptStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	PaneStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	PaneTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	ErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	WatchStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
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
