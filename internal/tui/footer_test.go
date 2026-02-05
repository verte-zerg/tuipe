package tui

import (
	"strings"
	"testing"
)

func TestRenderFooterFormats(t *testing.T) {
	m := &Model{
		targetRunes: []rune("abcd"),
		inputRunes:  []rune("ab"),
		hasLast:     true,
		lastWPM:     72.4,
		lastAcc:     0.978,
		allWPM:      68.1,
		allAcc:      0.969,
	}
	out := m.renderFooter()
	if out == "" {
		t.Fatalf("expected footer output")
	}
	if !containsAll(out, []string{"Progress 50%", "Last 72.4 WPM", "97.8%", "All-time 68.1 WPM", "96.9%"}) {
		t.Fatalf("footer missing expected segments: %s", out)
	}
}

func containsAll(haystack string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
