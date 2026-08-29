package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"pgspy/internal/db"
	"pgspy/internal/ui"
	"pgspy/internal/ui/modules"

	tea "github.com/charmbracelet/bubbletea"
)

const defaultDSN = "postgres://postgres:postgres@localhost:5432/automation_db?sslmode=disable"

func main() {
	dsn := os.Getenv("PGSPY_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := db.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgspy: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	mods := []ui.Module{
		modules.NewBufferCache(client),
	}
	p := tea.NewProgram(ui.NewApp(client, mods), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "pgspy: %v\n", err)
		os.Exit(1)
	}
}
