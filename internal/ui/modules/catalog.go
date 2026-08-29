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

var (
	selStyle    = lipgloss.NewStyle().Background(lipgloss.Color("209")).Foreground(lipgloss.Color("16")).Bold(true)
	markerStyle = lipgloss.NewStyle().Bold(true)
)

type Tables struct {
	client *db.Client
	dbName string
	tables []db.TableInfo
	sel    int
	err    string
}

func NewTables(client *db.Client) *Tables {
	return &Tables{client: client}
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
		if m.sel > len(m.tables)-1 {
			m.sel = len(m.tables) - 1
		}
		if m.sel < 0 {
			m.sel = 0
		}
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return catalogTickMsg{} })
	case catalogTickMsg:
		return m, fetchCatalogCmd(m.client)
	case ui.RefreshMsg:
		return m, fetchCatalogCmd(m.client)
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.sel > 0 {
				m.sel--
			}
			return m, nil
		case "down", "j":
			if m.sel < len(m.tables)-1 {
				m.sel++
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
	if visible < 0 {
		visible = 0
	}

	if innerW >= 24 && visible > 0 {
		leftW := innerW * 35 / 100
		if leftW < 20 {
			leftW = 20
		}
		rightW := innerW - leftW - 3
		if rightW < 1 {
			rightW = 1
		}

		offset := 0
		if m.sel >= visible+offset {
			offset = m.sel - visible + 1
		}
		if m.sel < offset {
			offset = m.sel
		}
		maxOffset := len(m.tables) - visible
		if maxOffset < 0 {
			maxOffset = 0
		}
		if offset > maxOffset {
			offset = maxOffset
		}
		if offset < 0 {
			offset = 0
		}

		leftLines := make([]string, 0, visible)
		for i := 0; i < visible; i++ {
			idx := offset + i
			if idx >= len(m.tables) {
				leftLines = append(leftLines, "")
				continue
			}
			t := m.tables[idx]
			name := ui.FitWidth(t.Schema+"."+t.Name, leftW)
			if idx == m.sel {
				name = selStyle.Width(leftW).Render(name)
			}
			leftLines = append(leftLines, name)
		}

		rightLines := []string{}
		if len(m.tables) > 0 && m.sel < len(m.tables) {
			t := m.tables[m.sel]
			rightLines = append(rightLines, ui.FitWidth(t.Schema+"."+t.Name, rightW))
			rightLines = append(rightLines, ui.HintStyle.Render(ui.FitWidth(prettyBytes(t.SizeBytes), rightW)))
			for _, ix := range t.Indexes {
				prefix := ""
				if ix.Primary {
					prefix = markerStyle.Render("[PK] ")
				} else if ix.Unique {
					prefix = markerStyle.Render("[U] ")
				}
				cols := ""
				if ix.Columns != "" {
					cols = " (" + ix.Columns + ")"
				}
				line := prefix + ix.Name + cols + " " + prettyBytes(ix.SizeBytes)
				rightLines = append(rightLines, ui.FitWidth(line, rightW))
			}
			rightLines = append(rightLines, "")
			rightLines = append(rightLines, ui.PaneTitleStyle.Render("FOREIGN KEYS"))
			if len(t.Refs) == 0 && len(t.RefBy) == 0 {
				rightLines = append(rightLines, ui.HintStyle.Render("none"))
			}
			for _, r := range t.Refs {
				rightLines = append(rightLines, ui.FitWidth("→ "+r, rightW))
			}
			for _, r := range t.RefBy {
				rightLines = append(rightLines, ui.FitWidth("← "+r, rightW))
			}
		} else {
			rightLines = append(rightLines, ui.HintStyle.Render("no tables"))
		}

		for len(leftLines) < visible {
			leftLines = append(leftLines, "")
		}
		if len(rightLines) > visible {
			rightLines = rightLines[:visible]
		}
		for len(rightLines) < visible {
			rightLines = append(rightLines, "")
		}

		body := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, leftLines...),
			" │ ",
			lipgloss.JoinVertical(lipgloss.Left, rightLines...))
		lines = append(lines, body)
	}

	if m.err != "" {
		lines = append(lines, ui.ErrorStyle.Render(ui.FitWidth("error: "+m.err, innerW)))
	}

	return ui.PaneStyle.Width(width).Height(height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
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
