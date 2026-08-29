package main

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"snoopg/internal/db"
	"snoopg/internal/ui"
	"snoopg/internal/ui/modules"
)

const e2eLabDSN = "postgres://postgres:postgres@localhost:5433/snoopg_lab?sslmode=disable"

func runOneCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case m := <-done:
		return m
	case <-time.After(15 * time.Second):
		t.Fatal("command timed out")
		return nil
	}
}

func runFast(cmd tea.Cmd) (tea.Msg, bool) {
	if cmd == nil {
		return nil, false
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case m := <-done:
		return m, true
	case <-time.After(300 * time.Millisecond):
		return nil, false
	}
}

func pump(t *testing.T, app tea.Model, msg tea.Msg) tea.Model {
	t.Helper()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if res, ok := runFast(c); ok && res != nil {
				app = pump(t, app, res)
			}
		}
		return app
	}
	var cmd tea.Cmd
	app, cmd = app.Update(msg)
	if cmd != nil {
		if res, ok := runFast(cmd); ok && res != nil {
			app = pump(t, app, res)
		}
	}
	return app
}

func TestConnectionsLiveDatabaseSwitch(t *testing.T) {
	client, err := db.New(context.Background(), e2eLabDSN)
	if err != nil {
		t.Skipf("skipping: lab postgres unreachable: %v", err)
	}
	defer client.Close()

	app := ui.NewApp(client, []ui.Module{modules.NewConnections(client)})

	if m := runOneCmd(t, app.Init()); m != nil {
		app = pump(t, app, m)
	}

	app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	for i := 0; i < 5; i++ {
		app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	}
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	var cmd tea.Cmd
	app, cmd = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on database entry produced no command")
	}
	app = pump(t, app, runOneCmd(t, cmd))

	view := app.View()
	if !strings.Contains(view, "localhost:5433/postgres") {
		t.Fatalf("view does not show switched database, header got:\n%s", view[:min(len(view), 200)])
	}
}
