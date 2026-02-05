package stats

import (
	"testing"
	"unicode/utf8"
)

func TestPlotWidthFor(t *testing.T) {
	axisWidth := utf8.RuneCountInString(axisLabelTop) + utf8.RuneCountInString(axisSeparator)
	total := 80
	expected := total - axisWidth
	if expected < minPlotWidth {
		expected = minPlotWidth
	}
	if got := PlotWidthFor(total); got != expected {
		t.Fatalf("expected width %d, got %d", expected, got)
	}
	if got := PlotWidthFor(0); got != minPlotWidth {
		t.Fatalf("expected min width %d, got %d", minPlotWidth, got)
	}
}
