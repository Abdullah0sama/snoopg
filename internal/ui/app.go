package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgconn"

	"snoopg/internal/db"
)

type app struct {
	client       *db.Client
	mods         []Module
	active       int
	events      []string
	ext         []string
	extActs     []db.Activity
	extOn       bool
	watchSel    int
	watchSelect bool
	connLabel    string
	suggestWords []string
	hist         []string
	histIdx      int
	results      db.QueryResult
	lastQuery    string
	showResults  bool
	resOffset    int
	input        textinput.Model
	width        int
	height       int
	explainMode  bool
}

func labelFor(dsn string) string {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil || cfg.Database == "" {
		return dsn
	}
	return fmt.Sprintf("%s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
}

type execResultMsg struct {
	query        string
	rowsAffected int64
	elapsed      time.Duration
	err          error
	result       db.QueryResult
}

type completionMsg struct {
	words []string
}

type extMsg struct {
	lines []string
	acts  []db.Activity
	err   error
}

type extTickMsg struct{}

func NewApp(client *db.Client, mods []Module) tea.Model {
	input := textinput.New()
	input.Placeholder = "type a query, enter to run"
	input.Focus()
	return &app{
		client:    client,
		mods:      mods,
		input:     input,
		histIdx:   -1,
		connLabel: labelFor(client.DSN()),
	}
}

func completionCmd(c *db.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		words, err := c.CompletionWords(ctx)
		if err != nil {
			words = nil
		}
		return completionMsg{words: words}
	}
}

func queryCmd(c *db.Client, query string) tea.Cmd {
	return func() tea.Msg {
		res, elapsed, err := c.Query(context.Background(), query)
		return execResultMsg{
			query:        query,
			rowsAffected: res.RowsAffected,
			elapsed:      elapsed,
			err:          err,
			result:       res,
		}
	}
}

func explainCmd(c *db.Client, query string, analyzed bool) tea.Cmd {
	return func() tea.Msg {
		plan, elapsed, err := c.Explain(context.Background(), query, analyzed)
		return ExplainResultMsg{
			Query:    query,
			Plan:     plan,
			Elapsed:  elapsed,
			Analyzed: analyzed,
			Err:      err,
		}
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
		return extMsg{lines: lines, acts: acts, err: err}
	}
}

func (a *app) toggleWatch() tea.Cmd {
	a.extOn = !a.extOn
	if !a.extOn {
		a.ext = nil
		a.extActs = nil
		a.watchSelect = false
		return nil
	}
	return fetchExtCmd(a.client)
}

func (a *app) Init() tea.Cmd {
	if len(a.mods) == 0 {
		return completionCmd(a.client)
	}
	return tea.Batch(a.mods[a.active].Init(), completionCmd(a.client))
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	}

	if a.showResults {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			visible := a.resultsVisible()
			maxOff := len(a.results.Rows) - visible
			if maxOff < 0 {
				maxOff = 0
			}
			switch msg.String() {
			case "q", "ctrl+c":
				return a, tea.Quit
			case "j", "down":
				if a.resOffset < maxOff {
					a.resOffset++
				}
			case "k", "up":
				if a.resOffset > 0 {
					a.resOffset--
				}
			default:
				a.showResults = false
			}
		}
		return a, nil
	}

	switch msg := msg.(type) {
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
		if msg.err == nil {
			a.addHistory(msg.query)
			if len(msg.result.Columns) > 0 {
				a.results = msg.result
				a.lastQuery = msg.query
				a.resOffset = 0
				a.showResults = true
				return a, nil
			}
			if len(a.mods) > 0 {
				m, cmd := a.mods[a.active].Update(RefreshMsg{})
				a.mods[a.active] = m
				return a, cmd
			}
		}
		return a, nil

	case ExplainResultMsg:
		ts := time.Now().Format("15:04:05")
		q := truncate(msg.Query, 50)
		var line string
		if msg.Err != nil {
			line = fmt.Sprintf("%s %s — ERROR: %s", ts, q, truncate(msg.Err.Error(), 80))
		} else {
			line = fmt.Sprintf("%s explain %s — %.2fs", ts, q, msg.Elapsed.Seconds())
		}
		a.events = append(a.events, line)
		if len(a.events) > 200 {
			a.events = a.events[len(a.events)-200:]
		}
		if msg.Err == nil {
			a.addHistory(msg.Query)
		}
		for i, mod := range a.mods {
			if mod.Title() == "plans" {
				a.active = i
				m, cmd := mod.Update(msg)
				a.mods[i] = m
				return a, cmd
			}
		}
		return a, nil

	case completionMsg:
		a.suggestWords = msg.words
		return a, nil

	case extMsg:
		if msg.err == nil {
			a.ext = msg.lines
			a.extActs = msg.acts
			if a.watchSel > len(a.extActs)-1 {
				a.watchSel = len(a.extActs) - 1
			}
			if a.watchSel < 0 {
				a.watchSel = 0
			}
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

	case ProfileChangedMsg:
		a.connLabel = msg.Label
		cmds := []tea.Cmd{completionCmd(a.client)}
		if len(a.mods) > 0 {
			m, cmd := a.mods[a.active].Update(RefreshMsg{})
			a.mods[a.active] = m
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case tea.KeyMsg:
		if msg.String() != "ctrl+c" && !a.input.Focused() && len(a.mods) > 0 {
			if sink, ok := a.mods[a.active].(KeySink); ok && sink.WantsAllKeys() {
				m, cmd := a.mods[a.active].Update(msg)
				a.mods[a.active] = m
				return a, cmd
			}
		}
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "ctrl+e":
			a.explainMode = !a.explainMode
			if a.explainMode {
				a.input.Placeholder = "EXPLAIN ANALYZE will run — type a query"
			} else {
				a.input.Placeholder = "type a query, enter to run"
			}
			return a, nil
		case "tab":
			if a.input.Focused() {
				var cmd tea.Cmd
				a.input, cmd = a.input.Update(msg)
				return a, cmd
			}
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
		case "enter":
			if a.input.Focused() {
				if strings.TrimSpace(a.input.Value()) != "" {
					query := a.input.Value()
					a.input.SetValue("")
					a.input.ShowSuggestions = false
					if a.explainMode {
						return a, explainCmd(a.client, query, true)
					}
					return a, queryCmd(a.client, query)
				}
				return a, nil
			}
			if a.watchSelect && len(a.extActs) > 0 {
				act := a.extActs[a.watchSel]
				a.watchSelect = false
				return a, explainCmd(a.client, act.Query, false)
			}
			if len(a.mods) > 0 {
				m, cmd := a.mods[a.active].Update(msg)
				a.mods[a.active] = m
				return a, cmd
			}
			return a, nil
		case "esc":
			if a.watchSelect {
				a.watchSelect = false
				return a, nil
			}
			if a.input.Focused() {
				a.input.Blur()
			} else {
				a.input.Focus()
			}
			return a, nil
		case "f2":
			return a, a.toggleWatch()
		case "up":
			if !a.input.Focused() && a.watchSelect {
				if a.watchSel > 0 {
					a.watchSel--
				}
				return a, nil
			}
			if a.input.Focused() {
				if a.input.ShowSuggestions {
					var cmd tea.Cmd
					a.input, cmd = a.input.Update(msg)
					a.updateSuggestions()
					return a, cmd
				}
				a.historyUp()
				return a, nil
			}
		case "down":
			if !a.input.Focused() && a.watchSelect {
				if a.watchSel < len(a.extActs)-1 {
					a.watchSel++
				}
				return a, nil
			}
			if a.input.Focused() {
				if a.input.ShowSuggestions {
					var cmd tea.Cmd
					a.input, cmd = a.input.Update(msg)
					a.updateSuggestions()
					return a, cmd
				}
				a.historyDown()
				return a, nil
			}
		default:
			if a.input.Focused() {
				var cmd tea.Cmd
				a.input, cmd = a.input.Update(msg)
				a.updateSuggestions()
				return a, cmd
			}
			if a.watchSelect {
				switch msg.String() {
				case "j":
					if a.watchSel < len(a.extActs)-1 {
						a.watchSel++
					}
				case "k":
					if a.watchSel > 0 {
						a.watchSel--
					}
				}
				return a, nil
			}
			switch msg.String() {
			case "q":
				return a, tea.Quit
			case "e":
				return a, a.toggleWatch()
			case "w":
				if a.extOn && len(a.extActs) > 0 {
					a.watchSelect = true
					if a.watchSel > len(a.extActs)-1 {
						a.watchSel = len(a.extActs) - 1
					}
				}
				return a, nil
			case "r":
				if a.lastQuery != "" {
					a.resOffset = 0
					a.showResults = true
					return a, nil
				}
			}
			if d := msg.String(); len(d) == 1 && d[0] >= '1' && d[0] <= '9' {
				idx := int(d[0] - '1')
				if idx < len(a.mods) {
					a.active = idx
					return a, a.mods[a.active].Init()
				}
				return a, nil
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
	if a.showResults {
		return a.resultsView()
	}

	tabs := make([]string, 0, len(a.mods))
	for i, m := range a.mods {
		if i == a.active {
			tabs = append(tabs, TabActiveStyle.Render(m.Title()))
		} else {
			tabs = append(tabs, TabStyle.Render(m.Title()))
		}
	}
	header := Logo()
	if a.connLabel != "" {
		header += HintStyle.Render(" · " + a.connLabel)
	}
	_, readOnly := a.client.Info()
	if readOnly {
		header += " " + WarnStyle.Render("[RO]")
	} else {
		header += " " + HintStyle.Render("[RW]")
	}
	header += " " + lipgloss.JoinHorizontal(lipgloss.Left, tabs...)

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
		modView = PaneStyle.Width(leftW).Height(contentH - 2).Render("")
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
	if a.watchSelect {
		title += WatchStyle.Render("select")
		title += HintStyle.Render(" · enter: explain · esc: back")
	} else if a.extOn {
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
			line := FitWidth(a.ext[i], rightInnerW)
			if a.watchSelect && i == a.watchSel {
				evLines = append(evLines, SelStyle.Render(line))
			} else {
				evLines = append(evLines, WatchStyle.Render(line))
			}
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
	eventsPane := PaneStyle.Width(rightW).Height(contentH - 2).Render(lipgloss.JoinVertical(lipgloss.Left, evLines...))

	content := lipgloss.JoinHorizontal(lipgloss.Top, modView, " ", eventsPane)

	var inputLine string
	if a.input.Focused() {
		a.input.Width = a.width - 8
		if a.input.Width < 1 {
			a.input.Width = 1
		}
		prompt := "query: "
		if a.explainMode {
			prompt = "explain: "
		}
		inputLine = PromptStyle.Render(prompt) + a.input.View()
	} else {
		inputLine = HintStyle.Render("esc: focus query · j/k: select · e: watch · w: pick query · r: results · q: quit")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, inputLine)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "ON",
	"AND", "OR", "NOT", "GROUP", "BY", "ORDER", "LIMIT", "OFFSET", "INSERT",
	"INTO", "VALUES", "UPDATE", "SET", "DELETE", "CREATE", "TABLE", "DROP",
	"ALTER", "INDEX", "VACUUM", "ANALYZE", "EXPLAIN", "CHECKPOINT", "BEGIN",
	"COMMIT", "ROLLBACK", "COUNT", "SUM", "AVG", "MIN", "MAX", "AS", "DISTINCT",
	"NULL", "IS", "LIKE", "ILIKE", "IN", "BETWEEN", "HAVING", "UNION",
	"RETURNING", "WITH", "CASE", "WHEN", "THEN", "ELSE", "END", "USING",
}

func (a *app) updateSuggestions() {
	v := a.input.Value()
	fields := strings.FieldsFunc(v, func(r rune) bool {
		return !(r == '_' || r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})
	word := ""
	if len(fields) > 0 {
		word = fields[len(fields)-1]
	}
	if word == "" {
		a.input.SetSuggestions(nil)
		a.input.ShowSuggestions = false
		return
	}
	lw := strings.ToLower(word)
	var out []string
	for _, k := range sqlKeywords {
		if strings.HasPrefix(strings.ToLower(k), lw) && !strings.EqualFold(k, word) {
			out = append(out, k)
		}
	}
	for _, w := range a.suggestWords {
		if strings.HasPrefix(strings.ToLower(w), lw) && !strings.EqualFold(w, word) {
			out = append(out, w)
		}
		if len(out) >= 8 {
			break
		}
	}
	if len(out) == 0 {
		a.input.SetSuggestions(nil)
		a.input.ShowSuggestions = false
		return
	}
	a.input.SetSuggestions(out)
	a.input.ShowSuggestions = true
}

func (a *app) addHistory(query string) {
	if len(a.hist) == 0 || a.hist[len(a.hist)-1] != query {
		a.hist = append(a.hist, query)
		if len(a.hist) > 100 {
			a.hist = a.hist[len(a.hist)-100:]
		}
	}
	a.histIdx = -1
}

func (a *app) historyUp() {
	if len(a.hist) == 0 {
		return
	}
	if a.histIdx == -1 {
		a.histIdx = len(a.hist) - 1
	} else if a.histIdx > 0 {
		a.histIdx--
	}
	a.input.SetValue(a.hist[a.histIdx])
	a.input.CursorEnd()
}

func (a *app) historyDown() {
	if a.histIdx == -1 {
		return
	}
	if a.histIdx < len(a.hist)-1 {
		a.histIdx++
		a.input.SetValue(a.hist[a.histIdx])
		a.input.CursorEnd()
	} else {
		a.histIdx = -1
		a.input.SetValue("")
	}
}

func (a *app) resultsVisible() int {
	visible := a.height - 8
	if visible < 1 {
		visible = 1
	}
	return visible
}

func padCell(s string, w int) string {
	r := []rune(s)
	if len(r) > w {
		r = r[:w]
	}
	return string(r) + strings.Repeat(" ", w-len(r))
}

func (a *app) resultsView() string {
	boxW := a.width * 4 / 5
	if boxW > a.width-2 {
		boxW = a.width - 2
	}
	if boxW < 40 {
		boxW = 40
	}
	boxH := a.height - 1
	if boxH < 8 {
		boxH = 8
	}
	innerW := boxW - 4

	title := PaneTitleStyle.Render("results") + HintStyle.Render(" — "+truncate(strings.TrimSpace(a.lastQuery), innerW-12))
	lines := []string{title}

	if len(a.results.Columns) > 0 {
		widths := make([]int, len(a.results.Columns))
		for i, c := range a.results.Columns {
			widths[i] = len([]rune(c))
			if widths[i] > 28 {
				widths[i] = 28
			}
		}
		for _, row := range a.results.Rows {
			for i, cell := range row {
				if i >= len(widths) {
					break
				}
				l := len([]rune(cell))
				if l > 28 {
					l = 28
				}
				if l > widths[i] {
					widths[i] = l
				}
			}
		}
		hdr := make([]string, len(a.results.Columns))
		for i, c := range a.results.Columns {
			hdr[i] = PaneTitleStyle.Render(padCell(c, widths[i]+2))
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, hdr...))
		lines = append(lines, HintStyle.Render(strings.Repeat("─", innerW)))

		visible := a.resultsVisible()
		for i := a.resOffset; i < len(a.results.Rows) && i < a.resOffset+visible; i++ {
			cells := make([]string, len(widths))
			for j := range widths {
				if j < len(a.results.Rows[i]) {
					cells[j] = padCell(a.results.Rows[i][j], widths[j]+2)
				} else {
					cells[j] = strings.Repeat(" ", widths[j]+2)
				}
			}
			lines = append(lines, FitWidth(lipgloss.JoinHorizontal(lipgloss.Left, cells...), innerW))
		}
		if a.results.Truncated {
			lines = append(lines, HintStyle.Render("showing first 200 rows"))
		}
	} else {
		lines = append(lines, HintStyle.Render(fmt.Sprintf("%d rows affected", a.results.RowsAffected)))
	}

	footer := HintStyle.Render("j/k: scroll · esc/enter: close · r: reopen")
	box := PaneStyle.Width(boxW).Height(boxH - 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	out := lipgloss.JoinVertical(lipgloss.Left, box, footer)
	leftPad := (a.width - boxW) / 2
	if leftPad > 0 {
		outLines := strings.Split(out, "\n")
		for i := range outLines {
			outLines[i] = strings.Repeat(" ", leftPad) + outLines[i]
		}
		out = strings.Join(outLines, "\n")
	}
	return out
}
