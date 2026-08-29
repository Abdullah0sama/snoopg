package ui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"snoopg/internal/db"
)

type connectForm struct {
	input textinput.Model
	save  bool
	err   string
	dsn   string
}

type connectResultMsg struct {
	dsn string
	err error
}

func RunConnect() (dsn string, save bool, err error) {
	form := &connectForm{}
	form.input = textinput.New()
	form.input.Placeholder = "postgres://user:pass@host:5432/db"
	form.input.Prompt = "dsn: "
	form.input.Focus()
	form.input.Width = 60
	p := tea.NewProgram(form, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return "", false, err
	}
	return form.dsn, form.save, nil
}

func connectCmd(dsn string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client, err := db.New(ctx, dsn)
		if err != nil {
			return connectResultMsg{dsn: dsn, err: err}
		}
		client.Close()
		return connectResultMsg{dsn: dsn}
	}
}

func (f *connectForm) Init() tea.Cmd {
	return nil
}

func (f *connectForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return f, tea.Quit
		case "tab":
			f.save = !f.save
			return f, nil
		case "enter":
			return f, connectCmd(f.input.Value())
		default:
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			return f, cmd
		}
	case connectResultMsg:
		if msg.err != nil {
			f.err = msg.err.Error()
			return f, nil
		}
		f.dsn = msg.dsn
		return f, tea.Quit
	}
	return f, nil
}

func (f *connectForm) View() string {
	title := Logo() + " " + HeaderStyle.Render("— no saved connection")
	fields := f.input.View()
	saveLine := HintStyle.Render("save: no")
	if f.save {
		saveLine = WatchStyle.Render("save: yes")
	}
	help := HintStyle.Render("enter: connect · tab: toggle save · esc: quit")
	out := title + "\n\n" + fields + "\n\n" + saveLine + "\n\n" + help
	if f.err != "" {
		out += "\n\n" + ErrorStyle.Render("error: "+f.err)
	}
	return out
}
