package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgconn"

	"snoopg/internal/config"
	"snoopg/internal/db"
	"snoopg/internal/ui"
)

type namedProfile struct {
	Name     string
	DSN      string
	ReadOnly bool
	TokenCmd string
	Label    string
}

type Connections struct {
	client       *db.Client
	profiles     []namedProfile
	dbs          []string
	currentLabel string
	sel          int
	form         bool
	formName     string
	formReadOnly bool
	input        textinput.Model
	err          string
}

func NewConnections(client *db.Client) *Connections {
	return &Connections{client: client, currentLabel: shortLabel(client.DSN())}
}

func shortLabel(dsn string) string {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil || cfg.Database == "" {
		return dsn
	}
	return fmt.Sprintf("%s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
}

func (m *Connections) Title() string { return "connections" }

func (m *Connections) WantsAllKeys() bool { return m.form }

func (m *Connections) Init() tea.Cmd { return fetchConnCmd(m.client) }

type fetchConnMsg struct {
	profiles []namedProfile
	dbs      []string
	err      error
}

type connTickMsg struct{}

type connectResultMsg struct {
	label string
	err   error
}

func fetchConnCmd(c *db.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cfg, cfgErr := config.Load()
		var profiles []namedProfile
		if cfgErr == nil {
		for _, name := range cfg.Names() {
			p := cfg.Profiles[name]
			profiles = append(profiles, namedProfile{
				Name:     name,
				DSN:      p.DSN,
				ReadOnly: p.ReadOnly,
				TokenCmd: p.TokenCmd,
				Label:    shortLabel(p.DSN),
			})
		}
		}
		dbs, dbsErr := c.ListDatabases(ctx)
		var err error
		if cfgErr != nil {
			err = cfgErr
		} else if dbsErr != nil {
			err = dbsErr
		}
		return fetchConnMsg{profiles: profiles, dbs: dbs, err: err}
	}
}

func connectProfileCmd(c *db.Client, dsn, tokenCmd string, readOnly bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if tokenCmd != "" {
			resolved, err := db.ResolvePassword(dsn, tokenCmd)
			if err != nil {
				return connectResultMsg{err: err}
			}
			dsn = resolved
		}
		if err := c.Reconnect(ctx, dsn, readOnly); err != nil {
			return connectResultMsg{err: err}
		}
		return connectResultMsg{label: shortLabel(c.DSN())}
	}
}

func switchDbCmd(c *db.Client, database string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := c.SwitchDatabase(ctx, database); err != nil {
			return connectResultMsg{err: err}
		}
		return connectResultMsg{label: shortLabel(c.DSN())}
	}
}

func (m *Connections) Update(msg tea.Msg) (ui.Module, tea.Cmd) {
	if m.form {
		return m.updateForm(msg)
	}
	switch msg := msg.(type) {
	case fetchConnMsg:
		m.profiles = msg.profiles
		m.dbs = msg.dbs
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
		}
		if m.sel > m.entryCount()-1 {
			m.sel = m.entryCount() - 1
		}
		if m.sel < 0 {
			m.sel = 0
		}
		return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return connTickMsg{} })
	case connTickMsg:
		return m, fetchConnCmd(m.client)
	case ui.RefreshMsg:
		return m, fetchConnCmd(m.client)
	case ui.ProfileChangedMsg:
		m.currentLabel = msg.Label
		return m, fetchConnCmd(m.client)
	case connectResultMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.currentLabel = msg.label
		m.err = ""
		return m, func() tea.Msg { return ui.ProfileChangedMsg{Label: msg.label} }
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.sel > 0 {
				m.sel--
			}
		case "down", "j":
			if m.sel < m.entryCount()-1 {
				m.sel++
			}
		case "enter":
			name, kind := m.entryAt(m.sel)
			if kind == "profile" {
				for _, p := range m.profiles {
					if p.Name == name {
						return m, connectProfileCmd(m.client, p.DSN, p.TokenCmd, p.ReadOnly)
					}
				}
			} else if kind == "db" {
				return m, switchDbCmd(m.client, name)
			}
		case "n":
			m.form = true
			m.formName = ""
			m.formReadOnly = false
			m.input = textinput.New()
			m.input.Placeholder = "postgres://user:pass@host:5432/db"
			m.input.Prompt = "dsn: "
			m.input.Width = 70
			m.input.Focus()
			return m, nil
		case "e":
			name, kind := m.entryAt(m.sel)
			if kind == "profile" {
				for _, p := range m.profiles {
					if p.Name == name {
						m.form = true
						m.formName = name
						m.formReadOnly = p.ReadOnly
						m.input = textinput.New()
						m.input.Prompt = "dsn: "
						m.input.Width = 70
						m.input.SetValue(p.DSN)
						m.input.CursorEnd()
						m.input.Focus()
						return m, nil
					}
				}
			}
		case "d":
			name, kind := m.entryAt(m.sel)
			if kind == "profile" {
				if err := config.Delete(name); err != nil {
					m.err = err.Error()
				}
				return m, fetchConnCmd(m.client)
			}
		case "m":
			name, kind := m.entryAt(m.sel)
			if kind == "profile" {
				for _, p := range m.profiles {
					if p.Name == name {
						if err := config.SetReadOnly(name, !p.ReadOnly); err != nil {
							m.err = err.Error()
						}
						return m, fetchConnCmd(m.client)
					}
				}
			}
		}
	}
	return m, nil
}

func (m *Connections) updateForm(msg tea.Msg) (ui.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.form = false
			return m, nil
		case "tab":
			m.formReadOnly = !m.formReadOnly
			return m, nil
		case "enter":
			dsn := strings.TrimSpace(m.input.Value())
			if dsn == "" {
				return m, nil
			}
			name := m.formName
			if name == "" {
				name = profileName(dsn)
			}
			if err := config.Save(name, dsn, m.formReadOnly); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.form = false
			m.err = ""
			return m, tea.Batch(connectProfileCmd(m.client, dsn, "", m.formReadOnly), fetchConnCmd(m.client))
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case connectResultMsg:
		m.err = ""
		return m, nil
	}
	return m, nil
}

func profileName(dsn string) string {
	cfg, err := pgconn.ParseConfig(dsn)
	if err == nil && cfg.Database != "" {
		return cfg.Database
	}
	return "default"
}

func (m *Connections) entryCount() int {
	return len(m.profiles) + len(m.dbs)
}

func (m *Connections) entryAt(i int) (name string, kind string) {
	if i < 0 || i >= m.entryCount() {
		return "", ""
	}
	if i < len(m.profiles) {
		return m.profiles[i].Name, "profile"
	}
	return m.dbs[i-len(m.profiles)], "db"
}

func (m *Connections) View(width, height int) string {
	if width <= 4 || height <= 2 {
		return ""
	}
	innerW := width - 4
	if innerW < 1 {
		innerW = 1
	}
	innerH := height - 2
	if innerH < 1 {
		innerH = 1
	}

	if m.form {
		return m.formView(width, height, innerW)
	}

	title := ui.PaneTitleStyle.Render("CONNECTIONS") + ui.HintStyle.Render(" · "+m.currentLabel)
	lines := []string{title, ui.HintStyle.Render(strings.Repeat("─", innerW))}
	lines = append(lines, ui.HintStyle.Render("profiles"))

	idx := 0
	for _, p := range m.profiles {
		mark := " "
		if p.Label == m.currentLabel {
			mark = "●"
		}
		ro := "RW"
		if p.ReadOnly {
			ro = "RO"
		}
		line := fmt.Sprintf(" %s %-14s %-28s %s", mark, p.Name, p.Label, ro)
		if idx == m.sel {
			line = ui.SelStyle.Width(innerW).Render(ui.FitWidth(line, innerW))
		} else {
			line = ui.FitWidth(line, innerW)
		}
		lines = append(lines, line)
		idx++
	}

	lines = append(lines, ui.HintStyle.Render("databases"))
	for _, d := range m.dbs {
		mark := " "
		if shortDBName(m.currentLabel) == d {
			mark = "●"
		}
		line := fmt.Sprintf(" %s %s", mark, d)
		if idx == m.sel {
			line = ui.SelStyle.Width(innerW).Render(ui.FitWidth(line, innerW))
		} else {
			line = ui.FitWidth(line, innerW)
		}
		lines = append(lines, line)
		idx++
	}

	lines = append(lines, ui.HintStyle.Render("enter: connect · n: new · e: edit · d: delete · m: read-only"))
	if m.err != "" {
		lines = append(lines, ui.ErrorStyle.Render(ui.FitWidth("error: "+m.err, innerW)))
	}

	return ui.PaneStyle.Width(width).Height(height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func shortDBName(label string) string {
	if i := strings.LastIndex(label, "/"); i >= 0 {
		return label[i+1:]
	}
	return label
}

func (m *Connections) formView(width, height, innerW int) string {
	title := ui.PaneTitleStyle.Render("CONNECTIONS")
	if m.formName != "" {
		title += ui.HintStyle.Render(" — edit " + m.formName)
	} else {
		title += ui.HintStyle.Render(" — new profile")
	}
	ro := "read-only: no"
	if m.formReadOnly {
		ro = ui.WarnStyle.Render("read-only: yes")
	}
	lines := []string{
		title,
		"",
		m.input.View(),
		"",
		ro,
		"",
		ui.HintStyle.Render("enter: save & connect · tab: toggle read-only · esc: cancel"),
	}
	if m.err != "" {
		lines = append(lines, ui.ErrorStyle.Render(ui.FitWidth("error: "+m.err, innerW)))
	}
	return ui.PaneStyle.Width(width).Height(height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
