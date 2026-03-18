package plugin

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/gdamore/tcell/v2"

	"numentext/internal/editor"
	"numentext/internal/ui"
)

// RenderMarkdownToTview converts markdown text to tview color-tagged text.
// Uses the core ParseMarkdownLine engine to parse inline formatting,
// then converts MarkdownSegments to tview color tags.
func RenderMarkdownToTview(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var result strings.Builder

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		segments := editor.ParseMarkdownLine(line, true, tcell.StyleDefault.Foreground(ui.ColorText), i)
		for _, seg := range segments {
			result.WriteString(segmentToTview(seg))
		}
	}

	return result.String()
}

// RenderCodeToTview converts source code to tview color-tagged text
// using Chroma syntax highlighting.
func RenderCodeToTview(code, language string) string {
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		// Fall back to plain text on tokenization error
		return escapeTview(code)
	}

	var result strings.Builder
	for _, token := range iterator.Tokens() {
		fg := tokenColorForTview(token.Type)
		bold := tokenBoldForTview(token.Type)
		text := escapeTview(token.Value)
		if fg != "" || bold {
			if bold {
				result.WriteString("[::b]")
			}
			if fg != "" {
				result.WriteString("[" + fg + "]")
			}
			result.WriteString(text)
			if fg != "" {
				result.WriteString("[-]")
			}
			if bold {
				result.WriteString("[::B]")
			}
		} else {
			result.WriteString(text)
		}
	}

	return result.String()
}

// RenderCellsToTview renders a list of PanelCells to tview-formatted text.
// Code cells get syntax highlighting, markdown cells get markdown rendering,
// output and raw cells are rendered as plain text with optional labels.
func RenderCellsToTview(cells []PanelCell) string {
	var result strings.Builder

	for i, cell := range cells {
		if i > 0 {
			// Separator between cells
			result.WriteString("\n[::d]")
			result.WriteString(strings.Repeat("-", 40))
			result.WriteString("[-::-]\n")
		}

		// Type label
		label := cell.Type
		if cell.Language != "" {
			label += " (" + cell.Language + ")"
		}
		result.WriteString("[::b]" + escapeTview(label) + "[::B]\n")

		switch cell.Type {
		case "code":
			result.WriteString(RenderCodeToTview(cell.Content, cell.Language))
		case "markdown":
			result.WriteString(RenderMarkdownToTview(cell.Content))
		case "output":
			result.WriteString("[::d]")
			result.WriteString(escapeTview(cell.Content))
			result.WriteString("[::D]")
		default: // "raw" and anything else
			result.WriteString(escapeTview(cell.Content))
		}
	}

	return result.String()
}

// segmentToTview converts a MarkdownSegment to tview color-tagged text.
func segmentToTview(seg editor.MarkdownSegment) string {
	fg, _, _ := seg.Style.Decompose()
	bold := false
	italic := false
	dim := false

	// Extract attributes by checking the style
	// tcell styles don't have a direct "get attributes" method,
	// so we compare against known styles
	testStyle := tcell.StyleDefault
	if seg.Style == testStyle {
		return escapeTview(seg.Text)
	}

	// Check attributes by comparing with base style with attributes set
	_, _, attrs := seg.Style.Decompose()
	if attrs&tcell.AttrBold != 0 {
		bold = true
	}
	if attrs&tcell.AttrItalic != 0 {
		italic = true
	}
	if attrs&tcell.AttrDim != 0 {
		dim = true
	}

	text := escapeTview(seg.Text)
	if text == "" {
		return ""
	}

	var result strings.Builder

	// Build attribute string
	var attrStr string
	if bold {
		attrStr += "b"
	}
	if italic {
		attrStr += "i"
	}
	if dim {
		attrStr += "d"
	}

	fgHex := colorToHex(fg)
	if fgHex != "" || attrStr != "" {
		result.WriteString("[")
		if fgHex != "" {
			result.WriteString(fgHex)
		}
		if attrStr != "" {
			result.WriteString("::")
			result.WriteString(attrStr)
		}
		result.WriteString("]")
	}

	result.WriteString(text)

	if fgHex != "" || attrStr != "" {
		result.WriteString("[-::-]")
	}

	return result.String()
}

// colorToHex converts a tcell.Color to a tview hex color string like "#RRGGBB".
func colorToHex(c tcell.Color) string {
	if c == tcell.ColorDefault {
		return ""
	}
	r, g, b := c.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// escapeTview escapes text for safe use in tview by replacing [ and ] with
// their escaped forms.
func escapeTview(text string) string {
	// tview uses [ ] as color tag delimiters. Escape them.
	text = strings.ReplaceAll(text, "[", "[[]")
	return text
}

// tokenColorForTview returns a tview hex color string for a chroma token type.
func tokenColorForTview(t chroma.TokenType) string {
	switch {
	case t == chroma.Keyword || t == chroma.KeywordConstant || t == chroma.KeywordDeclaration ||
		t == chroma.KeywordNamespace || t == chroma.KeywordPseudo || t == chroma.KeywordReserved ||
		t == chroma.KeywordType:
		return colorToHex(ui.ColorKeyword)
	case t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo:
		return colorToHex(ui.ColorType)
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic:
		return colorToHex(ui.ColorFunc)
	case t == chroma.NameClass || t == chroma.NameDecorator:
		return colorToHex(ui.ColorType)
	case t == chroma.LiteralString || t == chroma.LiteralStringAffix || t == chroma.LiteralStringBacktick ||
		t == chroma.LiteralStringChar || t == chroma.LiteralStringDelimiter || t == chroma.LiteralStringDoc ||
		t == chroma.LiteralStringDouble || t == chroma.LiteralStringEscape || t == chroma.LiteralStringHeredoc ||
		t == chroma.LiteralStringInterpol || t == chroma.LiteralStringOther || t == chroma.LiteralStringRegex ||
		t == chroma.LiteralStringSingle || t == chroma.LiteralStringSymbol:
		return colorToHex(ui.ColorString)
	case t == chroma.LiteralNumber || t == chroma.LiteralNumberBin || t == chroma.LiteralNumberFloat ||
		t == chroma.LiteralNumberHex || t == chroma.LiteralNumberInteger || t == chroma.LiteralNumberIntegerLong ||
		t == chroma.LiteralNumberOct:
		return colorToHex(ui.ColorNumber)
	case t == chroma.Comment || t == chroma.CommentHashbang || t == chroma.CommentMultiline ||
		t == chroma.CommentPreproc || t == chroma.CommentPreprocFile || t == chroma.CommentSingle ||
		t == chroma.CommentSpecial:
		return colorToHex(ui.ColorComment)
	default:
		return ""
	}
}

// tokenBoldForTview returns whether the given token type should be bold.
func tokenBoldForTview(t chroma.TokenType) bool {
	switch {
	case t == chroma.Keyword || t == chroma.KeywordConstant || t == chroma.KeywordDeclaration ||
		t == chroma.KeywordNamespace || t == chroma.KeywordPseudo || t == chroma.KeywordReserved ||
		t == chroma.KeywordType:
		return true
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic:
		return true
	default:
		return false
	}
}

// FormatPanelSelectHighlight re-renders panel content with a highlighted row.
// Lines of content are split by newline; the selected row gets a highlight prefix.
func FormatPanelSelectHighlight(content string, selectedRow int) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		if i == selectedRow {
			result.WriteString("[#000000:#00aaaa]> ")
			result.WriteString(escapeTview(line))
			result.WriteString("[-:-]")
		} else {
			result.WriteString("  ")
			result.WriteString(escapeTview(line))
		}
	}
	return result.String()
}
