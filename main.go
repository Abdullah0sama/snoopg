package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jackc/pgx/v5/pgconn"

	"snoopg/internal/config"
	"snoopg/internal/db"
	"snoopg/internal/ui"
	"snoopg/internal/ui/modules"
)

func resolveDSN(dsnFlag string) string {
	if dsn := os.Getenv("SNOOPG_DSN"); dsn != "" {
		return dsn
	}
	if dsnFlag != "" {
		return dsnFlag
	}
	if cfg, err := config.Load(); err == nil {
		if dsn := cfg.Profiles[cfg.Last]; dsn != "" {
			return dsn
		}
	}
	return ""
}

func profileName(dsn string) string {
	if cfg, err := pgconn.ParseConfig(dsn); err == nil && cfg.Database != "" {
		return cfg.Database
	}
	return "default"
}

func main() {
	dsnFlag := flag.String("dsn", "", "PostgreSQL connection string")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dsn := resolveDSN(*dsnFlag)
	if dsn == "" {
		var err error
		var save bool
		dsn, save, err = ui.RunConnect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "snoopg: %v\n", err)
			os.Exit(1)
		}
		if dsn == "" {
			return
		}
		if save {
			name := profileName(dsn)
			if err := config.Save(name, dsn); err != nil {
				fmt.Fprintf(os.Stderr, "snoopg: could not save connection: %v\n", err)
			} else {
				fmt.Printf("saved connection %q — next launch uses it automatically\n", name)
			}
		}
	}

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
