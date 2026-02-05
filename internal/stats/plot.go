// Package stats contains statistics calculations and reporting.
package stats

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// Series represents a named data series for plotting.
type Series struct {
	Name   string
	Values []float64
}

type seriesMinMaxRange struct {
	min float64
	max float64
}

type lineStyle struct {
	name   string
	period int
	on     int
}

type ansiColor struct {
	name string
	code string
}

const (
	defaultPlotHeight   = 10
	minPlotWidth        = 10
	axisLabelTop        = "100%"
	axisLabelMid        = "50%"
	axisLabelBottom     = "0%"
	axisSeparator       = " │ "
	scaleNote           = "Scaled per series; see min/max below."
	colorReset          = "\x1b[0m"
	terminalWidthBackup = 80
)

var lineStyles = []lineStyle{
	{name: "solid", period: 1, on: 1},
	{name: "dashed", period: 6, on: 3},
	{name: "dotted", period: 4, on: 1},
	{name: "dashdot", period: 8, on: 3},
}

var colorPalette = []ansiColor{
	{name: "cyan", code: "\x1b[36m"},
	{name: "magenta", code: "\x1b[35m"},
	{name: "yellow", code: "\x1b[33m"},
	{name: "green", code: "\x1b[32m"},
	{name: "blue", code: "\x1b[34m"},
}

// PlotSeries renders a multi-line text plot for the provided series.
func PlotSeries(w io.Writer, title string, series []Series, width, height int) error {
	return plotSeries(w, title, series, width, height, false)
}

// PlotSeriesWithColor renders a multi-line text plot with optional forced color output.
func PlotSeriesWithColor(w io.Writer, title string, series []Series, width, height int, forceColor bool) error {
	return plotSeries(w, title, series, width, height, forceColor)
}

func plotSeries(w io.Writer, title string, series []Series, width, height int, forceColor bool) error {
	series = filterSeries(series)
	if len(series) == 0 {
		return nil
	}

	maxLen := maxSeriesLen(series)
	if maxLen == 0 {
		return nil
	}

	if height <= 0 {
		height = defaultPlotHeight
	}
	if width <= 0 {
		width = autoPlotWidth()
	}
	if width < minPlotWidth {
		width = minPlotWidth
	}
	if width < 1 {
		width = 1
	}

	scaled := make([]Series, 0, len(series))
	for _, s := range series {
		scaled = append(scaled, Series{
			Name:   s.Name,
			Values: resampleSeries(s.Values, width),
		})
	}

	minMax := make([]seriesMinMaxRange, 0, len(scaled))
	for _, s := range scaled {
		minVal, maxVal := seriesMinMaxSingle(s.Values)
		if math.Abs(maxVal-minVal) < 1e-9 {
			minVal--
			maxVal++
		}
		minMax = append(minMax, seriesMinMaxRange{min: minVal, max: maxVal})
	}

	seriesCells := make([][][]uint8, 0, len(scaled))
	for i := 0; i < len(scaled); i++ {
		seriesCells = append(seriesCells, makeCells(height, width))
	}
	for si, s := range scaled {
		if len(s.Values) == 0 {
			continue
		}
		style := lineStyles[si%len(lineStyles)]
		prevX, prevY := -1, -1
		for x, v := range s.Values {
			row := valueToRow(v, minMax[si].min, minMax[si].max, height*4)
			if row < 0 {
				row = 0
			}
			if row >= height*4 {
				row = height*4 - 1
			}
			px := x * 2
			py := row
			if prevX >= 0 {
				drawLine(prevX, prevY, px, py, func(dx, dy int) {
					if style.shouldPlot(dx) {
						setBrailleDot(seriesCells[si], dx, dy)
					}
				})
			} else if style.shouldPlot(px) {
				setBrailleDot(seriesCells[si], px, py)
			}
			prevX, prevY = px, py
		}
	}

	useColor := shouldUseColor(w, forceColor)
	leftAxisWidth := len(axisLabelTop)
	axisLabels := makeAxisLabels(height)

	if title != "" {
		if _, err := fmt.Fprintln(w, title); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, scaleNote); err != nil {
		return err
	}
	for i, s := range scaled {
		if _, err := fmt.Fprintf(w, "%s: min=%.2f max=%.2f\n", s.Name, minMax[i].min, minMax[i].max); err != nil {
			return err
		}
	}
	for y := 0; y < height; y++ {
		prefix := fmt.Sprintf("%*s%s", leftAxisWidth, axisLabels[y], axisSeparator)
		var row strings.Builder
		row.WriteString(prefix)
		for x := 0; x < width; x++ {
			mask, colorIdx := composeCell(seriesCells, x, y)
			ch := brailleFromMask(mask)
			if useColor && colorIdx >= 0 {
				color := colorPalette[colorIdx%len(colorPalette)].code
				row.WriteString(color)
				row.WriteRune(ch)
				row.WriteString(colorReset)
			} else {
				row.WriteRune(ch)
			}
		}
		if _, err := fmt.Fprintln(w, row.String()); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, renderLegend(scaled, useColor)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	return nil
}

func filterSeries(series []Series) []Series {
	out := make([]Series, 0, len(series))
	for _, s := range series {
		if len(s.Values) == 0 {
			continue
		}
		out = append(out, s)
	}
	return out
}

func maxSeriesLen(series []Series) int {
	maxLen := 0
	for _, s := range series {
		if len(s.Values) > maxLen {
			maxLen = len(s.Values)
		}
	}
	return maxLen
}

func autoPlotWidth() int {
	return PlotWidthFor(terminalWidth())
}

// PlotWidthFor computes a plot width that fits within the total available width.
func PlotWidthFor(totalWidth int) int {
	if totalWidth <= 0 {
		return minPlotWidth
	}
	axisWidth := utf8.RuneCountInString(axisLabelTop) + utf8.RuneCountInString(axisSeparator)
	plotWidth := totalWidth - axisWidth
	if plotWidth < minPlotWidth {
		plotWidth = minPlotWidth
	}
	if plotWidth < 1 {
		plotWidth = 1
	}
	return plotWidth
}

func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return terminalWidthBackup
	}
	return width
}

func shouldUseColor(w io.Writer, force bool) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if force {
		return true
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func makeAxisLabels(height int) []string {
	labels := make([]string, height)
	if height <= 0 {
		return labels
	}
	labels[0] = axisLabelTop
	if height > 2 {
		labels[height/2] = axisLabelMid
	}
	if height > 1 {
		labels[height-1] = axisLabelBottom
	}
	return labels
}

func makeCells(height, width int) [][]uint8 {
	cells := make([][]uint8, height)
	for y := 0; y < height; y++ {
		cells[y] = make([]uint8, width)
	}
	return cells
}

func composeCell(seriesCells [][][]uint8, x, y int) (uint8, int) {
	var mask uint8
	colorIdx := -1
	for i, cells := range seriesCells {
		if y < 0 || y >= len(cells) {
			continue
		}
		if x < 0 || x >= len(cells[y]) {
			continue
		}
		cellMask := cells[y][x]
		if cellMask == 0 {
			continue
		}
		if colorIdx == -1 {
			colorIdx = i
		}
		mask |= cellMask
	}
	return mask, colorIdx
}

func (ls lineStyle) shouldPlot(x int) bool {
	if ls.period <= 1 {
		return true
	}
	if x < 0 {
		x = -x
	}
	return x%ls.period < ls.on
}

func resampleSeries(values []float64, width int) []float64 {
	if len(values) == 0 || width <= 0 {
		return nil
	}
	if len(values) == width {
		out := make([]float64, len(values))
		copy(out, values)
		return out
	}
	out := make([]float64, width)
	if len(values) > width {
		for i := 0; i < width; i++ {
			start := int(float64(i) * float64(len(values)) / float64(width))
			end := int(float64(i+1) * float64(len(values)) / float64(width))
			if end <= start {
				end = start + 1
			}
			if end > len(values) {
				end = len(values)
			}
			var sum float64
			for _, v := range values[start:end] {
				sum += v
			}
			out[i] = sum / float64(end-start)
		}
		return out
	}
	if width == 1 {
		out[0] = values[0]
		return out
	}
	if len(values) == 1 {
		for i := range out {
			out[i] = values[0]
		}
		return out
	}
	for i := 0; i < width; i++ {
		pos := float64(i) * float64(len(values)-1) / float64(width-1)
		idx := int(math.Floor(pos))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(values)-1 {
			out[i] = values[len(values)-1]
			continue
		}
		frac := pos - float64(idx)
		out[i] = values[idx]*(1-frac) + values[idx+1]*frac
	}
	return out
}

func seriesMinMaxSingle(values []float64) (float64, float64) {
	minVal := math.Inf(1)
	maxVal := math.Inf(-1)
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	if minVal == math.Inf(1) {
		minVal = 0
	}
	if maxVal == math.Inf(-1) {
		maxVal = 0
	}
	return minVal, maxVal
}

func valueToRow(v, minVal, maxVal float64, height int) int {
	if height <= 1 {
		return 0
	}
	pos := (v - minVal) / (maxVal - minVal)
	row := int(math.Round((1 - pos) * float64(height-1)))
	if row < 0 {
		row = 0
	}
	if row >= height {
		row = height - 1
	}
	return row
}

func renderLegend(series []Series, useColor bool) string {
	parts := make([]string, 0, len(series))
	marker := brailleFromMask(0x01)
	for i, s := range series {
		styleName := lineStyles[i%len(lineStyles)].name
		label := fmt.Sprintf("%c %s (%s)", marker, s.Name, styleName)
		if useColor {
			color := colorPalette[i%len(colorPalette)].code
			label = color + label + colorReset
		}
		parts = append(parts, label)
	}
	return "Legend: " + strings.Join(parts, "  ")
}

func drawLine(x0, y0, x1, y1 int, plot func(x, y int)) {
	dx := int(math.Abs(float64(x1 - x0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -int(math.Abs(float64(y1 - y0)))
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		plot(x0, y0)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			if x0 == x1 {
				break
			}
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			if y0 == y1 {
				break
			}
			err += dx
			y0 += sy
		}
	}
}

func setBrailleDot(cells [][]uint8, x, y int) {
	if y < 0 || x < 0 {
		return
	}
	cellY := y / 4
	cellX := x / 2
	if cellY < 0 || cellY >= len(cells) {
		return
	}
	if cellX < 0 || cellX >= len(cells[cellY]) {
		return
	}
	dotMask := brailleDotMask(x%2, y%4)
	cells[cellY][cellX] |= dotMask
}

func brailleDotMask(x, y int) uint8 {
	switch {
	case x == 0 && y == 0:
		return 0x01
	case x == 0 && y == 1:
		return 0x02
	case x == 0 && y == 2:
		return 0x04
	case x == 0 && y == 3:
		return 0x40
	case x == 1 && y == 0:
		return 0x08
	case x == 1 && y == 1:
		return 0x10
	case x == 1 && y == 2:
		return 0x20
	case x == 1 && y == 3:
		return 0x80
	default:
		return 0
	}
}

func brailleFromMask(mask uint8) rune {
	return rune(0x2800 + int(mask))
}
