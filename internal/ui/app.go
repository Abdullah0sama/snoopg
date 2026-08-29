package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"snoopg/internal/db"
)

type app struct {
	client *db.Client
	mods   []Module
	active int
	events []string
	ext    []string
	extOn  bool
	input  textinput.Model
	width  int
	height int
}

type execResultMsg struct {
	query        string
	rowsAffected int64
	elapsed      time.Duration
	err          error
}

type extMsg struct {
	lines []string
	err   error
}

type extTickMsg struct{}

func NewApp(client *db.Client, mods []Module) tea.Model {
	input := textinput.New()
	input.Placeholder = "type a query, enter to run"
	input.Focus()
	return &app{
		client: client,
		mods:   mods,
		input:  input,
	}
}

func execQueryCmd(c *db.Client, query string) tea.Cmd {
	return func() tea.Msg {
		n, elapsed, err := c.Exec(context.Background(), query)
		return execResultMsg{query: query, rowsAffected: n, elapsed: elapsed, err: err}
	}
}

func fetchExtCmd(c *db.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		acts, err := c.Activity(ctx)
		lines := make([]string, 0, len(acts))
		for _, act := range acts {
			secs := act.Seconds
			if secs < 0 {
				secs = 0
			}
			line := fmt.Sprintf("pid=%-6d %-12s %s (%.1fs)", act.PID, act.Database, truncate(act.Query, 40), secs)
			if act.WaitEvent != "" {
				line += " · " + act.WaitEvent
			}
			lines = append(lines, line)
		}
		return extMsg{lines: lines, err: err}
	}
}

func (a *app) toggleWatch() tea.Cmd {
	a.extOn = !a.extOn
	if !a.extOn {
		a.ext = nil
		return nil
	}
	return fetchExtCmd(a.client)
}

func (a *app) Init() tea.Cmd {
	if len(a.mods) == 0 {
		return nil
	}
	return a.mods[a.active].Init()
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case execResultMsg:
		ts := time.Now().Format("15:04:05")
		q := truncate(msg.query, 60)
		var line string
		if msg.err != nil {
			line = fmt.Sprintf("%s %s — ERROR: %s", ts, q, truncate(msg.err.Error(), 80))
		} else {
			line = fmt.Sprintf("%s %s — %.1fs, %d rows", ts, q, msg.elapsed.Seconds(), msg.rowsAffected)
		}
		a.events = append(a.events, line)
		if len(a.events) > 200 {
			a.events = a.events[len(a.events)-200:]
		}
		if msg.err == nil && len(a.mods) > 0 {
			m, cmd := a.mods[a.active].Update(RefreshMsg{})
			a.mods[a.active] = m
			return a, cmd
		}
		return a, nil

	case extMsg:
		if msg.err == nil {
			a.ext = msg.lines
		}
		if a.extOn {
			return a, tea.Tick(time.Second, func(time.Time) tea.Msg { return extTickMsg{} })
		}
		return a, nil

	case extTickMsg:
		if a.extOn {
			return a, fetchExtCmd(a.client)
		}
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "tab":
			if len(a.mods) > 0 {
				a.active = (a.active + 1) % len(a.mods)
				return a, a.mods[a.active].Init()
			}
			return a, nil
		case "shift+tab":
			if len(a.mods) > 0 {
				a.active = (a.active - 1 + len(a.mods)) % len(a.mods)
				return a, a.mods[a.active].Init()
			}
			return a, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx < len(a.mods) {
				a.active = idx
				return a, a.mods[a.active].Init()
			}
			return a, nil
		case "enter":
			if strings.TrimSpace(a.input.Value()) != "" {
				query := a.input.Value()
				a.input.SetValue("")
				return a, execQueryCmd(a.client, query)
			}
			return a, nil
		case "esc":
			if a.input.Focused() {
				a.input.Blur()
			} else {
				a.input.Focus()
			}
			return a, nil
		case "f2":
			return a, a.toggleWatch()
		default:
			if a.input.Focused() {
				var cmd tea.Cmd
				a.input, cmd = a.input.Update(msg)
				return a, cmd
			}
			if msg.String() == "e" {
				return a, a.toggleWatch()
			}
		}
	}

	if len(a.mods) > 0 {
		m, cmd := a.mods[a.active].Update(msg)
		a.mods[a.active] = m
		return a, cmd
	}
	return a, nil
}

func (a *app) View() string {
	if a.width <= 0 {
		return "loading..."
	}

	tabs := make([]string, 0, len(a.mods))
	for i, m := range a.mods {
		if i == a.active {
			tabs = append(tabs, TabActiveStyle.Render(m.Title()))
		} else {
			tabs = append(tabs, TabStyle.Render(m.Title()))
		}
	}
	header := Logo() + " " + lipgloss.JoinHorizontal(lipgloss.Left, tabs...)

	contentH := a.height - 2
	if contentH < 1 {
		contentH = 1
	}
	leftW := a.width * 3 / 5
	rightW := a.width - leftW - 1
	if leftW < 8 {
		leftW = 8
	}
	if rightW < 8 {
		rightW = 8
	}
	if leftW+rightW+1 > a.width {
		leftW = a.width / 2
		rightW = a.width - leftW - 1
	}

	var modView string
	if len(a.mods) > 0 {
		modView = a.mods[a.active].View(leftW, contentH)
	} else {
		modView = PaneStyle.Width(leftW).Height(contentH).Render("")
	}

	innerH := contentH - 2
	if innerH < 1 {
		innerH = 1
	}
	rightInnerW := rightW - 4
	if rightInnerW < 1 {
		rightInnerW = 1
	}
	title := PaneTitleStyle.Render("EVENTS") + " "
	if a.extOn {
		title += WatchStyle.Render("watch on")
	} else {
		title += HintStyle.Render("watch off (f2)")
	}
	evLines := []string{title}
	evCount := innerH - 1
	if evCount < 0 {
		evCount = 0
	}
	localCount := evCount
	if a.extOn && len(a.ext) > 0 {
		extCap := evCount / 3
		if extCap < 1 {
			extCap = 1
		}
		extRows := len(a.ext)
		if extRows > extCap {
			extRows = extCap
		}
		for i := 0; i < extRows; i++ {
			evLines = append(evLines, WatchStyle.Render(FitWidth(a.ext[i], rightInnerW)))
		}
		evLines = append(evLines, HintStyle.Render(strings.Repeat("─", rightInnerW)))
		localCount = evCount - extRows - 1
	}
	if localCount < 0 {
		localCount = 0
	}
	start := 0
	if len(a.events) > localCount {
		start = len(a.events) - localCount
	}
	for i := start; i < len(a.events); i++ {
		evLines = append(evLines, FitWidth(a.events[i], rightInnerW))
	}
	for len(evLines) < innerH {
		evLines = append(evLines, "")
	}
	eventsPane := PaneStyle.Width(rightW).Height(contentH).Render(lipgloss.JoinVertical(lipgloss.Left, evLines...))

	content := lipgloss.JoinHorizontal(lipgloss.Top, modView, " ", eventsPane)

	var inputLine string
	if a.input.Focused() {
		a.input.Width = a.width - 8
		if a.input.Width < 1 {
			a.input.Width = 1
		}
		inputLine = PromptStyle.Render("query: ") + a.input.View()
	} else {
		inputLine = HintStyle.Render("esc: focus query · j/k: select table · e: watch queries · tab: module")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, inputLine)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
