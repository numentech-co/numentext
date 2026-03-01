package editor

import (
	"fmt"
	"strings"
)

// GutterWidth returns the width needed for line numbers
func GutterWidth(lineCount int) int {
	width := 1
	n := lineCount
	for n >= 10 {
		width++
		n /= 10
	}
	return width + 2 // padding on each side
}

// FormatGutterLine formats a line number for display
func FormatGutterLine(lineNum, totalLines int) string {
	width := GutterWidth(totalLines) - 1 // -1 for trailing space
	return fmt.Sprintf("%*d ", width, lineNum)
}

// FormatGutter returns all gutter lines as a string
func FormatGutter(lineCount, scrollOffset, visibleLines int) string {
	var sb strings.Builder
	for i := 0; i < visibleLines; i++ {
		lineNum := scrollOffset + i + 1
		if lineNum <= lineCount {
			sb.WriteString(FormatGutterLine(lineNum, lineCount))
		}
		if i < visibleLines-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
