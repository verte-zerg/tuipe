package stats

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlotSeries(t *testing.T) {
	var buf bytes.Buffer
	err := PlotSeries(&buf, "Test Plot", []Series{
		{Name: "A", Values: []float64{1, 2, 3, 2, 1}},
		{Name: "B", Values: []float64{1, 1, 2, 3, 4}},
	}, 5, 4)
	if err != nil {
		t.Fatalf("PlotSeries failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Test Plot") {
		t.Fatalf("expected title in output")
	}
	if !strings.Contains(out, "Scaled per series") {
		t.Fatalf("expected scale note in output")
	}
	if !strings.Contains(out, "Legend:") {
		t.Fatalf("expected legend in output")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	expectedMin := 1 + 1 + 2 + 4 + 1
	if len(lines) < expectedMin {
		t.Fatalf("expected at least %d lines of output, got %d", expectedMin, len(lines))
	}
}
