package editor

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"

	"numentext/internal/graphics"
	"numentext/internal/ui"
)

// ensureMarkdownBlocks updates the cached block info if needed.
func (e *Editor) ensureMarkdownBlocks(tab *Tab) []BlockInfo {
	if !tab.MarkdownMode {
		return nil
	}

	if tab.mdBlocks != nil && tab.mdBlocks.Version == e.hlVersion {
		return tab.mdBlocks.Blocks
	}

	lines := make([]string, tab.Buffer.LineCount())
	for i := 0; i < tab.Buffer.LineCount(); i++ {
		lines[i] = tab.Buffer.Line(i)
	}
	blocks := DetectBlocks(lines)
	tab.mdBlocks = &MarkdownBlocks{
		Blocks:  blocks,
		Version: e.hlVersion,
	}
	return blocks
}

// isCursorInBlock returns true if the cursor is inside the given block.
func isCursorInBlock(tab *Tab, block *BlockInfo) bool {
	return tab.CursorRow >= block.StartLine && tab.CursorRow <= block.EndLine
}

// drawMarkdownBlockLine renders a markdown block line with special formatting.
// Returns true if it handled the rendering, false if normal rendering should proceed.
func (e *Editor) drawMarkdownBlockLine(screen tcell.Screen, editorX, screenY, maxWidth int, lineIdx int, tab *Tab, block *BlockInfo, highlighted []HighlightedLine) bool {
	cursorInBlock := isCursorInBlock(tab, block)

	switch block.Type {
	case BlockFencedCode:
		return e.drawFencedCodeLine(screen, editorX, screenY, maxWidth, lineIdx, tab, block, highlighted, cursorInBlock)
	case BlockBlockquote:
		return e.drawBlockquoteLine(screen, editorX, screenY, maxWidth, lineIdx, tab, block, cursorInBlock)
	case BlockList:
		return e.drawListLine(screen, editorX, screenY, maxWidth, lineIdx, tab, cursorInBlock)
	case BlockTable:
		return e.drawTableLine(screen, editorX, screenY, maxWidth, lineIdx, tab, block, cursorInBlock)
	case BlockFrontmatter:
		return e.drawFrontmatterLine(screen, editorX, screenY, maxWidth, lineIdx, tab, block, cursorInBlock)
	case BlockImage:
		return e.drawImageLine(screen, editorX, screenY, maxWidth, lineIdx, tab, block, cursorInBlock)
	}
	return false
}

// --- Fenced Code Block rendering ---

func (e *Editor) drawFencedCodeLine(screen tcell.Screen, editorX, screenY, maxWidth int, lineIdx int, tab *Tab, block *BlockInfo, highlighted []HighlightedLine, cursorInBlock bool) bool {
	line := tab.Buffer.Line(lineIdx)
	bg := ColorMarkdownCodeBlockBg
	isFenceLine := lineIdx == block.StartLine || lineIdx == block.EndLine

	if !cursorInBlock && isFenceLine {
		// Hide fence lines: fill with background
		style := tcell.StyleDefault.Background(bg)
		for cx := editorX; cx < editorX+maxWidth; cx++ {
			screen.SetContent(cx, screenY, ' ', nil, style)
		}
		return true
	}

	// Fill full width with code block background
	style := tcell.StyleDefault.Background(bg)
	for cx := editorX; cx < editorX+maxWidth; cx++ {
		screen.SetContent(cx, screenY, ' ', nil, style)
	}

	// Render the line content with syntax highlighting but code block background
	scrollScreenCol := byteOffsetToScreenCol(line, tab.ScrollCol, e.tabSize)
	screenCol := 0
	for i, ch := range line {
		if ch == '\t' {
			tabW := e.tabSize - (screenCol % e.tabSize)
			for t := 0; t < tabW; t++ {
				sx := editorX + screenCol - scrollScreenCol + t
				if sx >= editorX && sx < editorX+maxWidth {
					s := e.codeBlockCharStyle(highlighted, lineIdx, i, bg, tab)
					screen.SetContent(sx, screenY, ' ', nil, s)
				}
			}
			screenCol += tabW
			continue
		}
		sx := editorX + screenCol - scrollScreenCol
		if sx >= editorX && sx < editorX+maxWidth {
			s := e.codeBlockCharStyle(highlighted, lineIdx, i, bg, tab)
			screen.SetContent(sx, screenY, ch, nil, s)
		}
		screenCol++
	}

	return true
}

func (e *Editor) codeBlockCharStyle(highlighted []HighlightedLine, lineIdx, byteOff int, bg tcell.Color, tab *Tab) tcell.Style {
	fg := ui.ColorText
	bold := false
	if lineIdx < len(highlighted) && byteOff < len(highlighted[lineIdx].Styles) {
		fg = highlighted[lineIdx].Styles[byteOff].Fg
		bold = highlighted[lineIdx].Styles[byteOff].Bold
	}
	style := tcell.StyleDefault.Foreground(fg).Background(bg)
	if bold {
		style = style.Bold(true)
	}
	// Selection overrides
	if tab.HasSelect && e.isInSelection(tab, lineIdx, byteOff) {
		style = tcell.StyleDefault.Foreground(ui.ColorSelectedText).Background(ui.ColorSelected)
	}
	return style
}

// --- Blockquote rendering ---

func (e *Editor) drawBlockquoteLine(screen tcell.Screen, editorX, screenY, maxWidth int, lineIdx int, tab *Tab, block *BlockInfo, cursorInBlock bool) bool {
	line := tab.Buffer.Line(lineIdx)
	cursorOnLine := tab.CursorRow == lineIdx

	if cursorOnLine {
		// Show raw content - let normal rendering handle it, but with bg tint
		return false
	}

	depth := BlockquoteDepth(line)
	content := BlockquoteContent(line)

	// Fill background
	bg := ColorMarkdownQuoteBg
	style := tcell.StyleDefault.Background(bg)
	for cx := editorX; cx < editorX+maxWidth; cx++ {
		screen.SetContent(cx, screenY, ' ', nil, style)
	}

	// Draw border bars
	borderStyle := tcell.StyleDefault.Foreground(ColorMarkdownQuoteBorder).Background(bg)
	barChar := '|'
	if ui.Style.Modern {
		barChar = '\u2502' // vertical line
	}
	for d := 0; d < depth; d++ {
		sx := editorX + d*2
		if sx < editorX+maxWidth {
			screen.SetContent(sx, screenY, barChar, nil, borderStyle)
		}
		if sx+1 < editorX+maxWidth {
			screen.SetContent(sx+1, screenY, ' ', nil, style)
		}
	}

	// Draw content after the bars with inline markdown formatting
	textX := editorX + depth*2
	baseStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(bg)
	segments := ParseMarkdownLine(content, true, baseStyle)
	for _, seg := range segments {
		for _, ch := range seg.Text {
			if textX >= editorX+maxWidth {
				break
			}
			screen.SetContent(textX, screenY, ch, nil, seg.Style)
			textX++
		}
	}

	return true
}

// --- List rendering ---

func (e *Editor) drawListLine(screen tcell.Screen, editorX, screenY, maxWidth int, lineIdx int, tab *Tab, cursorInBlock bool) bool {
	cursorOnLine := tab.CursorRow == lineIdx

	if cursorOnLine {
		// Show raw content
		return false
	}

	line := tab.Buffer.Line(lineIdx)
	info := ParseListLine(line)

	// Fill background
	bg := ui.ColorBg
	style := tcell.StyleDefault.Background(bg)
	for cx := editorX; cx < editorX+maxWidth; cx++ {
		screen.SetContent(cx, screenY, ' ', nil, style)
	}

	sx := editorX + info.Indent
	if sx >= editorX+maxWidth {
		return true
	}

	if info.IsCheckbox {
		// Draw checkbox
		checkStr := CheckboxEmpty()
		checkStyle := tcell.StyleDefault.Foreground(ColorMarkdownCheckbox).Background(bg)
		if info.IsChecked {
			checkStr = CheckboxChecked()
		}
		for _, ch := range checkStr {
			if sx >= editorX+maxWidth {
				break
			}
			screen.SetContent(sx, screenY, ch, nil, checkStyle)
			sx++
		}
		// Space after checkbox
		if sx < editorX+maxWidth {
			screen.SetContent(sx, screenY, ' ', nil, style)
			sx++
		}
	} else if info.IsOrdered {
		// Ordered list: show number as-is
		numStyle := tcell.StyleDefault.Foreground(ColorMarkdownBullet).Background(bg)
		for _, ch := range info.RawMarker {
			if sx >= editorX+maxWidth {
				break
			}
			screen.SetContent(sx, screenY, ch, nil, numStyle)
			sx++
		}
		if sx < editorX+maxWidth {
			screen.SetContent(sx, screenY, ' ', nil, style)
			sx++
		}
	} else {
		// Unordered: replace marker with bullet
		bulletStyle := tcell.StyleDefault.Foreground(ColorMarkdownBullet).Background(bg)
		bullet := ListBulletChar()
		if sx < editorX+maxWidth {
			screen.SetContent(sx, screenY, bullet, nil, bulletStyle)
			sx++
		}
		if sx < editorX+maxWidth {
			screen.SetContent(sx, screenY, ' ', nil, style)
			sx++
		}
	}

	// Draw content with inline markdown formatting (bold, italic, code, links)
	baseStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(bg)
	segments := ParseMarkdownLine(info.Content, true, baseStyle)
	for _, seg := range segments {
		for _, ch := range seg.Text {
			if sx >= editorX+maxWidth {
				break
			}
			screen.SetContent(sx, screenY, ch, nil, seg.Style)
			sx++
		}
	}

	return true
}

// --- Table rendering ---

func (e *Editor) drawTableLine(screen tcell.Screen, editorX, screenY, maxWidth int, lineIdx int, tab *Tab, block *BlockInfo, cursorInBlock bool) bool {
	if cursorInBlock {
		// Show raw content for editing
		return false
	}

	line := tab.Buffer.Line(lineIdx)
	rendered := RenderTableRow(line, block.ColWidths, block.ColAligns)

	// Fill background
	bg := ColorMarkdownTableBg
	style := tcell.StyleDefault.Background(bg)
	for cx := editorX; cx < editorX+maxWidth; cx++ {
		screen.SetContent(cx, screenY, ' ', nil, style)
	}

	// Draw rendered table row
	borderStyle := tcell.StyleDefault.Foreground(ColorMarkdownTableBorder).Background(bg)
	textStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(bg)

	sx := editorX
	for _, ch := range rendered {
		if sx >= editorX+maxWidth {
			break
		}
		s := textStyle
		if ch == '|' || ch == '\u2502' || ch == '\u2500' || ch == '-' {
			s = borderStyle
		}
		screen.SetContent(sx, screenY, ch, nil, s)
		sx++
	}

	return true
}

// --- Frontmatter rendering ---

func (e *Editor) drawFrontmatterLine(screen tcell.Screen, editorX, screenY, maxWidth int, lineIdx int, tab *Tab, block *BlockInfo, cursorInBlock bool) bool {
	line := tab.Buffer.Line(lineIdx)
	bg := ColorMarkdownFrontmatterBg
	trimmed := strings.TrimSpace(line)

	// Fill background
	style := tcell.StyleDefault.Background(bg)
	for cx := editorX; cx < editorX+maxWidth; cx++ {
		screen.SetContent(cx, screenY, ' ', nil, style)
	}

	// Delimiter lines (---)
	if trimmed == "---" {
		delimStyle := tcell.StyleDefault.Foreground(ui.ColorTextMuted).Background(bg)
		sx := editorX
		for _, ch := range line {
			if sx >= editorX+maxWidth {
				break
			}
			screen.SetContent(sx, screenY, ch, nil, delimStyle)
			sx++
		}
		return true
	}

	// YAML key: value coloring
	colonIdx := strings.Index(trimmed, ":")
	if colonIdx > 0 {
		keyStyle := tcell.StyleDefault.Foreground(ColorMarkdownFrontmatterKey).Background(bg)
		valStyle := tcell.StyleDefault.Foreground(ColorMarkdownFrontmatterVal).Background(bg)

		sx := editorX
		inValue := false
		colonSeen := false
		for _, ch := range line {
			if sx >= editorX+maxWidth {
				break
			}
			if !colonSeen && ch == ':' {
				colonSeen = true
				screen.SetContent(sx, screenY, ch, nil, keyStyle)
				sx++
				inValue = true
				continue
			}
			if inValue {
				screen.SetContent(sx, screenY, ch, nil, valStyle)
			} else {
				screen.SetContent(sx, screenY, ch, nil, keyStyle)
			}
			sx++
		}
		return true
	}

	// Fallback: plain text in frontmatter
	textStyle := tcell.StyleDefault.Foreground(ui.ColorText).Background(bg)
	sx := editorX
	for _, ch := range line {
		if sx >= editorX+maxWidth {
			break
		}
		screen.SetContent(sx, screenY, ch, nil, textStyle)
		sx++
	}
	return true
}

// --- Image placeholder rendering ---

func (e *Editor) drawImageLine(screen tcell.Screen, editorX, screenY, maxWidth int, lineIdx int, tab *Tab, block *BlockInfo, cursorInBlock bool) bool {
	cursorOnLine := tab.CursorRow == lineIdx

	// Try to load the image to get dimensions and encoded data.
	var imgWidth, imgHeight int
	var cachedImg *graphics.CachedImage
	basePath := ""
	if tab.FilePath != "" {
		basePath = filepath.Dir(tab.FilePath)
	}

	if basePath != "" && block.ImagePath != "" {
		// Max allowed is 1/3 of editor area in each dimension.
		_, _, editorW, editorH := e.GetInnerRect()
		// Subtract tab bar + breadcrumb rows from height
		editorH -= 2
		if editorH < 3 {
			editorH = 3
		}
		actualCellW, actualCellH := graphics.CellSize()
		maxAllowedW := (editorW * actualCellW) / 3
		maxAllowedH := (editorH * actualCellH) / 3

		ci, err := e.imageCache.Load(block.ImagePath, basePath, maxAllowedW, e.graphicsCap, maxAllowedH)
		if err == nil {
			imgWidth = ci.OrigWidth
			imgHeight = ci.OrigHeight
			cachedImg = ci
		}
	}

	// When cursor is on the image line and terminal doesn't support graphics,
	// fall back to normal text rendering (show raw markdown syntax).
	if cursorOnLine && (cachedImg == nil || cachedImg.Encoded == "" || e.graphicsCap == graphics.GraphicsNone) {
		return false
	}

	bg := ColorMarkdownImageBg
	fg := ColorMarkdownImageFg
	style := tcell.StyleDefault.Foreground(fg).Background(bg)

	// If we have encoded image data (Sixel or Kitty), queue it for
	// post-draw output. Use floating layout: image on the left, text beside it.
	if cachedImg != nil && cachedImg.Encoded != "" && e.graphicsCap != graphics.GraphicsNone {
		// Image occupies up to 1/3 of editor width.
		// Compute terminal columns from pixel width using actual cell dimensions.
		cellW, _ := graphics.CellSize()
		imgCols := (cachedImg.Width + cellW - 1) / cellW
		maxImgCols := maxWidth / 3
		if imgCols > maxImgCols {
			imgCols = maxImgCols
		}
		if imgCols < 1 {
			imgCols = 1
		}

		// Fill the anchor line with background (clears any previous text).
		for cx := editorX; cx < editorX+maxWidth; cx++ {
			screen.SetContent(cx, screenY, ' ', nil, style)
		}

		// When cursor is on the image line, overlay the editable text
		// in the text region beside the image.
		if cursorOnLine {
			line := tab.Buffer.Line(lineIdx)
			editStyle := tcell.StyleDefault.Foreground(ui.ColorTextPrimary).Background(ui.ColorBg)
			sx := editorX + imgCols + 1
			for _, ch := range line {
				if sx >= editorX+maxWidth {
					break
				}
				screen.SetContent(sx, screenY, ch, nil, editStyle)
				sx++
			}
		}

		// Queue the image for output after the draw cycle.
		e.pendingImages = append(e.pendingImages, PendingImage{
			ScreenRow:   screenY,
			ScreenCol:   editorX,
			Width:       imgCols,
			Height:      cachedImg.TermRows,
			EncodedData: cachedImg.Encoded,
			Protocol:    e.graphicsCap,
		})

		// Set floating image state so subsequent lines flow beside the image.
		// Subtract 1 because the anchor line itself is already drawn.
		e.floatImageCols = imgCols
		e.floatImageRows = cachedImg.TermRows - 1
		e.floatImageLineIdx = lineIdx

		return true
	}

	// Fallback: render text placeholder for terminals without image support
	// or when the image could not be loaded/encoded.
	placeholder := graphics.FormatPlaceholder(block.ImageAlt, block.ImagePath, imgWidth, imgHeight)

	// Fill the full width with background.
	for cx := editorX; cx < editorX+maxWidth; cx++ {
		screen.SetContent(cx, screenY, ' ', nil, style)
	}

	// Draw the placeholder text.
	sx := editorX + 1 // slight indent
	for _, ch := range placeholder {
		if sx >= editorX+maxWidth-1 {
			break
		}
		screen.SetContent(sx, screenY, ch, nil, style)
		sx++
	}

	return true
}
