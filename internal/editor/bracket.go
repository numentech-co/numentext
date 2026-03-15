package editor

import (
	"numentext/internal/ui"
)

// bracketPairs maps opening brackets to closing brackets
var bracketPairs = map[byte]byte{
	'(': ')',
	'[': ']',
	'{': '}',
}

// closingToOpening maps closing brackets to opening brackets
var closingToOpening = map[byte]byte{
	')': '(',
	']': '[',
	'}': '{',
}

// isBracket returns true if the byte is a bracket character
func isBracket(ch byte) bool {
	_, isOpen := bracketPairs[ch]
	_, isClose := closingToOpening[ch]
	return isOpen || isClose
}

// isOpenBracket returns true if the byte is an opening bracket
func isOpenBracket(ch byte) bool {
	_, ok := bracketPairs[ch]
	return ok
}

// isInStringOrComment checks if a character at the given byte offset in a line
// is inside a string or comment, based on syntax highlighting colors.
func isInStringOrComment(highlighted []HighlightedLine, line, col int) bool {
	if line < 0 || line >= len(highlighted) {
		return false
	}
	hl := highlighted[line]
	if col < 0 || col >= len(hl.Styles) {
		return false
	}
	fg := hl.Styles[col].Fg
	return fg == ui.ColorString || fg == ui.ColorComment
}

// BracketMatch holds the result of a bracket matching search
type BracketMatch struct {
	FoundBracket bool // cursor is on a bracket
	HasMatch     bool // matching bracket was found
	StartLine    int  // line of the bracket under cursor
	StartCol     int  // col (byte offset) of bracket under cursor
	MatchLine    int  // line of matching bracket
	MatchCol     int  // col (byte offset) of matching bracket
}

// FindMatchingBracket finds the matching bracket for the bracket at cursor position.
// It checks the character at cursor position and the character just before cursor.
// It uses syntax highlighting data to skip brackets inside strings and comments.
func (e *Editor) FindMatchingBracket(tab *Tab, highlighted []HighlightedLine) BracketMatch {
	if tab == nil {
		return BracketMatch{}
	}

	line := tab.Buffer.Line(tab.CursorRow)

	// Check character at cursor position first
	if tab.CursorCol < len(line) && isBracket(line[tab.CursorCol]) {
		if !isInStringOrComment(highlighted, tab.CursorRow, tab.CursorCol) {
			return e.findMatch(tab, highlighted, tab.CursorRow, tab.CursorCol, line[tab.CursorCol])
		}
	}

	// Check character just before cursor (adjacent bracket)
	if tab.CursorCol > 0 && tab.CursorCol-1 < len(line) && isBracket(line[tab.CursorCol-1]) {
		if !isInStringOrComment(highlighted, tab.CursorRow, tab.CursorCol-1) {
			return e.findMatch(tab, highlighted, tab.CursorRow, tab.CursorCol-1, line[tab.CursorCol-1])
		}
	}

	return BracketMatch{}
}

// findMatch searches for the matching bracket starting from the given position.
func (e *Editor) findMatch(tab *Tab, highlighted []HighlightedLine, row, col int, bracket byte) BracketMatch {
	result := BracketMatch{
		FoundBracket: true,
		StartLine:    row,
		StartCol:     col,
	}

	if isOpenBracket(bracket) {
		// Search forward for closing bracket
		closing := bracketPairs[bracket]
		matchRow, matchCol, found := e.searchForward(tab, highlighted, row, col, bracket, closing)
		result.HasMatch = found
		if found {
			result.MatchLine = matchRow
			result.MatchCol = matchCol
		}
	} else {
		// Search backward for opening bracket
		opening := closingToOpening[bracket]
		matchRow, matchCol, found := e.searchBackward(tab, highlighted, row, col, opening, bracket)
		result.HasMatch = found
		if found {
			result.MatchLine = matchRow
			result.MatchCol = matchCol
		}
	}

	return result
}

// searchForward scans forward from (row, col) for matching closing bracket.
// Skips brackets in strings/comments and handles nesting.
func (e *Editor) searchForward(tab *Tab, highlighted []HighlightedLine, startRow, startCol int, open, close byte) (int, int, bool) {
	depth := 1
	row := startRow
	col := startCol + 1 // start after the opening bracket

	for row < tab.Buffer.LineCount() {
		line := tab.Buffer.Line(row)
		for col < len(line) {
			ch := line[col]
			if (ch == open || ch == close) && !isInStringOrComment(highlighted, row, col) {
				if ch == open {
					depth++
				} else if ch == close {
					depth--
					if depth == 0 {
						return row, col, true
					}
				}
			}
			col++
		}
		row++
		col = 0
	}
	return 0, 0, false
}

// searchBackward scans backward from (row, col) for matching opening bracket.
// Skips brackets in strings/comments and handles nesting.
func (e *Editor) searchBackward(tab *Tab, highlighted []HighlightedLine, startRow, startCol int, open, close byte) (int, int, bool) {
	depth := 1
	row := startRow
	col := startCol - 1 // start before the closing bracket

	for row >= 0 {
		if col < 0 {
			row--
			if row < 0 {
				break
			}
			col = len(tab.Buffer.Line(row)) - 1
			continue
		}
		line := tab.Buffer.Line(row)
		for col >= 0 {
			ch := line[col]
			if (ch == open || ch == close) && !isInStringOrComment(highlighted, row, col) {
				if ch == close {
					depth++
				} else if ch == open {
					depth--
					if depth == 0 {
						return row, col, true
					}
				}
			}
			col--
		}
		row--
		if row >= 0 {
			col = len(tab.Buffer.Line(row)) - 1
		}
	}
	return 0, 0, false
}
