// Package stats contains statistics calculations and reporting.
package stats

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/verte-zerg/tuipe/internal/model"
)

const sparkChars = " .:-=+*#%@"

// SessionMetrics computes WPM, CPM, and accuracy for a session.
func SessionMetrics(correct, incorrect int, durationMs int64) (wpm, cpm, accuracy float64) {
	if durationMs <= 0 {
		return 0, 0, 0
	}
	minutes := float64(durationMs) / 60000.0
	if minutes <= 0 {
		return 0, 0, 0
	}
	wpm = (float64(correct) / 5.0) / minutes
	cpm = float64(correct) / minutes
	den := float64(correct + incorrect)
	if den > 0 {
		accuracy = float64(correct) / den
	}
	return wpm, cpm, accuracy
}

// MovingAverage computes a rolling mean over the provided window size.
func MovingAverage(values []float64, window int) []float64 {
	if window <= 1 || len(values) == 0 {
		out := make([]float64, len(values))
		copy(out, values)
		return out
	}
	out := make([]float64, len(values))
	var sum float64
	for i := 0; i < len(values); i++ {
		sum += values[i]
		if i >= window {
			sum -= values[i-window]
		}
		den := float64(i + 1)
		if i >= window {
			den = float64(window)
		}
		out[i] = sum / den
	}
	return out
}

// Sparkline renders a single-line ASCII sparkline for the values.
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	minVal := values[0]
	maxVal := values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	if math.Abs(maxVal-minVal) < 1e-9 {
		return strings.Repeat(string(sparkChars[len(sparkChars)/2]), len(values))
	}
	var b strings.Builder
	for _, v := range values {
		pos := (v - minVal) / (maxVal - minVal)
		idx := int(math.Round(pos * float64(len(sparkChars)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		b.WriteByte(sparkChars[idx])
	}
	return b.String()
}

// RenderSummary prints a summary table for sessions.
func RenderSummary(w io.Writer, sessions []model.SessionAggregate) error {
	if len(sessions) == 0 {
		_, err := fmt.Fprintln(w, "No sessions found.")
		return err
	}
	var totalWPM, totalCPM, totalAcc float64
	bestWPM := 0.0
	for _, s := range sessions {
		wpm, cpm, acc := SessionMetrics(s.Correct, s.Incorrect, s.DurationMs)
		totalWPM += wpm
		totalCPM += cpm
		totalAcc += acc
		if wpm > bestWPM {
			bestWPM = wpm
		}
	}
	count := float64(len(sessions))
	if _, err := fmt.Fprintln(w, "Summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Sessions: %d\n", len(sessions)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg WPM: %.2f\n", totalWPM/count); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Best WPM: %.2f\n", bestWPM); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg CPM: %.2f\n", totalCPM/count); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg Accuracy: %.2f%%\n", (totalAcc/count)*100); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	return nil
}

// RenderCurves prints learning curves for WPM and accuracy.
func RenderCurves(w io.Writer, sessions []model.SessionAggregate, window int) error {
	return RenderCurvesWithSize(w, sessions, window, 0, 10, false)
}

// RenderCurvesWithSize prints learning curves sized to a given total width.
func RenderCurvesWithSize(w io.Writer, sessions []model.SessionAggregate, window, totalWidth, height int, useColor bool) error {
	if len(sessions) == 0 {
		return nil
	}
	wpms := make([]float64, len(sessions))
	accs := make([]float64, len(sessions))
	for i, s := range sessions {
		wpm, _, acc := SessionMetrics(s.Correct, s.Incorrect, s.DurationMs)
		wpms[i] = wpm
		accs[i] = acc * 100
	}
	wpms = MovingAverage(wpms, window)
	accs = MovingAverage(accs, window)

	width := 0
	if totalWidth > 0 {
		width = PlotWidthFor(totalWidth)
	}
	return PlotSeriesWithColor(w, "Learning Curves", []Series{
		{Name: "WPM", Values: wpms},
		{Name: "Accuracy", Values: accs},
	}, width, height, useColor)
}

// RenderCharTable prints per-character aggregates.
func RenderCharTable(w io.Writer, aggs []model.CharAggregate) error {
	if len(aggs) == 0 {
		_, err := fmt.Fprintln(w, "No character stats found.")
		return err
	}
	type row struct {
		char      string
		acc       float64
		latency   float64
		correct   int
		incorrect int
	}
	rows := make([]row, 0, len(aggs))
	for _, agg := range aggs {
		charLabel := agg.Char
		if charLabel == " " {
			charLabel = "<space>"
		}
		total := agg.Correct + agg.Incorrect
		acc := 0.0
		if total > 0 {
			acc = float64(agg.Correct) / float64(total)
		}
		lat := 0.0
		if agg.LatencyCount > 0 {
			lat = float64(agg.LatencySumMs) / float64(agg.LatencyCount)
		}
		rows = append(rows, row{
			char:      charLabel,
			acc:       acc,
			latency:   lat,
			correct:   agg.Correct,
			incorrect: agg.Incorrect,
		})
	}
	// Sort by lowest accuracy.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].acc == rows[j].acc {
			return rows[i].char < rows[j].char
		}
		return rows[i].acc < rows[j].acc
	})

	if _, err := fmt.Fprintln(w, "Per-Character (Windowed)"); err != nil {
		return err
	}

	headers := []string{"Char", "Accuracy", "Avg Latency (ms)", "Correct", "Incorrect"}
	tableRows := make([][]string, 0, len(rows))
	for _, r := range rows {
		tableRows = append(tableRows, []string{
			r.char,
			fmt.Sprintf("%.2f%%", r.acc*100),
			fmt.Sprintf("%.1f", r.latency),
			fmt.Sprintf("%d", r.correct),
			fmt.Sprintf("%d", r.incorrect),
		})
	}
	rightAlign := map[int]bool{1: true, 2: true, 3: true, 4: true}
	lines := formatTable(headers, tableRows, rightAlign)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	return nil
}

// RenderCharCurves prints per-character learning curves.
func RenderCharCurves(w io.Writer, sessions []model.SessionAggregate, perSession map[int64]map[string]model.CharAggregate, chars []string, window int) error {
	return RenderCharCurvesWithSize(w, sessions, perSession, chars, window, 0, 10, false)
}

// RenderCharCurvesWithSize prints per-character learning curves sized to a given total width.
func RenderCharCurvesWithSize(w io.Writer, sessions []model.SessionAggregate, perSession map[int64]map[string]model.CharAggregate, chars []string, window, totalWidth, height int, useColor bool) error {
	if len(chars) == 0 || len(sessions) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "Per-Character Curves"); err != nil {
		return err
	}
	for _, ch := range chars {
		accSeries := make([]float64, len(sessions))
		latSeries := make([]float64, len(sessions))
		for i, s := range sessions {
			if data, ok := perSession[s.SessionID]; ok {
				if agg, ok := data[ch]; ok {
					total := agg.Correct + agg.Incorrect
					if total > 0 {
						accSeries[i] = float64(agg.Correct) / float64(total) * 100
					}
					if agg.LatencyCount > 0 {
						latSeries[i] = float64(agg.LatencySumMs) / float64(agg.LatencyCount)
					}
				}
			}
		}
		accSeries = MovingAverage(accSeries, window)
		latSeries = MovingAverage(latSeries, window)
		width := 0
		if totalWidth > 0 {
			width = PlotWidthFor(totalWidth)
		}
		if err := PlotSeriesWithColor(w, fmt.Sprintf("Char %s", ch), []Series{
			{Name: "Accuracy", Values: accSeries},
			{Name: "Latency", Values: latSeries},
		}, width, height, useColor); err != nil {
			return err
		}
	}
	return nil
}
