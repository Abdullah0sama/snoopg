package main

import (
	"fmt"
	"os"

	"pgspy/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(ui.NewApp(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "pgspy: %v\n", err)
		os.Exit(1)
	}
}
