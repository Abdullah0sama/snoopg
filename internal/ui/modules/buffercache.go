package modules

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pgspy/internal/db"
	"pgspy/internal/ui"
)

var (
	barFillStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	barDirtyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	barEmptyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	orangeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
)

type BufferCache struct {
	client    *db.Client
	stats     db.CacheStats
	relations []db.RelationStat
	err       string
}

func NewBufferCache(client *db.Client) *BufferCache {
	return &BufferCache{client: client}
}

func (m *BufferCache) Title() string { return "buffer cache" }

func (m *BufferCache) Init() tea.Cmd { return fetchStatsCmd(m.client) }

type statsMsg struct {
	stats     db.CacheStats
	relations []db.RelationStat
	err       error
}

type tickMsg struct{}

func fetchStatsCmd(c *db.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stats, sErr := c.CacheStats(ctx)
		relations, rErr := c.RelationStats(ctx)
		var err error
		if sErr != nil {
			err = sErr
		} else if rErr != nil {
			err = rErr
		}
		return statsMsg{stats: stats, relations: relations, err: err}
	}
}

func (m *BufferCache) Update(msg tea.Msg) (ui.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case statsMsg:
		m.stats = msg.stats
		m.relations = msg.relations
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
		}
		return m, tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
	case tickMsg:
		return m, fetchStatsCmd(m.client)
	case ui.RefreshMsg:
		return m, fetchStatsCmd(m.client)
	}
	return m, nil
}

func (m *BufferCache) View(width, height int) string {
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

	summary := ui.HintStyle.Render(fmt.Sprintf("%d pages · dirty %d · pinned %d · avg usage %.1f",
		m.stats.Total, m.stats.Dirty, m.stats.Pinned, m.stats.AvgUsage))

	lines := []string{ui.PaneTitleStyle.Render("BUFFER CACHE"), summary}
	rows := innerH - 2
	if rows > 18 {
		rows = 18
	}
	if rows < 0 {
		rows = 0
	}

	maxBuf := 0
	for _, r := range m.relations {
		if r.Buffers > maxBuf {
			maxBuf = r.Buffers
		}
	}
	for i := 0; i < rows; i++ {
		if i < len(m.relations) {
			lines = append(lines, relationRow(m.relations[i], maxBuf))
		} else {
			lines = append(lines, "")
		}
	}
	if m.err != "" {
		lines = append(lines, ui.ErrorStyle.Render(ui.FitWidth("error: "+m.err, innerW)))
	}

	return ui.PaneStyle.Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func relationRow(r db.RelationStat, maxBuf int) string {
	name := lipgloss.NewStyle().Width(18).Align(lipgloss.Right).Render(r.Relation)

	const barLen = 30
	filled := 0
	if maxBuf > 0 {
		filled = int(float64(r.Buffers) / float64(maxBuf) * float64(barLen))
	}
	dirtyCells := 0
	if r.Buffers > 0 && r.Dirty > 0 {
		dirtyCells = int(math.Round(float64(r.Dirty) / float64(r.Buffers) * float64(filled)))
		if dirtyCells > filled {
			dirtyCells = filled
		}
	}
	bar := barFillStyle.Render(strings.Repeat("█", filled-dirtyCells)) +
		barDirtyStyle.Render(strings.Repeat("█", dirtyCells)) +
		barEmptyStyle.Render(strings.Repeat("░", barLen-filled))

	count := ui.HintStyle.Render(fmt.Sprintf("%d", r.Buffers))
	if r.Dirty > 0 {
		count += orangeStyle.Render(fmt.Sprintf(" · dirty %d", r.Dirty))
	}
	return name + " " + bar + " " + count
}
