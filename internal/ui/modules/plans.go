package modules

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"snoopg/internal/db"
	"snoopg/internal/ui"
)

type Plans struct {
	client   *db.Client
	plan     *db.ExplainPlan
	query    string
	analyzed bool
	err      string
	offset   int
}

func NewPlans(client *db.Client) *Plans {
	return &Plans{client: client}
}

func (m *Plans) Title() string { return "plans" }

func (m *Plans) Init() tea.Cmd { return nil }

func (m *Plans) Update(msg tea.Msg) (ui.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.ExplainResultMsg:
		m.plan = msg.Plan
		m.query = msg.Query
		m.analyzed = msg.Analyzed
		if msg.Err != nil {
			m.err = msg.Err.Error()
		} else {
			m.err = ""
		}
		m.offset = 0
		return m, nil
	case ui.RefreshMsg:
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.offset > 0 {
				m.offset--
			}
		case "down", "j":
			m.offset++
		}
	}
	return m, nil
}

func (m *Plans) View(width, height int) string {
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

	title := ui.PaneTitleStyle.Render("PLANS")
	if m.query != "" {
		title += ui.HintStyle.Render(" — " + ui.FitWidth(m.query, 60))
	}
	if m.plan != nil && !m.analyzed {
		title += ui.WarnStyle.Render(" (estimated)")
	}
	lines := []string{title}

	if m.err != "" {
		lines = append(lines, ui.ErrorStyle.Render(ui.FitWidth("error: "+m.err, innerW)))
		return ui.PaneStyle.Width(width).Height(height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	}
	if m.plan == nil {
		lines = append(lines, ui.HintStyle.Render("run a query in explain mode (ctrl+e) to see its plan"))
		return ui.PaneStyle.Width(width).Height(height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	}

	rootCost := m.plan.Plan.TotalCost
	var allLines []string
	allLines = append(allLines, renderNode(m.plan.Plan, "", true, true, rootCost, m.analyzed)...)
	allLines = append(allLines, ui.HintStyle.Render("j/k: scroll"))

	visible := innerH - 1
	if visible < 1 {
		visible = 1
	}
	maxOffset := len(allLines) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
	for i := m.offset; i < m.offset+visible && i < len(allLines); i++ {
		lines = append(lines, ui.FitWidth(allLines[i], innerW))
	}

	return ui.PaneStyle.Width(width).Height(height - 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func renderNode(n db.PlanNode, prefix string, isLast bool, isRoot bool, rootCost float64, analyzed bool) []string {
	line := n.NodeType
	if n.RelationName != "" {
		line += " on " + n.RelationName
	}
	line += fmt.Sprintf(" (cost=%.2f..%.2f rows=%.0f width=%.0f)", n.StartupCost, n.TotalCost, n.PlanRows, n.PlanWidth)
	if analyzed && n.ActualTotalTime > 0 {
		line += fmt.Sprintf(" (actual time=%.2f..%.2f rows=%.0f loops=%.0f)", n.ActualStartupTime, n.ActualTotalTime, n.ActualRows, n.ActualLoops)
	}

	share := 0.0
	if rootCost > 0 {
		share = n.TotalCost / rootCost
	}
	if isRoot {
		line = ui.PaneTitleStyle.Render(line)
	} else if share > 0.5 {
		line = ui.WarnStyle.Render(line)
	} else if share > 0.25 {
		line = ui.PaneTitleStyle.Render(line)
	}

	out := []string{prefix + line}
	for i, child := range n.Plans {
		last := i == len(n.Plans)-1
		childPrefix := prefix + "   "
		if !isLast {
			childPrefix = prefix + "│  "
		}
		connector := "├─ "
		if last {
			connector = "└─ "
		}
		children := renderNode(child, childPrefix+connector, last, false, rootCost, analyzed)
		out = append(out, children...)
	}
	return out
}
