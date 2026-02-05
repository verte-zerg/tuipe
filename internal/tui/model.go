// Package tui provides the Bubble Tea typing interface.
package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/verte-zerg/tuipe/internal/generator"
	"github.com/verte-zerg/tuipe/internal/model"
	statsPkg "github.com/verte-zerg/tuipe/internal/stats"
	"github.com/verte-zerg/tuipe/internal/store"
)

type charStat struct {
	correct      int
	incorrect    int
	latencySumMs int64
	latencyCount int64
}

// Model implements the Bubble Tea typing UI.
type Model struct {
	config            model.Config
	store             *store.Store
	gen               *generator.Generator
	words             []string
	wordListPath      string
	punctSet          []rune
	weakSet           map[rune]struct{}
	weakNoticePrinted bool

	width  int
	height int

	targetRunes []rune
	inputRunes  []rune

	started       bool
	startedAt     time.Time
	prevCorrectAt time.Time

	correctNonSpace   int
	incorrectNonSpace int
	charStats         map[rune]*charStat

	lastWPM float64
	lastAcc float64
	hasLast bool

	allWPM       float64
	allAcc       float64
	allCorrect   int
	allIncorrect int
	allDuration  int64
}

var (
	correctStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0"))
	incorrectStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4D4F"))
	pendingStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8C8C8C"))
	currentWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#C89A3A"))
	cursorStyle      = pendingStyle.Copy().Underline(true)
	footerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E6E6E"))
)

// NewModel constructs a typing TUI model.
func NewModel(cfg model.Config, store *store.Store, gen *generator.Generator, words []string, wordListPath string, punctSet []rune, weakSet map[rune]struct{}, weakNoticePrinted bool) *Model {
	m := &Model{
		config:            cfg,
		store:             store,
		gen:               gen,
		words:             words,
		wordListPath:      wordListPath,
		punctSet:          punctSet,
		weakSet:           weakSet,
		weakNoticePrinted: weakNoticePrinted,
	}
	m.resetSession()
	m.loadFooterStats()
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
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyBackspace, tea.KeyDelete:
			m.handleBackspace()
			return m, nil
		case tea.KeySpace:
			m.handleRunes([]rune{' '})
			return m, nil
		case tea.KeyRunes:
			m.handleRunes(msg.Runes)
			return m, nil
		default:
			return m, nil
		}
	default:
		return m, nil
	}
}

// View implements tea.Model.
func (m *Model) View() string {
	if len(m.targetRunes) == 0 {
		return ""
	}
	cursorIndex := -1
	if len(m.inputRunes) < len(m.targetRunes) {
		cursorIndex = len(m.inputRunes)
	}
	styledRunes := buildStyledRunes(m.targetRunes, m.inputRunes, cursorIndex)
	if m.width == 0 || m.height == 0 {
		return renderStyledRunes(styledRunes)
	}
	contentWidth := int(float64(m.width) * 0.70)
	if contentWidth < 1 {
		contentWidth = 1
	}
	wrapped := wrapStyledRunes(styledRunes, contentWidth)
	content := lipgloss.NewStyle().Width(contentWidth).Render(wrapped)
	footer := m.renderFooter()
	if footer == "" || m.height < 3 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}
	bodyHeight := m.height - 1
	body := lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, content)
	footerLine := lipgloss.Place(m.width, 1, lipgloss.Center, lipgloss.Center, footer)
	return body + "\n" + footerLine
}

func (m *Model) handleBackspace() {
	if len(m.inputRunes) == 0 {
		return
	}
	m.inputRunes = m.inputRunes[:len(m.inputRunes)-1]
}

func (m *Model) handleRunes(runes []rune) {
	for _, r := range runes {
		if len(m.inputRunes) >= len(m.targetRunes) {
			return
		}
		if !m.started {
			m.started = true
			m.startedAt = time.Now()
		}
		pos := len(m.inputRunes)
		expected := m.targetRunes[pos]
		m.inputRunes = append(m.inputRunes, r)
		m.updateStats(expected, r)
		if len(m.inputRunes) == len(m.targetRunes) {
			m.finishSession()
			m.resetSession()
		}
	}
}

func (m *Model) loadFooterStats() {
	ctx := context.Background()
	sessions, err := m.store.ListSessions(ctx, model.StatsConfig{Lang: m.config.Lang})
	if err != nil {
		logErrf("failed to load session stats: %v\n", err)
		return
	}
	if len(sessions) == 0 {
		return
	}
	last := sessions[len(sessions)-1]
	wpm, _, acc := statsPkg.SessionMetrics(last.Correct, last.Incorrect, last.DurationMs)
	m.lastWPM = wpm
	m.lastAcc = acc
	m.hasLast = true

	for _, s := range sessions {
		m.allCorrect += s.Correct
		m.allIncorrect += s.Incorrect
		m.allDuration += s.DurationMs
	}
	m.recomputeAllTime()
}

func (m *Model) recomputeAllTime() {
	wpm, _, acc := statsPkg.SessionMetrics(m.allCorrect, m.allIncorrect, m.allDuration)
	m.allWPM = wpm
	m.allAcc = acc
}

func (m *Model) renderFooter() string {
	if len(m.targetRunes) == 0 {
		return ""
	}
	progress := 0
	if len(m.targetRunes) > 0 {
		progress = int(float64(len(m.inputRunes)) / float64(len(m.targetRunes)) * 100)
	}
	segments := []string{fmt.Sprintf("Progress %d%%", progress)}
	if m.hasLast {
		segments = append(segments, fmt.Sprintf("Last %.1f WPM · %.1f%%", m.lastWPM, m.lastAcc*100))
	}
	segments = append(segments, fmt.Sprintf("All-time %.1f WPM · %.1f%%", m.allWPM, m.allAcc*100))
	footer := strings.Join(segments, "  ")
	return footerStyle.Render(footer)
}

func (m *Model) updateStats(expected, typed rune) {
	if expected == ' ' {
		return
	}
	entry := m.charEntry(expected)
	if typed == expected {
		m.correctNonSpace++
		entry.correct++
		now := time.Now()
		if !m.prevCorrectAt.IsZero() {
			delta := now.Sub(m.prevCorrectAt)
			entry.latencySumMs += delta.Milliseconds()
			entry.latencyCount++
		}
		m.prevCorrectAt = now
		return
	}
	m.incorrectNonSpace++
	entry.incorrect++
}

func (m *Model) charEntry(expected rune) *charStat {
	if m.charStats == nil {
		m.charStats = map[rune]*charStat{}
	}
	entry, ok := m.charStats[expected]
	if !ok {
		entry = &charStat{}
		m.charStats[expected] = entry
	}
	return entry
}

func (m *Model) resetSession() {
	m.inputRunes = nil
	m.started = false
	m.startedAt = time.Time{}
	m.prevCorrectAt = time.Time{}
	m.correctNonSpace = 0
	m.incorrectNonSpace = 0
	m.charStats = map[rune]*charStat{}

	text := m.generateText()
	m.targetRunes = []rune(text)
}

func (m *Model) generateText() string {
	var words []string
	if m.config.FocusWeak && len(m.weakSet) > 0 {
		words = m.gen.GenerateWeighted(m.words, m.config.Words, m.config.CapsPct, m.config.PunctPct, m.punctSet, m.weakSet, m.config.WeakFactor)
	} else {
		words = m.gen.Generate(m.words, m.config.Words, m.config.CapsPct, m.config.PunctPct, m.punctSet)
	}
	return strings.Join(words, " ")
}

func (m *Model) finishSession() {
	if !m.started {
		return
	}
	endedAt := time.Now()
	stats := model.SessionStats{
		StartedAt:         m.startedAt,
		EndedAt:           endedAt,
		Lang:              m.config.Lang,
		Words:             m.config.Words,
		CapsPct:           m.config.CapsPct,
		PunctPct:          m.config.PunctPct,
		PunctSet:          m.config.PunctSet,
		WordListPath:      m.wordListPath,
		CorrectNonSpace:   m.correctNonSpace,
		IncorrectNonSpace: m.incorrectNonSpace,
		DurationMs:        endedAt.Sub(m.startedAt).Milliseconds(),
	}

	charStats := make([]model.CharStats, 0, len(m.charStats))
	for ch, entry := range m.charStats {
		charStats = append(charStats, model.CharStats{
			Char:         string(ch),
			Correct:      entry.correct,
			Incorrect:    entry.incorrect,
			LatencySumMs: entry.latencySumMs,
			LatencyCount: entry.latencyCount,
		})
	}

	ctx := context.Background()
	if _, err := m.store.InsertSession(ctx, stats, charStats); err != nil {
		logErrf("failed to save session: %v\n", err)
	}
	wpm, _, acc := statsPkg.SessionMetrics(stats.CorrectNonSpace, stats.IncorrectNonSpace, stats.DurationMs)
	m.lastWPM = wpm
	m.lastAcc = acc
	m.hasLast = true
	m.allCorrect += stats.CorrectNonSpace
	m.allIncorrect += stats.IncorrectNonSpace
	m.allDuration += stats.DurationMs
	m.recomputeAllTime()

	if m.config.FocusWeak {
		m.refreshWeakSet()
	}
}

func (m *Model) refreshWeakSet() {
	ctx := context.Background()
	aggs, err := m.store.GetWeakChars(ctx, m.config.WeakWindow, m.config.Lang)
	if err != nil {
		logErrf("failed to load weak chars: %v\n", err)
		return
	}
	if len(aggs) == 0 {
		if !m.weakNoticePrinted {
			logErrln("no stats available for weak-char focus yet; using normal generator")
			m.weakNoticePrinted = true
		}
		m.weakSet = map[rune]struct{}{}
		return
	}
	m.weakSet = statsPkg.SelectWeakChars(aggs, m.config.WeakTop)
}

func logErrf(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, format, args...); err != nil {
		// Best-effort logging to stderr.
		_ = err
	}
}

func logErrln(args ...any) {
	if _, err := fmt.Fprintln(os.Stderr, args...); err != nil {
		// Best-effort logging to stderr.
		_ = err
	}
}
