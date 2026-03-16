package editor

import (
	"strings"

	"numentext/internal/ui"

	"github.com/gdamore/tcell/v2"
)

// BlockType identifies the type of markdown block
type BlockType int

const (
	BlockNone BlockType = iota
	BlockFencedCode
	BlockBlockquote
	BlockList
	BlockTable
	BlockFrontmatter
)

// BlockInfo describes a block-level markdown element spanning multiple lines
type BlockInfo struct {
	Type      BlockType
	StartLine int // first line of the block (0-based)
	EndLine   int // last line of the block (0-based, inclusive)
	Language  string // for fenced code blocks

	// Table-specific
	ColWidths []int // calculated column widths
	ColAligns []int // alignment per column: -1=left, 0=center, 1=right
}

// MarkdownBlocks holds cached block detection results for a buffer.
type MarkdownBlocks struct {
	Blocks  []BlockInfo
	Version int // matches hlVersion when computed
}

// DetectBlocks scans lines and returns all block-level markdown elements.
func DetectBlocks(lines []string) []BlockInfo {
	var blocks []BlockInfo

	// First pass: detect frontmatter (must be at line 0)
	if fmBlock := detectFrontmatter(lines); fmBlock != nil {
		blocks = append(blocks, *fmBlock)
	}

	// Detect fenced code blocks
	blocks = append(blocks, detectFencedCodeBlocks(lines)...)

	// Build a set of lines already claimed by fenced code or frontmatter
	claimed := make(map[int]bool)
	for _, b := range blocks {
		for i := b.StartLine; i <= b.EndLine; i++ {
			claimed[i] = true
		}
	}

	// Detect tables (contiguous lines of | cells with a separator row)
	blocks = append(blocks, detectTables(lines, claimed)...)

	// Rebuild claimed set including tables
	claimed = make(map[int]bool)
	for _, b := range blocks {
		for i := b.StartLine; i <= b.EndLine; i++ {
			claimed[i] = true
		}
	}

	// Detect blockquotes (contiguous lines starting with >)
	blocks = append(blocks, detectBlockquotes(lines, claimed)...)

	// Detect lists (contiguous lines starting with list markers)
	// Rebuild claimed
	claimed = make(map[int]bool)
	for _, b := range blocks {
		for i := b.StartLine; i <= b.EndLine; i++ {
			claimed[i] = true
		}
	}
	blocks = append(blocks, detectLists(lines, claimed)...)

	return blocks
}

// IsInBlock returns the block info if lineIdx is inside any block, nil otherwise.
func IsInBlock(blocks []BlockInfo, lineIdx int) *BlockInfo {
	for i := range blocks {
		if lineIdx >= blocks[i].StartLine && lineIdx <= blocks[i].EndLine {
			return &blocks[i]
		}
	}
	return nil
}

// --- Frontmatter detection ---

func detectFrontmatter(lines []string) *BlockInfo {
	if len(lines) < 3 {
		return nil
	}
	if strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return &BlockInfo{
				Type:      BlockFrontmatter,
				StartLine: 0,
				EndLine:   i,
			}
		}
	}
	return nil
}

// --- Fenced code block detection ---

func detectFencedCodeBlocks(lines []string) []BlockInfo {
	var blocks []BlockInfo
	inFence := false
	var startLine int
	var lang string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inFence {
			if strings.HasPrefix(trimmed, "```") {
				inFence = true
				startLine = i
				lang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			}
		} else {
			if trimmed == "```" {
				blocks = append(blocks, BlockInfo{
					Type:      BlockFencedCode,
					StartLine: startLine,
					EndLine:   i,
					Language:  lang,
				})
				inFence = false
				lang = ""
			}
		}
	}
	return blocks
}

// --- Table detection ---

func isTableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) > 0 && trimmed[0] == '|' && trimmed[len(trimmed)-1] == '|'
}

func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !isTableLine(line) {
		return false
	}
	// Remove leading/trailing |, split by |, check each cell is only dashes/colons/spaces
	inner := strings.Trim(trimmed, "|")
	cells := strings.Split(inner, "|")
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if len(cell) == 0 {
			return false
		}
		for _, ch := range cell {
			if ch != '-' && ch != ':' {
				return false
			}
		}
		// Must have at least one dash
		if !strings.Contains(cell, "-") {
			return false
		}
	}
	return true
}

func detectTables(lines []string, claimed map[int]bool) []BlockInfo {
	var blocks []BlockInfo

	i := 0
	for i < len(lines) {
		if claimed[i] || !isTableLine(lines[i]) {
			i++
			continue
		}

		// Look for a separator row (must be second row of a table, or we scan ahead)
		start := i
		end := i
		hasSep := false

		for j := i; j < len(lines); j++ {
			if claimed[j] || !isTableLine(lines[j]) {
				break
			}
			if isTableSeparator(lines[j]) {
				hasSep = true
			}
			end = j
		}

		if hasSep && end > start {
			// Calculate column widths and alignments
			colWidths, colAligns := calcTableLayout(lines[start : end+1])
			blocks = append(blocks, BlockInfo{
				Type:      BlockTable,
				StartLine: start,
				EndLine:   end,
				ColWidths: colWidths,
				ColAligns: colAligns,
			})
		}

		i = end + 1
	}

	return blocks
}

func calcTableLayout(tableLines []string) ([]int, []int) {
	maxCols := 0
	var allCells [][]string

	for _, line := range tableLines {
		cells := splitTableCells(line)
		allCells = append(allCells, cells)
		if len(cells) > maxCols {
			maxCols = len(cells)
		}
	}

	colWidths := make([]int, maxCols)
	colAligns := make([]int, maxCols) // default left (-1)
	for i := range colAligns {
		colAligns[i] = -1
	}

	// Find separator row to detect alignment
	for _, cells := range allCells {
		isSep := true
		for _, cell := range cells {
			cell = strings.TrimSpace(cell)
			if len(cell) == 0 || !isSepCell(cell) {
				isSep = false
				break
			}
		}
		if isSep {
			for j, cell := range cells {
				cell = strings.TrimSpace(cell)
				if j >= maxCols {
					break
				}
				left := strings.HasPrefix(cell, ":")
				right := strings.HasSuffix(cell, ":")
				if left && right {
					colAligns[j] = 0 // center
				} else if right {
					colAligns[j] = 1 // right
				} else {
					colAligns[j] = -1 // left (default)
				}
			}
		}
	}

	// Calculate widths from non-separator rows
	for _, cells := range allCells {
		isSep := true
		for _, cell := range cells {
			if !isSepCell(strings.TrimSpace(cell)) {
				isSep = false
				break
			}
		}
		if isSep {
			continue
		}
		for j, cell := range cells {
			if j >= maxCols {
				break
			}
			w := len(strings.TrimSpace(cell))
			if w > colWidths[j] {
				colWidths[j] = w
			}
		}
	}

	// Minimum width of 3
	for i := range colWidths {
		if colWidths[i] < 3 {
			colWidths[i] = 3
		}
	}

	return colWidths, colAligns
}

func isSepCell(cell string) bool {
	if len(cell) == 0 {
		return false
	}
	for _, ch := range cell {
		if ch != '-' && ch != ':' {
			return false
		}
	}
	return strings.Contains(cell, "-")
}

func splitTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	// Remove leading and trailing |
	if len(trimmed) > 0 && trimmed[0] == '|' {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '|' {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return strings.Split(trimmed, "|")
}

// --- Blockquote detection ---

func isBlockquoteLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "> ") || trimmed == ">"
}

func detectBlockquotes(lines []string, claimed map[int]bool) []BlockInfo {
	var blocks []BlockInfo

	i := 0
	for i < len(lines) {
		if claimed[i] || !isBlockquoteLine(lines[i]) {
			i++
			continue
		}
		start := i
		for i < len(lines) && !claimed[i] && isBlockquoteLine(lines[i]) {
			i++
		}
		blocks = append(blocks, BlockInfo{
			Type:      BlockBlockquote,
			StartLine: start,
			EndLine:   i - 1,
		})
	}
	return blocks
}

// --- List detection ---

func isListLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	// Unordered: - item, * item, + item
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}
	// Checkbox variants
	if strings.HasPrefix(trimmed, "- [ ] ") || strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
		return true
	}
	// Ordered: digits followed by . or )
	for j, ch := range trimmed {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if j > 0 && (ch == '.' || ch == ')') && j+1 < len(trimmed) && trimmed[j+1] == ' ' {
			return true
		}
		break
	}
	return false
}

func detectLists(lines []string, claimed map[int]bool) []BlockInfo {
	var blocks []BlockInfo

	i := 0
	for i < len(lines) {
		if claimed[i] || !isListLine(lines[i]) {
			i++
			continue
		}
		start := i
		for i < len(lines) && !claimed[i] && isListLine(lines[i]) {
			i++
		}
		blocks = append(blocks, BlockInfo{
			Type:      BlockList,
			StartLine: start,
			EndLine:   i - 1,
		})
	}
	return blocks
}

// --- Rendering helpers ---

// RenderTableRow renders a table row with proper alignment and padding.
// Returns the rendered string for display.
func RenderTableRow(row string, colWidths []int, colAligns []int) string {
	cells := splitTableCells(row)

	// Check if this is a separator row
	isSep := true
	for _, cell := range cells {
		if !isSepCell(strings.TrimSpace(cell)) {
			isSep = false
			break
		}
	}

	var sb strings.Builder
	modern := ui.Style.Modern

	if modern {
		sb.WriteRune('\u2502') // |
	} else {
		sb.WriteByte('|')
	}

	for i := 0; i < len(colWidths); i++ {
		w := colWidths[i]
		if isSep {
			// Render separator
			for j := 0; j < w+2; j++ {
				if modern {
					sb.WriteRune('\u2500') // -
				} else {
					sb.WriteByte('-')
				}
			}
		} else {
			content := ""
			if i < len(cells) {
				content = strings.TrimSpace(cells[i])
			}
			align := -1
			if i < len(colAligns) {
				align = colAligns[i]
			}
			padded := padCell(content, w, align)
			sb.WriteByte(' ')
			sb.WriteString(padded)
			sb.WriteByte(' ')
		}
		if modern {
			sb.WriteRune('\u2502') // |
		} else {
			sb.WriteByte('|')
		}
	}

	return sb.String()
}

func padCell(content string, width, align int) string {
	contentLen := len(content)
	if contentLen >= width {
		return content[:width]
	}
	padding := width - contentLen

	switch align {
	case 0: // center
		left := padding / 2
		right := padding - left
		return strings.Repeat(" ", left) + content + strings.Repeat(" ", right)
	case 1: // right
		return strings.Repeat(" ", padding) + content
	default: // left
		return content + strings.Repeat(" ", padding)
	}
}

// BlockquoteDepth returns the nesting depth of a blockquote line (number of > prefixes).
func BlockquoteDepth(line string) int {
	trimmed := strings.TrimSpace(line)
	depth := 0
	for strings.HasPrefix(trimmed, ">") {
		depth++
		trimmed = strings.TrimPrefix(trimmed, ">")
		trimmed = strings.TrimLeft(trimmed, " ")
	}
	return depth
}

// BlockquoteContent returns the text content after stripping all > prefixes.
func BlockquoteContent(line string) string {
	trimmed := strings.TrimSpace(line)
	for strings.HasPrefix(trimmed, ">") {
		trimmed = strings.TrimPrefix(trimmed, ">")
		trimmed = strings.TrimLeft(trimmed, " ")
	}
	return trimmed
}

// ListLineInfo returns info about a list line for rendering.
type ListLineInfo struct {
	Indent     int    // number of leading spaces
	IsOrdered  bool   // true for 1. 2. etc
	IsCheckbox bool   // true for - [ ] or - [x]
	IsChecked  bool   // true for - [x] or - [X]
	Content    string // text after the marker
	RawMarker  string // the raw marker (-, *, +, 1., etc.)
}

// ParseListLine parses a list line into its components.
func ParseListLine(line string) ListLineInfo {
	info := ListLineInfo{}

	// Count leading whitespace
	for _, ch := range line {
		if ch == ' ' {
			info.Indent++
		} else if ch == '\t' {
			info.Indent += 4
		} else {
			break
		}
	}

	trimmed := strings.TrimLeft(line, " \t")

	// Checkbox: - [ ] or - [x] or - [X]
	if strings.HasPrefix(trimmed, "- [ ] ") {
		info.IsCheckbox = true
		info.IsChecked = false
		info.RawMarker = "- [ ]"
		info.Content = trimmed[6:]
		return info
	}
	if strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
		info.IsCheckbox = true
		info.IsChecked = true
		info.RawMarker = trimmed[:5]
		info.Content = trimmed[6:]
		return info
	}

	// Unordered: - item, * item, + item
	if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
		info.RawMarker = string(trimmed[0])
		info.Content = trimmed[2:]
		return info
	}

	// Ordered: digits. or digits)
	for j, ch := range trimmed {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if j > 0 && (ch == '.' || ch == ')') && j+1 < len(trimmed) && trimmed[j+1] == ' ' {
			info.IsOrdered = true
			info.RawMarker = trimmed[:j+1]
			info.Content = trimmed[j+2:]
			return info
		}
		break
	}

	// Fallback: not actually a list line
	info.Content = trimmed
	return info
}

// --- Markdown block theme colors ---
// These are added as package-level variables for easy theming.

var (
	// Fenced code block
	ColorMarkdownCodeBlockBg = tcell.NewRGBColor(0, 0, 80) // Dark blue tint

	// Blockquote
	ColorMarkdownQuoteBorder = tcell.NewRGBColor(0, 170, 170) // Cyan border bar
	ColorMarkdownQuoteBg     = tcell.NewRGBColor(0, 0, 100)   // Slightly tinted bg

	// Table
	ColorMarkdownTableBorder = tcell.NewRGBColor(0, 170, 170) // Cyan borders
	ColorMarkdownTableBg     = tcell.NewRGBColor(0, 0, 90)    // Slight tint

	// Frontmatter
	ColorMarkdownFrontmatterBg  = tcell.NewRGBColor(40, 0, 60)    // Purple tint
	ColorMarkdownFrontmatterKey = tcell.NewRGBColor(86, 156, 214) // Blue for YAML keys
	ColorMarkdownFrontmatterVal = tcell.NewRGBColor(206, 145, 120) // Orange for YAML values

	// List
	ColorMarkdownBullet   = tcell.NewRGBColor(0, 170, 170)    // Cyan bullets
	ColorMarkdownCheckbox = tcell.NewRGBColor(85, 255, 85)     // Green for checkboxes
)

// IsMarkdownFile returns true if the filename has a markdown extension.
func IsMarkdownFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown") ||
		strings.HasSuffix(lower, ".mdown") || strings.HasSuffix(lower, ".mkd")
}

// --- Draw integration helpers ---

// MarkdownBlockStyle returns the background color to apply for a line inside a block.
func MarkdownBlockStyle(block *BlockInfo) tcell.Color {
	switch block.Type {
	case BlockFencedCode:
		return ColorMarkdownCodeBlockBg
	case BlockFrontmatter:
		return ColorMarkdownFrontmatterBg
	case BlockTable:
		return ColorMarkdownTableBg
	default:
		return ui.ColorBg
	}
}

// ListBulletChar returns the bullet character based on style mode.
func ListBulletChar() rune {
	if ui.Style.Modern {
		return '\u2022' // bullet
	}
	return '*'
}

// CheckboxEmpty returns the empty checkbox character.
func CheckboxEmpty() string {
	if ui.Style.Modern {
		return "\u2610" // ballot box
	}
	return "[ ]"
}

// CheckboxChecked returns the checked checkbox character.
func CheckboxChecked() string {
	if ui.Style.Modern {
		return "\u2611" // ballot box with check
	}
	return "[x]"
}
