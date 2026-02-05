// Package statsui provides the Bubble Tea stats interface.
package statsui

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/verte-zerg/tuipe/internal/model"
	"github.com/verte-zerg/tuipe/internal/stats"
	"github.com/verte-zerg/tuipe/internal/store"
)

const (
	tabOverview = iota
	tabCharTable
	tabCharCurves
)

const (
	plotHeight = 10
)

var (
	activeNavStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F0F0F0")).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("#C89A3A"))
	inactiveNavStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#B0B0B0")).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder(), true).
				BorderForeground(lipgloss.Color("#4A4A4A"))
	headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E6E6E"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4D4F"))
	cardStyle   = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("#4A4A4A"))
	cardTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8C8C8C"))
	cardValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	tableMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#B8B8B8"))
	modalStyle      = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("#C89A3A")).
			Padding(1, 2)
)

// Model implements the Bubble Tea stats UI.
type Model struct {
	store *store.Store
	cfg   model.StatsConfig

	report     stats.Report
	errMsg     string
	charErrMsg string

	tabs       []string
	activeTab  int
	viewports  []viewport.Model
	charTable  table.Model
	charLayout tableLayout

	width  int
	height int

	filterMode   bool
	filterInputs []textinput.Model
	filterIndex  int
	filterError  string

	charSelection       []string
	charSelectionCustom bool
	charPerSession      map[int64]map[string]model.CharAggregate

	charInputMode  bool
	charInput      textinput.Model
	charInputError string
}

type tableLayout struct {
	width    int
	height   int
	rowCount int
	colCount int
}

// NewModel constructs a stats UI model.
func NewModel(st *store.Store, cfg model.StatsConfig) *Model {
	m := &Model{
		store: st,
		cfg:   cfg,
		tabs:  []string{"Overview", "Char Table", "Char Curves"},
	}
	m.charSelection = parseChars(cfg.Chars)
	if len(m.charSelection) > 0 {
		m.charSelectionCustom = true
	}
	m.initInputs()
	m.initCharInput()
	m.initCharTable()
	m.initViewports()
	m.refreshReport()
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		m.renderTabContents()
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			return m, tea.Quit
		}
		if m.activeTab == tabCharTable {
			m.charTable.Focus()
		} else {
			m.charTable.Blur()
		}
		if m.filterMode {
			return m.updateFilter(msg)
		}
		if m.charInputMode {
			return m.updateCharInput(msg)
		}
		switch msg.String() {
		case "left", "h":
			m.moveTab(-1)
			return m, tea.ClearScreen
		case "right", "l":
			m.moveTab(1)
			return m, tea.ClearScreen
		case "=":
			m.cfg.CurveWindow = nextCurveWindow(m.cfg.CurveWindow)
			m.refreshReport()
			m.updateLayout()
			return m, nil
		case "-":
			m.cfg.CurveWindow = prevCurveWindow(m.cfg.CurveWindow)
			m.refreshReport()
			m.updateLayout()
			return m, nil
		case "/":
			return m.startFilter()
		case "enter":
			if m.activeTab == tabCharCurves {
				return m.startCharInput()
			}
			return m, nil
		case "g", "home":
			if m.activeTab == tabCharTable {
				m.charTable.GotoTop()
			} else {
				m.viewports[m.activeTab].GotoTop()
			}
			return m, nil
		case "G", "end":
			if m.activeTab == tabCharTable {
				m.charTable.GotoBottom()
			} else {
				m.viewports[m.activeTab].GotoBottom()
			}
			return m, nil
		default:
			if m.activeTab == tabCharTable {
				var cmd tea.Cmd
				m.charTable, cmd = m.charTable.Update(msg)
				return m, cmd
			}
			vp := m.viewports[m.activeTab]
			var cmd tea.Cmd
			vp, cmd = vp.Update(msg)
			m.viewports[m.activeTab] = vp
			return m, cmd
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.charInputMode {
		return fitLines(m.renderCharModal(), m.width, m.height)
	}
	headerHeight, bodyHeight, footerHeight := m.layoutHeights()
	header := fitLines(m.renderHeader(), m.width, headerHeight)
	body := fitLines(m.renderBody(bodyHeight), m.width, bodyHeight)
	footer := fitLines(m.renderFooter(), m.width, footerHeight)
	return strings.Join([]string{header, body, footer}, "\n")
}

func (m *Model) initViewports() {
	m.viewports = make([]viewport.Model, len(m.tabs))
	for i := range m.viewports {
		m.viewports[i] = viewport.New(0, 0)
	}
}

func (m *Model) initInputs() {
	m.filterInputs = []textinput.Model{
		newFilterInput("Lang: "),
		newFilterInput("Since (YYYY-MM-DD): "),
		newFilterInput("Last: "),
		newFilterInput("Curve window: "),
	}
	m.setInputsFromConfig()
}

func (m *Model) initCharTable() {
	m.charTable = buildCharTable(nil, nil, 0, 1)
}

func (m *Model) layoutHeights() (headerHeight, bodyHeight, footerHeight int) {
	tabsHeight := lipgloss.Height(activeNavStyle.Render("X"))
	if tabsHeight < 1 {
		tabsHeight = 1
	}
	headerHeight = tabsHeight + 1
	footerHeight = 1
	if !m.filterMode && m.errMsg != "" {
		footerHeight++
	}
	bodyHeight = m.height - headerHeight - footerHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	return headerHeight, bodyHeight, footerHeight
}

func (m *Model) initCharInput() {
	m.charInput = newFilterInput("Chars: ")
	m.charInput.Prompt = "Chars: "
	m.charInput.Placeholder = "asdfjkl;"
}

func newFilterInput(prompt string) textinput.Model {
	input := textinput.New()
	input.Prompt = prompt
	input.CharLimit = 0
	input.Cursor.SetMode(cursor.CursorBlink)
	return input
}

func (m *Model) setInputsFromConfig() {
	if len(m.filterInputs) == 0 {
		return
	}
	m.filterInputs[0].SetValue(strings.TrimSpace(m.cfg.Lang))
	if m.cfg.Since != nil {
		m.filterInputs[1].SetValue(m.cfg.Since.Format("2006-01-02"))
	} else {
		m.filterInputs[1].SetValue("")
	}
	if m.cfg.Last > 0 {
		m.filterInputs[2].SetValue(strconv.Itoa(m.cfg.Last))
	} else {
		m.filterInputs[2].SetValue("")
	}
	m.filterInputs[3].SetValue(strconv.Itoa(m.cfg.CurveWindow))
}

func (m *Model) updateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	_, vpHeight, _ := m.layoutHeights()
	for i := range m.viewports {
		m.viewports[i].Width = m.width
		m.viewports[i].Height = vpHeight
	}
	m.setCharTableSize(m.width, vpHeight)
	for i := range m.filterInputs {
		promptWidth := lipgloss.Width(m.filterInputs[i].Prompt)
		m.filterInputs[i].Width = maxInt(10, m.width-promptWidth-2)
	}
	promptWidth := lipgloss.Width(m.charInput.Prompt)
	m.charInput.Width = maxInt(10, modalInnerWidth(m.width)-promptWidth)
}

func (m *Model) moveTab(delta int) {
	count := len(m.tabs)
	if count == 0 {
		return
	}
	next := m.activeTab + delta
	if next < 0 {
		next = count - 1
	}
	if next >= count {
		next = 0
	}
	m.activeTab = next
	if m.activeTab == tabCharTable {
		m.charTable.Focus()
	} else {
		m.charTable.Blur()
	}
}

func (m *Model) renderTabs() string {
	parts := make([]string, 0, len(m.tabs))
	for i, tab := range m.tabs {
		if i == m.activeTab {
			parts = append(parts, activeNavStyle.Render(tab))
		} else {
			parts = append(parts, inactiveNavStyle.Render(tab))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m *Model) renderHeader() string {
	tabs := padLines(m.renderTabs(), m.width)
	filters := padLines(m.renderFilterSummary(), m.width)
	return tabs + "\n" + filters
}

func (m *Model) renderFilterSummary() string {
	lang := m.cfg.Lang
	if lang == "" {
		lang = "any"
	}
	since := "any"
	if m.cfg.Since != nil {
		since = m.cfg.Since.Format("2006-01-02")
	}
	last := "all"
	if m.cfg.Last > 0 {
		last = strconv.Itoa(m.cfg.Last)
	}
	summary := fmt.Sprintf("Settings: lang=%s  since=%s  last=%s  window=%d", lang, since, last, m.cfg.CurveWindow)
	summary = truncateLine(summary, m.width)
	return headerStyle.Render(summary)
}

func (m *Model) renderHelp() string {
	help := "Nav: left/right  Scroll: up/down/pgup/pgdn  Window: -/=  Settings: /  Quit: q"
	if m.activeTab == tabCharCurves {
		help = "Nav: left/right  Scroll: up/down/pgup/pgdn  Edit chars: enter  Window: -/=  Settings: /  Quit: q"
	}
	return headerStyle.Render(help)
}

func (m *Model) renderFilterHelp() string {
	return headerStyle.Render("tab/shift+tab: next field  enter: apply  esc: cancel  quit: q")
}

func (m *Model) renderFooter() string {
	if m.filterMode {
		return m.renderFilterHelp()
	}
	if m.errMsg != "" {
		return m.renderHelp() + "\n" + errorStyle.Render(m.errMsg)
	}
	return m.renderHelp()
}

func (m *Model) renderFilterForm() string {
	lines := []string{"Settings (enter to apply, esc to cancel)"}
	for _, input := range m.filterInputs {
		lines = append(lines, input.View())
	}
	if m.filterError != "" {
		lines = append(lines, errorStyle.Render(m.filterError))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderBody(height int) string {
	if m.filterMode {
		return fitLines(m.renderFilterForm(), m.width, height)
	}
	if m.activeTab == tabCharTable {
		switch {
		case len(m.report.Sessions) == 0:
			return fitLines("No sessions found.", m.width, height)
		case len(m.report.CharAggsAll) == 0:
			return fitLines("No character stats found.", m.width, height)
		default:
			view := tableMutedStyle.Render(m.charTable.View())
			return fitLines(view, m.width, height)
		}
	}
	return fitLines(m.viewports[m.activeTab].View(), m.width, height)
}

func (m *Model) refreshReport() {
	report, err := stats.BuildReport(context.Background(), m.store, m.cfg)
	if err != nil {
		m.errMsg = err.Error()
		m.charErrMsg = ""
		for i := range m.viewports {
			m.viewports[i].SetContent("Failed to load stats.")
		}
		return
	}
	m.errMsg = ""
	m.report = report
	if !m.charSelectionCustom {
		m.charSelection = stats.TopCharsByFrequency(m.report.CharAggsAll, 5)
	}
	m.loadCharPerSession()
	width := m.width
	if width <= 0 {
		width = 80
	}
	_, bodyHeight, _ := m.layoutHeights()
	applyCharTable(m, m.report.Sessions, m.report.CharAggsAll, width, bodyHeight, true)
	m.renderTabContents()
}

func (m *Model) renderTabContents() {
	if len(m.viewports) == 0 {
		return
	}
	if m.errMsg != "" {
		for i := range m.viewports {
			m.viewports[i].SetContent("Failed to load stats.")
		}
		return
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	m.viewports[tabOverview].SetContent(renderOverview(m.report.Sessions, m.cfg.CurveWindow, width))
	m.viewports[tabCharCurves].SetContent(renderCharCurves(m.report.Sessions, m.charSelection, m.charPerSession, m.cfg.CurveWindow, width, m.charErrMsg))
}

func renderOverview(sessions []model.SessionAggregate, window, width int) string {
	if len(sessions) == 0 {
		return "No sessions found."
	}
	summary := renderSummaryCards(sessions, width)
	curves := renderCurves(sessions, window, width)
	return strings.TrimRight(summary+"\n\n"+curves, "\n")
}

func renderSummaryCards(sessions []model.SessionAggregate, width int) string {
	if len(sessions) == 0 {
		return "No sessions found."
	}
	var totalWPM, totalCPM, totalAcc float64
	bestWPM := 0.0
	for _, s := range sessions {
		wpm, cpm, acc := stats.SessionMetrics(s.Correct, s.Incorrect, s.DurationMs)
		totalWPM += wpm
		totalCPM += cpm
		totalAcc += acc
		if wpm > bestWPM {
			bestWPM = wpm
		}
	}
	count := float64(len(sessions))
	cards := []string{
		metricCard("Sessions", fmt.Sprintf("%d", len(sessions))),
		metricCard("Avg WPM", fmt.Sprintf("%.1f", totalWPM/count)),
		metricCard("Best WPM", fmt.Sprintf("%.1f", bestWPM)),
		metricCard("Avg CPM", fmt.Sprintf("%.1f", totalCPM/count)),
		metricCard("Avg Acc", fmt.Sprintf("%.1f%%", (totalAcc/count)*100)),
	}
	if width < 80 {
		return strings.Join(cards, "\n")
	}
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, cards[0], cards[1], cards[2])
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, cards[3], cards[4])
	return lipgloss.JoinVertical(lipgloss.Left, row1, row2)
}

func metricCard(label, value string) string {
	content := fmt.Sprintf("%s\n%s", cardTitleStyle.Render(label), cardValueStyle.Render(value))
	return cardStyle.Render(content)
}

func renderCurves(sessions []model.SessionAggregate, window, width int) string {
	var buf bytes.Buffer
	if err := stats.RenderCurvesWithSize(&buf, sessions, window, width, plotHeight, true); err != nil {
		return fmt.Sprintf("Failed to render curves: %v", err)
	}
	return strings.TrimRight(buf.String(), "\n")
}

func buildCharTable(sessions []model.SessionAggregate, aggs []model.CharAggregate, width, height int) table.Model {
	columns := []table.Column{
		{Title: "Char", Width: 4},
		{Title: "Accuracy", Width: 9},
		{Title: "Avg Latency (ms)", Width: 17},
		{Title: "Correct", Width: 7},
		{Title: "Incorrect", Width: 9},
		{Title: "Total", Width: 6},
	}
	rows := make([]table.Row, 0, len(aggs))
	if len(sessions) > 0 && len(aggs) > 0 {
		sorted := sortCharAggsByTotal(aggs)
		for _, agg := range sorted {
			total := agg.Correct + agg.Incorrect
			acc := 0.0
			if total > 0 {
				acc = float64(agg.Correct) / float64(total) * 100
			}
			lat := 0.0
			if agg.LatencyCount > 0 {
				lat = float64(agg.LatencySumMs) / float64(agg.LatencyCount)
			}
			charLabel := agg.Char
			if charLabel == " " {
				charLabel = "<space>"
			}
			rows = append(rows, table.Row{
				charLabel,
				fmt.Sprintf("%.2f%%", acc),
				fmt.Sprintf("%.1f", lat),
				fmt.Sprintf("%d", agg.Correct),
				fmt.Sprintf("%d", agg.Incorrect),
				fmt.Sprintf("%d", total),
			})
		}
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(maxInt(1, height-1)),
	)
	t.SetWidth(width)
	styles := charTableStyles()
	t.SetStyles(styles)
	return t
}

func applyCharTable(m *Model, sessions []model.SessionAggregate, aggs []model.CharAggregate, width, height int, force bool) {
	cols, rows := buildCharTableData(sessions, aggs)
	viewportHeight := maxInt(1, height-1)
	if !force &&
		m.charLayout.width == width &&
		m.charLayout.height == viewportHeight &&
		m.charLayout.rowCount == len(rows) &&
		m.charLayout.colCount == len(cols) {
		return
	}
	m.charTable.SetColumns(cols)
	m.charTable.SetRows(rows)
	m.charLayout.rowCount = len(rows)
	m.charLayout.colCount = len(cols)
	m.setCharTableSize(width, height)
}

func (m *Model) setCharTableSize(width, height int) {
	viewportHeight := maxInt(1, height-1)
	if m.charLayout.width == width && m.charLayout.height == viewportHeight {
		return
	}
	m.charLayout.width = width
	m.charLayout.height = viewportHeight
	m.charTable.SetWidth(width)
	m.charTable.SetHeight(viewportHeight)
	viewportHeight = m.adjustCharTableHeight(height)
	if m.charLayout.height != viewportHeight {
		m.charLayout.height = viewportHeight
		m.charTable.SetHeight(viewportHeight)
	}
}

func charTableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("#4A4A4A")).
		Foreground(lipgloss.Color("#C0C0C0")).
		Bold(true).
		Padding(0, 1).
		PaddingLeft(0)
	styles.Cell = styles.Cell.
		Padding(0, 1).
		PaddingLeft(0)
	styles.Selected = styles.Cell.
		Foreground(lipgloss.Color("#F0F0F0")).
		Bold(true)
	return styles
}

func (m *Model) adjustCharTableHeight(bodyHeight int) int {
	target := maxInt(1, bodyHeight)
	height := m.charTable.Height()
	viewHeight := lipgloss.Height(m.charTable.View())
	if viewHeight == target {
		return height
	}
	height += target - viewHeight
	if height < 1 {
		height = 1
	}
	m.charTable.SetHeight(height)
	viewHeight = lipgloss.Height(m.charTable.View())
	if viewHeight == target {
		return height
	}
	height += target - viewHeight
	if height < 1 {
		height = 1
	}
	return height
}

func buildCharTableData(sessions []model.SessionAggregate, aggs []model.CharAggregate) ([]table.Column, []table.Row) {
	columns := []table.Column{
		{Title: "Char", Width: 4},
		{Title: "Accuracy", Width: 9},
		{Title: "Avg Latency (ms)", Width: 17},
		{Title: "Correct", Width: 7},
		{Title: "Incorrect", Width: 9},
		{Title: "Total", Width: 6},
	}
	rows := make([]table.Row, 0, len(aggs))
	if len(sessions) == 0 || len(aggs) == 0 {
		return columns, rows
	}
	sorted := sortCharAggsByTotal(aggs)
	for _, agg := range sorted {
		total := agg.Correct + agg.Incorrect
		acc := 0.0
		if total > 0 {
			acc = float64(agg.Correct) / float64(total) * 100
		}
		lat := 0.0
		if agg.LatencyCount > 0 {
			lat = float64(agg.LatencySumMs) / float64(agg.LatencyCount)
		}
		charLabel := agg.Char
		if charLabel == " " {
			charLabel = "<space>"
		}
		rows = append(rows, table.Row{
			charLabel,
			fmt.Sprintf("%.2f%%", acc),
			fmt.Sprintf("%.1f", lat),
			fmt.Sprintf("%d", agg.Correct),
			fmt.Sprintf("%d", agg.Incorrect),
			fmt.Sprintf("%d", total),
		})
	}
	return columns, rows
}

func renderCharCurves(sessions []model.SessionAggregate, chars []string, perSession map[int64]map[string]model.CharAggregate, window, width int, errMsg string) string {
	if len(sessions) == 0 {
		return "No sessions found."
	}
	if errMsg != "" {
		return fmt.Sprintf("Failed to load character curves: %s", errMsg)
	}
	if len(chars) == 0 {
		return "No characters selected. Press Enter to set chars."
	}
	header := headerStyle.Render(fmt.Sprintf("Chars: %s", strings.Join(chars, ", ")))
	var buf bytes.Buffer
	if err := stats.RenderCharCurvesWithSize(&buf, sessions, perSession, chars, window, width, plotHeight, true); err != nil {
		return fmt.Sprintf("Failed to render character curves: %v", err)
	}
	return strings.TrimRight(header+"\n"+buf.String(), "\n")
}

func (m *Model) startFilter() (tea.Model, tea.Cmd) {
	m.filterMode = true
	m.filterError = ""
	m.setInputsFromConfig()
	return m, m.setFilterIndex(0)
}

func (m *Model) startCharInput() (tea.Model, tea.Cmd) {
	m.charInputMode = true
	m.charInputError = ""
	m.charInput.SetValue(strings.Join(m.charSelection, ""))
	return m, m.charInput.Focus()
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.filterMode = false
		m.filterError = ""
		return m, nil
	case tea.KeyEnter:
		if err := m.applyFilter(); err != nil {
			m.filterError = err.Error()
			return m, nil
		}
		m.filterMode = false
		m.filterError = ""
		m.refreshReport()
		m.updateLayout()
		return m, nil
	case tea.KeyTab:
		return m, m.setFilterIndex(m.filterIndex + 1)
	case tea.KeyShiftTab:
		return m, m.setFilterIndex(m.filterIndex - 1)
	}
	var cmd tea.Cmd
	m.filterInputs[m.filterIndex], cmd = m.filterInputs[m.filterIndex].Update(msg)
	return m, cmd
}

func (m *Model) updateCharInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.charInputMode = false
		m.charInputError = ""
		return m, nil
	case tea.KeyEnter:
		m.applyCharInput()
		m.charInputMode = false
		m.charInputError = ""
		m.loadCharPerSession()
		m.renderTabContents()
		return m, nil
	}
	var cmd tea.Cmd
	m.charInput, cmd = m.charInput.Update(msg)
	normalized := normalizeCharInput(m.charInput.Value())
	if normalized != m.charInput.Value() {
		m.charInput.SetValue(normalized)
	}
	return m, cmd
}

func (m *Model) setFilterIndex(idx int) tea.Cmd {
	count := len(m.filterInputs)
	if count == 0 {
		return nil
	}
	if idx < 0 {
		idx = count - 1
	}
	if idx >= count {
		idx = 0
	}
	m.filterIndex = idx
	var cmd tea.Cmd
	for i := range m.filterInputs {
		if i == m.filterIndex {
			cmd = m.filterInputs[i].Focus()
		} else {
			m.filterInputs[i].Blur()
		}
	}
	return cmd
}

func (m *Model) applyFilter() error {
	lang := strings.TrimSpace(m.filterInputs[0].Value())
	sinceInput := strings.TrimSpace(m.filterInputs[1].Value())
	var since *time.Time
	if sinceInput != "" {
		parsed, err := time.ParseInLocation("2006-01-02", sinceInput, time.Local)
		if err != nil {
			return fmt.Errorf("invalid since date (expected YYYY-MM-DD)")
		}
		since = &parsed
	}

	lastInput := strings.TrimSpace(m.filterInputs[2].Value())
	last := 0
	if lastInput != "" {
		parsed, err := strconv.Atoi(lastInput)
		if err != nil || parsed < 0 {
			return fmt.Errorf("invalid last value (use 0 or positive integer)")
		}
		last = parsed
	}

	windowInput := strings.TrimSpace(m.filterInputs[3].Value())
	window := 0
	if windowInput != "" {
		parsed, err := strconv.Atoi(windowInput)
		if err != nil {
			return fmt.Errorf("invalid curve window (use integer)")
		}
		if parsed < 1 {
			return fmt.Errorf("invalid curve window (use integer >= 1)")
		}
		window = parsed
	}

	m.cfg = model.StatsConfig{
		Lang:        lang,
		Since:       since,
		Last:        last,
		CurveWindow: window,
	}
	return nil
}

func (m *Model) applyCharInput() {
	raw := normalizeCharInput(m.charInput.Value())
	if raw == "" {
		m.charSelectionCustom = false
		m.charSelection = stats.TopCharsByFrequency(m.report.CharAggsAll, 5)
		return
	}
	chars := parseRawChars(raw)
	if len(chars) == 0 {
		m.charSelectionCustom = false
		m.charSelection = stats.TopCharsByFrequency(m.report.CharAggsAll, 5)
		return
	}
	m.charSelectionCustom = true
	m.charSelection = chars
}

func (m *Model) renderCharModal() string {
	title := cardValueStyle.Render("Select Characters")
	body := []string{
		title,
		m.charInput.View(),
		headerStyle.Render("Type characters (no commas). Spaces are ignored."),
		headerStyle.Render("Enter to apply / Esc to cancel"),
	}
	if m.charInputError != "" {
		body = append(body, errorStyle.Render(m.charInputError))
	}
	box := modalStyle.Width(modalWidth(m.width)).Render(strings.Join(body, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) loadCharPerSession() {
	m.charErrMsg = ""
	m.charPerSession = nil
	if len(m.report.Sessions) == 0 || len(m.charSelection) == 0 {
		return
	}
	perSession, err := m.store.ListCharStatsForSessions(context.Background(), sessionIDs(m.report.Sessions), m.charSelection)
	if err != nil {
		m.charErrMsg = err.Error()
		return
	}
	m.charPerSession = perSession
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sessionIDs(sessions []model.SessionAggregate) []int64 {
	ids := make([]int64, len(sessions))
	for i, s := range sessions {
		ids[i] = s.SessionID
	}
	return ids
}

func parseChars(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	if strings.Contains(input, ",") {
		parts := strings.Split(input, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			out = append(out, part)
		}
		return out
	}
	out := make([]string, 0, len([]rune(input)))
	for _, r := range input {
		out = append(out, string(r))
	}
	return out
}

func parseRawChars(input string) []string {
	out := make([]string, 0, len([]rune(input)))
	for _, r := range input {
		if unicode.IsSpace(r) {
			continue
		}
		out = append(out, string(r))
	}
	return out
}

func normalizeCharInput(input string) string {
	if input == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range input {
		if r == ',' || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func nextCurveWindow(n int) int {
	if n < 5 {
		return 5
	}
	if n%5 == 0 {
		return n + 5
	}
	return ((n / 5) + 1) * 5
}

func prevCurveWindow(n int) int {
	if n <= 5 {
		return 1
	}
	if n%5 == 0 {
		return n - 5
	}
	return (n / 5) * 5
}

func modalWidth(width int) int {
	return maxInt(40, minInt(width-4, 80))
}

func modalInnerWidth(width int) int {
	w := modalWidth(width)
	w -= 6 // 2 border + 4 padding
	if w < 10 {
		return 10
	}
	return w
}

func sortCharAggsByTotal(aggs []model.CharAggregate) []model.CharAggregate {
	out := append([]model.CharAggregate(nil), aggs...)
	sort.Slice(out, func(i, j int) bool {
		totalI := out[i].Correct + out[i].Incorrect
		totalJ := out[j].Correct + out[j].Incorrect
		if totalI == totalJ {
			return out[i].Char < out[j].Char
		}
		return totalI > totalJ
	})
	return out
}

func padLines(s string, width int) string {
	if width <= 0 || s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = padLine(line, width)
	}
	return strings.Join(lines, "\n")
}

func padLine(line string, width int) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth < width {
		return line + strings.Repeat(" ", width-lineWidth)
	}
	return line
}

func fitLines(s string, width, height int) string {
	if width <= 0 || height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = padLine(line, width)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func truncateLine(s string, width int) string {
	if width <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
