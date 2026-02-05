package stats

import "testing"

func TestFormatTableAlignsColumns(t *testing.T) {
	headers := []string{"Char", "Accuracy", "Correct"}
	rows := [][]string{
		{"a", "97.50%", "12"},
		{"<space>", "8.00%", "3"},
	}
	rightAlign := map[int]bool{1: true, 2: true}

	lines := formatTable(headers, rows, rightAlign)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "Char    Accuracy Correct" {
		t.Fatalf("unexpected header line: %q", lines[0])
	}
	if lines[1] != "a         97.50%      12" {
		t.Fatalf("unexpected row line: %q", lines[1])
	}
	if lines[2] != "<space>    8.00%       3" {
		t.Fatalf("unexpected row line: %q", lines[2])
	}
}
