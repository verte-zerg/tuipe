// Package stats contains statistics calculations and reporting.
package stats

import (
	"strings"
	"unicode/utf8"
)

func formatTable(headers []string, rows [][]string, rightAlignCols map[int]bool) []string {
	colCount := len(headers)
	for _, row := range rows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	if colCount == 0 {
		return nil
	}

	widths := make([]int, colCount)
	for i, header := range headers {
		widths[i] = displayWidth(header)
	}
	for _, row := range rows {
		for i := 0; i < colCount; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}

	lines := make([]string, 0, len(rows)+1)
	if len(headers) > 0 {
		lines = append(lines, formatRow(headers, widths, rightAlignCols))
	}
	for _, row := range rows {
		lines = append(lines, formatRow(row, widths, rightAlignCols))
	}
	return lines
}

func formatRow(row []string, widths []int, rightAlignCols map[int]bool) string {
	var b strings.Builder
	for i := 0; i < len(widths); i++ {
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(padCell(cell, widths[i], rightAlignCols[i]))
	}
	return b.String()
}

func padCell(value string, width int, rightAlign bool) string {
	valueWidth := displayWidth(value)
	if valueWidth >= width {
		return value
	}
	padding := width - valueWidth
	if rightAlign {
		return strings.Repeat(" ", padding) + value
	}
	return value + strings.Repeat(" ", padding)
}

func displayWidth(value string) int {
	return utf8.RuneCountInString(value)
}
