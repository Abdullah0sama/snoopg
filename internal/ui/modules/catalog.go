package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"snoopg/internal/db"
	"snoopg/internal/ui"
)

type line struct {
	text string
	root bool
	tbl  int
}

type Tables struct {
	client   *db.Client
	dbName   string
	tables   []db.TableInfo
	err      string
	expanded map[string]bool
	sel      int
	offset   int
}

func NewTables(client *db.Client) *Tables {
	return &Tables{client: client, expanded: map[string]bool{}}
}

func (m *Tables) Title() string { return "tables" }

func (m *Tables) Init() tea.Cmd { return fetchCatalogCmd(m.client) }

type fetchMsg struct {
	dbName string
	tables []db.TableInfo
	err    error
}

type catalogTickMsg struct{}

func fetchCatalogCmd(c *db.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tables, err := c.Catalog(ctx)
		var dbName string
		if err == nil {
			dbName, err = c.CurrentDatabase(ctx)
		}
		return fetchMsg{dbName: dbName, tables: tables, err: err}
	}
}

func (m *Tables) buildLines() []line {
	var lines []line
	for i, t := range m.tables {
		key := t.Schema + "." + t.Name
		arrow := "▸"
		if m.expanded[key] {
			arrow = "▾"
		}
		lines = append(lines, line{
			text: fmt.Sprintf("%s %s.%s  %s", arrow, t.Schema, t.Name, prettyBytes(t.SizeBytes)),
			root: true,
			tbl:  i,
		})
		if !m.expanded[key] {
			continue
		}
		for _, ix := range t.Indexes {
			marker := "    "
			if ix.Primary {
				marker = "[PK]"
			} else if ix.Unique {
				marker = "[U]"
			}
			lines = append(lines, line{
				text: fmt.Sprintf("    %s %s (%s)  %s", marker, ix.Name, ix.Columns, prettyBytes(ix.SizeBytes)),
			})
		}
		for _, r := range t.Refs {
			lines = append(lines, line{text: "    → " + r})
		}
		for _, r := range t.RefBy {
			lines = append(lines, line{text: "    ← " + r})
		}
	}
	return lines
}

func (m *Tables) clampSel(lines int) {
	if m.sel > lines-1 {
		m.sel = lines - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

func (m *Tables) Update(msg tea.Msg) (ui.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchMsg:
		m.dbName = msg.dbName
		m.tables = msg.tables
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
		}
		m.clampSel(len(m.buildLines()))
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return catalogTickMsg{} })
	case catalogTickMsg:
		return m, fetchCatalogCmd(m.client)
	case ui.RefreshMsg:
		return m, fetchCatalogCmd(m.client)
	case tea.KeyMsg:
		lines := m.buildLines()
		switch msg.String() {
		case "up", "k":
			if m.sel > 0 {
				m.sel--
			}
			return m, nil
		case "down", "j":
			if m.sel < len(lines)-1 {
				m.sel++
			}
			return m, nil
		case "enter", "right":
			if m.sel >= 0 && m.sel < len(lines) && lines[m.sel].root {
				t := m.tables[lines[m.sel].tbl]
				key := t.Schema + "." + t.Name
				m.expanded[key] = true
			}
			return m, nil
		case "left":
			if m.sel >= 0 && m.sel < len(lines) && lines[m.sel].root {
				t := m.tables[lines[m.sel].tbl]
				key := t.Schema + "." + t.Name
				delete(m.expanded, key)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Tables) View(width, height int) string {
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

	title := ui.PaneTitleStyle.Render("TABLES")
	dbTag := ""
	if m.dbName != "" {
		dbTag = ui.WatchStyle.Render(" " + m.dbName)
	}
	count := ui.HintStyle.Render(fmt.Sprintf("%d tables", len(m.tables)))
	spacer := innerW - lipgloss.Width(title) - lipgloss.Width(dbTag) - lipgloss.Width(count)
	if spacer < 0 {
		spacer = 0
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, dbTag, strings.Repeat(" ", spacer), count)

	lines := []string{header}

	visible := innerH - 1
	if m.err != "" {
		visible--
	}
	all := m.buildLines()
	m.clampSel(len(all))
	if m.sel >= m.offset+visible {
		m.offset = m.sel - visible + 1
	}
	if m.sel < m.offset {
		m.offset = m.sel
	}
	maxOff := len(all) - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if m.offset > maxOff {
		m.offset = maxOff
	}
	if m.offset < 0 {
		m.offset = 0
	}

	rows := 0
	for i := m.offset; i < len(all) && rows < visible; i++ {
		text := ui.FitWidth(all[i].text, innerW)
		if i == m.sel {
			text = ui.SelStyle.Width(innerW).Render(padTo(text, innerW))
		} else if !all[i].root {
			text = ui.HintStyle.Render(padTo(text, innerW))
		}
		lines = append(lines, text)
		rows++
	}
	for rows < visible {
		lines = append(lines, "")
		rows++
	}

	if m.err != "" {
		lines = append(lines, ui.ErrorStyle.Render(ui.FitWidth("error: "+m.err, innerW)))
	}
	lines = append(lines, ui.HintStyle.Render(ui.FitWidth("j/k: move · enter/→: expand · ←: collapse", innerW)))

	return ui.PaneStyle.Width(width).Height(height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func padTo(s string, w int) string {
	r := []rune(s)
	if len(r) > w {
		r = r[:w]
	}
	return string(r) + strings.Repeat(" ", w-len(r))
}

func prettyBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div := int64(unit)
	exp := 0
	for n >= div*unit && exp < 2 {
		div *= unit
		exp++
	}
	switch exp {
	case 0:
		return fmt.Sprintf("%dkB", n/div)
	case 1:
		return fmt.Sprintf("%dMB", n/div)
	default:
		return fmt.Sprintf("%.1fGB", float64(n)/float64(div))
	}
}
