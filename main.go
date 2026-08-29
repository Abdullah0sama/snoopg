package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"snoopg/internal/db"
	"snoopg/internal/ui"
	"snoopg/internal/ui/modules"

	tea "github.com/charmbracelet/bubbletea"
)

const defaultDSN = "postgres://postgres:postgres@localhost:5432/automation_db?sslmode=disable&application_name=snoopg"

func main() {
	dsn := os.Getenv("SNOOPG_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := db.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snoopg: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	mods := []ui.Module{
		modules.NewBufferCache(client),
		modules.NewTables(client),
	}
	p := tea.NewProgram(ui.NewApp(client, mods), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "snoopg: %v\n", err)
		os.Exit(1)
	}
}
