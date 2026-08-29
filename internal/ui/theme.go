package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Brand   lipgloss.Color
	OnBrand lipgloss.Color
	Accent  lipgloss.Color
	Warn    lipgloss.Color
	Error   lipgloss.Color
	Muted   lipgloss.Color
	Faint   lipgloss.Color
}

var DefaultTheme = Theme{
	Brand:   "208",
	OnBrand: "0",
	Accent:  "39",
	Warn:    "214",
	Error:   "196",
	Muted:   "241",
	Faint:   "238",
}

var (
	HeaderStyle    lipgloss.Style
	LogoStyle      lipgloss.Style
	LogoBlockStyle lipgloss.Style
	TabStyle       lipgloss.Style
	TabActiveStyle lipgloss.Style
	HintStyle      lipgloss.Style
	PromptStyle    lipgloss.Style
	PaneStyle      lipgloss.Style
	PaneTitleStyle lipgloss.Style
	ErrorStyle     lipgloss.Style
	WatchStyle     lipgloss.Style
	BarFillStyle   lipgloss.Style
	BarDirtyStyle  lipgloss.Style
	BarEmptyStyle  lipgloss.Style
	WarnStyle      lipgloss.Style
	SelStyle       lipgloss.Style
)

func init() {
	SetTheme(DefaultTheme)
}

func SetTheme(t Theme) {
	HeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Brand)
	LogoStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Brand)
	LogoBlockStyle = lipgloss.NewStyle().Bold(true).Italic(true).Foreground(t.Brand)
	TabStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Muted).Padding(0, 1)
	TabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(t.OnBrand).Background(t.Brand).Padding(0, 1)
	HintStyle = lipgloss.NewStyle().Foreground(t.Muted)
	PromptStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Brand)
	PaneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Faint).Padding(0, 1)
	PaneTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	ErrorStyle = lipgloss.NewStyle().Foreground(t.Error)
	WatchStyle = lipgloss.NewStyle().Foreground(t.Accent)
	BarFillStyle = lipgloss.NewStyle().Foreground(t.Accent)
	BarDirtyStyle = lipgloss.NewStyle().Foreground(t.Warn)
	BarEmptyStyle = lipgloss.NewStyle().Foreground(t.Faint)
	WarnStyle = lipgloss.NewStyle().Foreground(t.Warn)
	SelStyle = lipgloss.NewStyle().Background(t.Brand).Foreground(t.OnBrand).Bold(true)
}

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
