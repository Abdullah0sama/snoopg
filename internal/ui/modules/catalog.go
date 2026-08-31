package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	tree "charm.land/bubbles/v2/tree"
	v2tea "charm.land/bubbletea/v2"
	v2lip "charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"snoopg/internal/db"
	"snoopg/internal/ui"
)

type Tables struct {
	client  *db.Client
	dbName  string
	tables  []db.TableInfo
	err     string
	tree    tree.Model
	lastSig string
	treeW   int
	treeH   int
}

func NewTables(client *db.Client) *Tables {
	m := &Tables{client: client}
	m.tree = tree.New(tree.Root(""), 0, 0)
	m.tree.SetShowHelp(false)
	m.tree.SetStyles(catalogTreeStyles())
	return m
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
		sig := catalogSig(msg.tables)
		if sig != m.lastSig {
			m.tree = newCatalogTree(msg.tables, m.treeW, m.treeH)
			m.lastSig = sig
		}
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return catalogTickMsg{} })
	case catalogTickMsg:
		return m, fetchCatalogCmd(m.client)
	case ui.RefreshMsg:
		return m, fetchCatalogCmd(m.client)
	case tea.KeyMsg:
		nt, cmd := m.tree.Update(bridgeKeyMsg(msg))
		m.tree = nt
		if cmd != nil {
			return m, func() tea.Msg { return cmd() }
		}
		return m, nil
	}
	return m, nil
}

func bridgeKeyMsg(msg tea.KeyMsg) v2tea.KeyPressMsg {
	k := v2tea.KeyPressMsg{}
	switch msg.Type {
	case tea.KeyRunes:
		k.Text = string(msg.Runes)
		if len(msg.Runes) > 0 {
			k.Code = msg.Runes[0]
		}
	case tea.KeyUp:
		k.Code = v2tea.KeyUp
	case tea.KeyDown:
		k.Code = v2tea.KeyDown
	case tea.KeyLeft:
		k.Code = v2tea.KeyLeft
	case tea.KeyRight:
		k.Code = v2tea.KeyRight
	case tea.KeyEnter:
		k.Code = v2tea.KeyEnter
	case tea.KeySpace:
		k.Code = v2tea.KeySpace
	case tea.KeyTab:
		k.Code = v2tea.KeyTab
	case tea.KeyEsc:
		k.Code = v2tea.KeyEscape
	case tea.KeyBackspace:
		k.Code = v2tea.KeyBackspace
	case tea.KeyDelete:
		k.Code = v2tea.KeyDelete
	case tea.KeyHome:
		k.Code = v2tea.KeyHome
	case tea.KeyEnd:
		k.Code = v2tea.KeyEnd
	case tea.KeyPgUp:
		k.Code = v2tea.KeyPgUp
	case tea.KeyPgDown:
		k.Code = v2tea.KeyPgDown
	case tea.KeyShiftTab:
		k.Code = v2tea.KeyTab
		k.Mod = v2tea.ModShift
	default:
		switch {
		case msg.Type >= tea.KeyCtrlA && msg.Type <= tea.KeyCtrlZ:
			k.Code = 'a' + rune(msg.Type-tea.KeyCtrlA)
			k.Mod = v2tea.ModCtrl
		case msg.Type >= tea.KeyF1 && msg.Type <= tea.KeyF20:
			k.Code = v2tea.KeyF1 + rune(msg.Type-tea.KeyF1)
		}
	}
	return k
}

func catalogTreeStyles() tree.Styles {
	st := tree.DefaultDarkStyles()
	st.SelectedNodeStyle = v2lip.NewStyle().Background(v2lip.Color("208")).Foreground(v2lip.Color("0")).Bold(true)
	st.CursorStyle = v2lip.NewStyle().PaddingRight(1).Foreground(v2lip.Color("208")).Bold(true)
	st.RootNodeStyle = v2lip.NewStyle().Foreground(v2lip.Color("238"))
	st.OpenIndicatorStyle = v2lip.NewStyle().Foreground(v2lip.Color("241"))
	return st
}

func newCatalogTree(tables []db.TableInfo, w, h int) tree.Model {
	root := tree.Root("")
	for _, t := range tables {
		tn := tree.Root(fmt.Sprintf("%s.%s  %s", t.Schema, t.Name, prettyBytes(t.SizeBytes)))
		var kids []any
		for _, ix := range t.Indexes {
			marker := "    "
			if ix.Primary {
				marker = "[PK]"
			} else if ix.Unique {
				marker = "[U]"
			}
			kids = append(kids, fmt.Sprintf("%s %s (%s)  %s", marker, ix.Name, ix.Columns, prettyBytes(ix.SizeBytes)))
		}
		for _, r := range t.Refs {
			kids = append(kids, "→ "+r)
		}
		for _, r := range t.RefBy {
			kids = append(kids, "← "+r)
		}
		tn.Child(kids...)
		root.Child(tn)
	}
	m := tree.New(root, w, h)
	m.SetShowHelp(false)
	m.SetStyles(catalogTreeStyles())
	return m
}

func catalogSig(tables []db.TableInfo) string {
	var b strings.Builder
	for _, t := range tables {
		b.WriteString(t.Schema)
		b.WriteByte('.')
		b.WriteString(t.Name)
		b.WriteByte(';')
		for _, ix := range t.Indexes {
			b.WriteString(ix.Name)
			b.WriteByte(',')
		}
		b.WriteByte(';')
		for _, r := range t.Refs {
			b.WriteString(r)
			b.WriteByte(',')
		}
		b.WriteByte(';')
		for _, r := range t.RefBy {
			b.WriteString(r)
			b.WriteByte(',')
		}
		b.WriteByte('|')
	}
	return b.String()
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

	treeH := innerH - 1
	if treeH < 1 {
		treeH = 1
	}
	if m.treeW != innerW || m.treeH != treeH {
		m.tree.SetSize(innerW, treeH)
		m.treeW = innerW
		m.treeH = treeH
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

	limit := innerH - 1
	if m.err != "" {
		limit--
	}
	if limit < 0 {
		limit = 0
	}
	treeLines := strings.Split(m.tree.View(), "\n")
	for i := range treeLines {
		treeLines[i] = ui.FitWidth(treeLines[i], innerW)
	}
	if len(treeLines) > limit {
		treeLines = treeLines[:limit]
	}
	for len(treeLines) < limit {
		treeLines = append(treeLines, "")
	}
	lines = append(lines, treeLines...)

	if m.err != "" {
		lines = append(lines, ui.ErrorStyle.Render(ui.FitWidth("error: "+m.err, innerW)))
	}

	lines = append(lines, ui.HintStyle.Render(ui.FitWidth("j/k: move · enter/→: expand · ←: collapse", innerW)))

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
