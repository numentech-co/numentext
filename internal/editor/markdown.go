package editor

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"numentext/internal/ui"
)

// MarkdownSegment represents a segment of a markdown line with formatting.
// BufStart and BufEnd are byte offsets into the original line buffer (inclusive of markers).
// VisBufStart is where the visible text begins in the buffer (may differ from
// BufStart when markers are hidden -- e.g. for **bold**, BufStart points to
// the first *, but VisBufStart points to the 'b').
type MarkdownSegment struct {
	Text        string      // visible text (without markers when hidden)
	Style       tcell.Style // formatting to apply
	BufStart    int         // start byte position in buffer (includes markers)
	BufEnd      int         // end byte position in buffer (exclusive, includes markers)
	VisBufStart int         // buffer position where visible text starts
}

// seg creates a MarkdownSegment where visible text starts at BufStart (no hidden prefix).
func seg(text string, style tcell.Style, bufStart, bufEnd int) MarkdownSegment {
	return MarkdownSegment{Text: text, Style: style, BufStart: bufStart, BufEnd: bufEnd, VisBufStart: bufStart}
}

// segHidden creates a MarkdownSegment with hidden prefix markers.
// visBufStart is where the visible text actually starts in the buffer.
func segHidden(text string, style tcell.Style, bufStart, bufEnd, visBufStart int) MarkdownSegment {
	return MarkdownSegment{Text: text, Style: style, BufStart: bufStart, BufEnd: bufEnd, VisBufStart: visBufStart}
}

// ParseMarkdownLine parses a line into segments for rendering.
// When hideMarkers is true (cursor not on this line), markers are excluded from Text.
// When hideMarkers is false (cursor on this line), markers are included but styling still applies.
// lineIdx is the 0-based line number, used to avoid treating --- at the top of a file
// as a horizontal rule (YAML frontmatter).
func ParseMarkdownLine(line string, hideMarkers bool, baseStyle tcell.Style, lineIdx ...int) []MarkdownSegment {
	// Check for horizontal rule: ---, ***, ___ alone on a line (with optional whitespace)
	// But skip --- at line 0 (likely YAML frontmatter delimiter)
	trimmed := strings.TrimSpace(line)
	isTopOfFile := len(lineIdx) > 0 && lineIdx[0] == 0
	if isHorizontalRule(trimmed) && !isTopOfFile {
		if hideMarkers {
			return []MarkdownSegment{seg("", baseStyle.Foreground(ui.ColorTextMuted), 0, len(line))}
		}
		return []MarkdownSegment{seg(line, baseStyle.Foreground(ui.ColorTextMuted), 0, len(line))}
	}

	// Check for header: lines starting with # through ######
	if headerLevel, headerText, prefixLen := parseHeader(line); headerLevel > 0 {
		style := headerStyle(baseStyle, headerLevel)
		if hideMarkers {
			return []MarkdownSegment{segHidden(headerText, style, 0, len(line), prefixLen)}
		}
		// Show markers but style the whole line
		return []MarkdownSegment{
			seg(line[:prefixLen], style.Dim(true), 0, prefixLen),
			seg(headerText, style, prefixLen, len(line)),
		}
	}

	// Parse inline formatting
	return parseInlineMarkdown(line, hideMarkers, baseStyle)
}

// isHorizontalRule checks if a trimmed line is a horizontal rule.
// Must be 3+ of the same char (-, *, _) with optional spaces between.
func isHorizontalRule(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	ch := trimmed[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	count := 0
	for _, c := range trimmed {
		if c == rune(ch) {
			count++
		} else if c != ' ' {
			return false
		}
	}
	return count >= 3
}

// parseHeader checks if a line is a header. Returns (level, text, prefixLen) or (0, "", 0).
func parseHeader(line string) (int, string, int) {
	level := 0
	for i := 0; i < len(line) && i < 6; i++ {
		if line[i] == '#' {
			level++
		} else {
			break
		}
	}
	if level == 0 || level > 6 {
		return 0, "", 0
	}
	if level >= len(line) || line[level] != ' ' {
		return 0, "", 0
	}
	prefixLen := level + 1 // "### "
	text := line[prefixLen:]
	return level, text, prefixLen
}

// headerStyle returns a styled tcell.Style for a header level.
func headerStyle(base tcell.Style, level int) tcell.Style {
	switch level {
	case 1:
		return base.Foreground(ui.ColorMarkdownH1).Bold(true)
	case 2:
		return base.Foreground(ui.ColorMarkdownH2).Bold(true)
	case 3:
		return base.Foreground(ui.ColorMarkdownH3).Bold(true)
	case 4:
		return base.Foreground(ui.ColorMarkdownH4).Bold(true)
	case 5:
		return base.Foreground(ui.ColorMarkdownH5).Bold(true)
	case 6:
		return base.Foreground(ui.ColorMarkdownH6).Bold(true)
	default:
		return base.Bold(true)
	}
}

// parseInlineMarkdown parses inline formatting: bold, italic, code, links, strikethrough.
func parseInlineMarkdown(line string, hideMarkers bool, baseStyle tcell.Style) []MarkdownSegment {
	var segments []MarkdownSegment
	i := 0

	for i < len(line) {
		// Backtick code spans
		if line[i] == '`' {
			if segs, advance := parseCodeSpan(line, i, hideMarkers, baseStyle); advance > 0 {
				segments = append(segments, segs...)
				i += advance
				continue
			}
		}

		// Links: [text](url) or ![alt](url)
		if line[i] == '[' || (line[i] == '!' && i+1 < len(line) && line[i+1] == '[') {
			if segs, advance := parseLink(line, i, hideMarkers, baseStyle); advance > 0 {
				segments = append(segments, segs...)
				i += advance
				continue
			}
		}

		// Strikethrough: ~~text~~
		if i+1 < len(line) && line[i] == '~' && line[i+1] == '~' {
			if segs, advance := parseStrikethrough(line, i, hideMarkers, baseStyle); advance > 0 {
				segments = append(segments, segs...)
				i += advance
				continue
			}
		}

		// Bold+italic: ***text***
		if i+2 < len(line) && line[i] == '*' && line[i+1] == '*' && line[i+2] == '*' {
			if segs, advance := parseBoldItalic(line, i, hideMarkers, baseStyle); advance > 0 {
				segments = append(segments, segs...)
				i += advance
				continue
			}
		}

		// Bold: **text**
		if i+1 < len(line) && line[i] == '*' && line[i+1] == '*' {
			if segs, advance := parseBold(line, i, hideMarkers, baseStyle); advance > 0 {
				segments = append(segments, segs...)
				i += advance
				continue
			}
		}

		// Italic: *text*
		if line[i] == '*' {
			if segs, advance := parseItalic(line, i, hideMarkers, baseStyle); advance > 0 {
				segments = append(segments, segs...)
				i += advance
				continue
			}
		}

		// Plain text: accumulate until next potential marker
		start := i
		i++
		for i < len(line) {
			if line[i] == '*' || line[i] == '`' || line[i] == '[' || line[i] == '~' ||
				(line[i] == '!' && i+1 < len(line) && line[i+1] == '[') {
				break
			}
			i++
		}
		segments = append(segments, seg(line[start:i], baseStyle, start, i))
	}

	if len(segments) == 0 {
		return []MarkdownSegment{seg(line, baseStyle, 0, len(line))}
	}
	return segments
}

// parseBoldItalic matches ***text*** starting at pos.
func parseBoldItalic(line string, pos int, hideMarkers bool, base tcell.Style) ([]MarkdownSegment, int) {
	if pos+3 >= len(line) {
		return nil, 0
	}
	end := strings.Index(line[pos+3:], "***")
	if end < 0 || end == 0 {
		return nil, 0
	}
	end += pos + 3
	inner := line[pos+3 : end]
	total := end + 3 - pos
	style := base.Bold(true).Italic(true).Foreground(ui.ColorText)

	if hideMarkers {
		return []MarkdownSegment{segHidden(inner, style, pos, pos+total, pos+3)}, total
	}
	return []MarkdownSegment{
		seg("***", style.Dim(true), pos, pos+3),
		seg(inner, style, pos+3, end),
		seg("***", style.Dim(true), end, end+3),
	}, total
}

// parseBold matches **text** starting at pos.
func parseBold(line string, pos int, hideMarkers bool, base tcell.Style) ([]MarkdownSegment, int) {
	if pos+2 >= len(line) {
		return nil, 0
	}
	searchStart := pos + 2
	for searchStart < len(line) {
		idx := strings.Index(line[searchStart:], "**")
		if idx < 0 {
			return nil, 0
		}
		closePos := searchStart + idx
		if closePos+2 < len(line) && line[closePos+2] == '*' {
			searchStart = closePos + 3
			continue
		}
		inner := line[pos+2 : closePos]
		if len(inner) == 0 {
			return nil, 0
		}
		total := closePos + 2 - pos
		style := base.Bold(true).Foreground(ui.ColorText)

		if hideMarkers {
			return []MarkdownSegment{segHidden(inner, style, pos, pos+total, pos+2)}, total
		}
		return []MarkdownSegment{
			seg("**", style.Dim(true), pos, pos+2),
			seg(inner, style, pos+2, closePos),
			seg("**", style.Dim(true), closePos, closePos+2),
		}, total
	}
	return nil, 0
}

// parseItalic matches *text* starting at pos.
func parseItalic(line string, pos int, hideMarkers bool, base tcell.Style) ([]MarkdownSegment, int) {
	if pos+1 >= len(line) {
		return nil, 0
	}
	if line[pos+1] == '*' {
		return nil, 0
	}
	end := strings.Index(line[pos+1:], "*")
	if end < 0 || end == 0 {
		return nil, 0
	}
	end += pos + 1
	inner := line[pos+1 : end]
	total := end + 1 - pos
	style := base.Italic(true).Foreground(ui.ColorText)

	if hideMarkers {
		return []MarkdownSegment{segHidden(inner, style, pos, pos+total, pos+1)}, total
	}
	return []MarkdownSegment{
		seg("*", style.Dim(true), pos, pos+1),
		seg(inner, style, pos+1, end),
		seg("*", style.Dim(true), end, end+1),
	}, total
}

// parseCodeSpan matches `code` starting at pos.
func parseCodeSpan(line string, pos int, hideMarkers bool, base tcell.Style) ([]MarkdownSegment, int) {
	if pos+1 >= len(line) {
		return nil, 0
	}
	end := strings.Index(line[pos+1:], "`")
	if end < 0 {
		return nil, 0
	}
	end += pos + 1
	inner := line[pos+1 : end]
	total := end + 1 - pos
	style := base.Foreground(ui.ColorMarkdownCode).Background(ui.ColorMarkdownCodeBg)

	if hideMarkers {
		return []MarkdownSegment{segHidden(inner, style, pos, pos+total, pos+1)}, total
	}
	return []MarkdownSegment{
		seg("`", style.Dim(true), pos, pos+1),
		seg(inner, style, pos+1, end),
		seg("`", style.Dim(true), end, end+1),
	}, total
}

// parseLink matches [text](url) or ![alt](url) starting at pos.
func parseLink(line string, pos int, hideMarkers bool, base tcell.Style) ([]MarkdownSegment, int) {
	isImage := false
	startBracket := pos
	if line[pos] == '!' {
		isImage = true
		startBracket = pos + 1
	}
	if startBracket >= len(line) || line[startBracket] != '[' {
		return nil, 0
	}
	closeBracket := strings.Index(line[startBracket+1:], "]")
	if closeBracket < 0 {
		return nil, 0
	}
	closeBracket += startBracket + 1
	if closeBracket+1 >= len(line) || line[closeBracket+1] != '(' {
		return nil, 0
	}
	closeParen := strings.Index(line[closeBracket+2:], ")")
	if closeParen < 0 {
		return nil, 0
	}
	closeParen += closeBracket + 2

	linkText := line[startBracket+1 : closeBracket]
	total := closeParen + 1 - pos

	style := base.Foreground(ui.ColorMarkdownLink)
	if isImage {
		style = base.Foreground(ui.ColorMarkdownLink).Italic(true)
	}

	if hideMarkers {
		displayText := linkText
		if isImage {
			displayText = "[img:" + linkText + "]"
		}
		// For links, the visible text maps to the link text portion in the buffer
		visBufStart := startBracket + 1 // after '['
		return []MarkdownSegment{segHidden(displayText, style, pos, pos+total, visBufStart)}, total
	}
	return []MarkdownSegment{seg(line[pos:pos+total], style, pos, pos+total)}, total
}

// parseStrikethrough matches ~~text~~ starting at pos.
func parseStrikethrough(line string, pos int, hideMarkers bool, base tcell.Style) ([]MarkdownSegment, int) {
	if pos+2 >= len(line) {
		return nil, 0
	}
	end := strings.Index(line[pos+2:], "~~")
	if end < 0 || end == 0 {
		return nil, 0
	}
	end += pos + 2
	inner := line[pos+2 : end]
	total := end + 2 - pos
	style := base.Dim(true).Foreground(ui.ColorTextMuted)

	if hideMarkers {
		return []MarkdownSegment{segHidden(inner, style, pos, pos+total, pos+2)}, total
	}
	return []MarkdownSegment{
		seg("~~", style, pos, pos+2),
		seg(inner, style, pos+2, end),
		seg("~~", style, end, end+2),
	}, total
}

// VisualToBufferCol converts a visual screen column to a buffer column.
// Uses segments generated by ParseMarkdownLine with hideMarkers=true.
func VisualToBufferCol(segments []MarkdownSegment, visualCol int) int {
	visPos := 0
	for _, s := range segments {
		segLen := len(s.Text)
		if visPos+segLen > visualCol {
			offset := visualCol - visPos
			return s.VisBufStart + offset
		}
		visPos += segLen
	}
	if len(segments) > 0 {
		return segments[len(segments)-1].BufEnd
	}
	return 0
}

// BufferToVisualCol converts a buffer column to a visual screen column.
// Uses segments generated by ParseMarkdownLine with hideMarkers=true.
func BufferToVisualCol(segments []MarkdownSegment, bufCol int) int {
	visPos := 0
	for _, s := range segments {
		if bufCol >= s.BufStart && bufCol < s.BufEnd {
			// Map buffer position to offset within visible text
			offset := bufCol - s.VisBufStart
			if offset < 0 {
				offset = 0
			}
			textLen := len(s.Text)
			if offset > textLen {
				offset = textLen
			}
			return visPos + offset
		}
		visPos += len(s.Text)
	}
	return visPos
}

// isMarkdownFile returns true if the file path suggests a markdown file.
func isMarkdownFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}
