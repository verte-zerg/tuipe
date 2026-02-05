package tui

import "testing"

func TestBuildStyledRunesCursor(t *testing.T) {
	target := []rune("ab")
	input := []rune("a")
	cursorIndex := len(input)

	runes := buildStyledRunes(target, input, cursorIndex)
	if len(runes) != 2 {
		t.Fatalf("expected 2 runes, got %d", len(runes))
	}
	if runes[0].s != correctStyle.Render("a") {
		t.Fatalf("expected correct style for first rune")
	}
	if runes[1].s != cursorStyle.Render("b") {
		t.Fatalf("expected cursor style for second rune")
	}
}

func TestBuildStyledRunesNoCursorWhenComplete(t *testing.T) {
	target := []rune("a")
	input := []rune("a")
	cursorIndex := -1

	runes := buildStyledRunes(target, input, cursorIndex)
	if len(runes) != 1 {
		t.Fatalf("expected 1 rune, got %d", len(runes))
	}
	if runes[0].s != correctStyle.Render("a") {
		t.Fatalf("expected correct style for completed rune")
	}
}

func TestBuildStyledRunesKeepsTargetOnMistype(t *testing.T) {
	target := []rune("ab")
	input := []rune("ax")
	cursorIndex := len(input)

	runes := buildStyledRunes(target, input, cursorIndex)
	if len(runes) != 2 {
		t.Fatalf("expected 2 runes, got %d", len(runes))
	}
	if runes[0].s != correctStyle.Render("a") {
		t.Fatalf("expected correct style for first rune")
	}
	if runes[1].s != incorrectStyle.Render("b") {
		t.Fatalf("expected incorrect style for second rune")
	}
}

func TestBuildStyledRunesWordHighlighting(t *testing.T) {
	target := []rune("one two")
	input := []rune("o")
	cursorIndex := len(input)

	runes := buildStyledRunes(target, input, cursorIndex)
	if runes[0].s != correctStyle.Render("o") {
		t.Fatalf("expected correct style for typed rune")
	}
	if runes[1].s != currentWordStyle.Render("n") {
		t.Fatalf("expected current word style for untyped in current word")
	}
	if runes[2].s != currentWordStyle.Render("e") {
		t.Fatalf("expected current word style for untyped in current word")
	}
	if runes[4].s != pendingStyle.Render("t") {
		t.Fatalf("expected pending style for next word")
	}
	if runes[6].s != pendingStyle.Render("o") {
		t.Fatalf("expected pending style for next word")
	}
}

func TestBuildStyledRunesWrongSpaceDot(t *testing.T) {
	target := []rune("a b")
	input := []rune("ax")
	cursorIndex := len(input)

	runes := buildStyledRunes(target, input, cursorIndex)
	if len(runes) != 3 {
		t.Fatalf("expected 3 runes, got %d", len(runes))
	}
	if runes[1].s != incorrectStyle.Render("•") {
		t.Fatalf("expected red dot for wrong space")
	}
}
