// Package tui provides the Bubble Tea typing interface.
package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

type styledRune struct {
	s       string
	width   int
	isSpace bool
}

func buildStyledRunes(targetRunes, inputRunes []rune, cursorIndex int) []styledRune {
	words := findWords(targetRunes)
	currentWord := wordForCursor(words, cursorIndex)

	out := make([]styledRune, 0, len(targetRunes))
	for i, target := range targetRunes {
		displayed := target
		style := pendingStyle
		typed := i < len(inputRunes)
		if typed {
			switch {
			case target == ' ' && inputRunes[i] != ' ':
				displayed = '•'
				style = incorrectStyle
			case inputRunes[i] == target:
				style = correctStyle
			default:
				style = incorrectStyle
			}
		} else if target != ' ' {
			if currentWord != nil && i >= currentWord.start && i < currentWord.end {
				style = currentWordStyle
			} else {
				style = pendingStyle
			}
		}
		if i == cursorIndex && i >= len(inputRunes) {
			style = style.Underline(true)
		}
		out = append(out, styledRune{
			s:       style.Render(string(displayed)),
			width:   runewidth.RuneWidth(displayed),
			isSpace: target == ' ',
		})
	}
	return out
}

type wordRange struct {
	start int
	end   int
}

func findWords(targetRunes []rune) []wordRange {
	words := []wordRange{}
	start := -1
	for i, r := range targetRunes {
		if r == ' ' {
			if start != -1 {
				words = append(words, wordRange{start: start, end: i})
				start = -1
			}
			continue
		}
		if start == -1 {
			start = i
		}
	}
	if start != -1 {
		words = append(words, wordRange{start: start, end: len(targetRunes)})
	}
	return words
}

func wordForCursor(words []wordRange, cursorIndex int) *wordRange {
	if len(words) == 0 {
		return nil
	}
	if cursorIndex < 0 {
		return &words[0]
	}
	wordIdx := -1
	for i, w := range words {
		if cursorIndex >= w.start && cursorIndex < w.end {
			wordIdx = i
			break
		}
		if cursorIndex < w.start {
			wordIdx = i
			break
		}
	}
	if wordIdx == -1 {
		return &words[len(words)-1]
	}
	return &words[wordIdx]
}

func renderStyledRunes(runes []styledRune) string {
	var b strings.Builder
	for _, item := range runes {
		b.WriteString(item.s)
	}
	return b.String()
}

func wrapStyledRunes(runes []styledRune, width int) string {
	if width <= 0 {
		return renderStyledRunes(runes)
	}
	var out strings.Builder
	line := make([]styledRune, 0, len(runes))
	lineWidth := 0
	lastSpaceIdx := -1

	for i := 0; i < len(runes); {
		item := runes[i]
		if lineWidth+item.width > width && len(line) > 0 {
			if lastSpaceIdx >= 0 {
				out.WriteString(renderStyledRunes(line[:lastSpaceIdx]))
				out.WriteRune('\n')
				line = append([]styledRune{}, line[lastSpaceIdx+1:]...)
				lineWidth = lineWidthOf(line)
				lastSpaceIdx = lastSpaceIndex(line)
			} else {
				out.WriteString(renderStyledRunes(line))
				out.WriteRune('\n')
				line = line[:0]
				lineWidth = 0
				lastSpaceIdx = -1
			}
			continue
		}
		line = append(line, item)
		lineWidth += item.width
		if item.isSpace {
			lastSpaceIdx = len(line) - 1
		}
		i++
	}
	out.WriteString(renderStyledRunes(line))
	return out.String()
}

func lineWidthOf(line []styledRune) int {
	total := 0
	for _, item := range line {
		total += item.width
	}
	return total
}

func lastSpaceIndex(line []styledRune) int {
	for i := len(line) - 1; i >= 0; i-- {
		if line[i].isSpace {
			return i
		}
	}
	return -1
}
