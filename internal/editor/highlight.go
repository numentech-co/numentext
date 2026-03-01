package editor

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/gdamore/tcell/v2"

	"numentext/internal/ui"
)

// CharStyle holds the style for a single character
type CharStyle struct {
	Fg   tcell.Color
	Bold bool
}

// HighlightedLine holds per-character style info for a line
type HighlightedLine struct {
	Styles []CharStyle
}

// Highlighter handles syntax highlighting using chroma
type Highlighter struct {
	lexer chroma.Lexer
	lang  string
}

func NewHighlighter(filename string) *Highlighter {
	h := &Highlighter{}
	h.DetectLanguage(filename)
	return h
}

func (h *Highlighter) DetectLanguage(filename string) {
	ext := filepath.Ext(filename)
	h.lexer = lexers.Match(filename)
	if h.lexer == nil {
		h.lexer = lexers.Fallback
	}

	switch strings.ToLower(ext) {
	case ".c", ".h":
		h.lang = "C"
	case ".cc", ".cpp", ".cxx", ".hpp", ".hxx":
		h.lang = "C++"
	case ".py":
		h.lang = "Python"
	case ".rs":
		h.lang = "Rust"
	case ".go":
		h.lang = "Go"
	case ".js":
		h.lang = "JavaScript"
	case ".ts":
		h.lang = "TypeScript"
	case ".tsx":
		h.lang = "TSX"
	case ".jsx":
		h.lang = "JSX"
	case ".java":
		h.lang = "Java"
	case ".json":
		h.lang = "JSON"
	case ".yaml", ".yml":
		h.lang = "YAML"
	case ".md":
		h.lang = "Markdown"
	case ".sh", ".bash":
		h.lang = "Shell"
	case ".html", ".htm":
		h.lang = "HTML"
	case ".css":
		h.lang = "CSS"
	default:
		h.lang = "Text"
	}
}

func (h *Highlighter) Language() string {
	return h.lang
}

// Highlight tokenizes the full text and returns per-line style info
func (h *Highlighter) Highlight(text string) []HighlightedLine {
	lines := strings.Split(text, "\n")
	result := make([]HighlightedLine, len(lines))

	// Pre-allocate styles for each line
	for i, line := range lines {
		result[i] = HighlightedLine{
			Styles: make([]CharStyle, len(line)),
		}
		// Default style
		for j := range result[i].Styles {
			result[i].Styles[j] = CharStyle{Fg: ui.ColorText}
		}
	}

	if h.lexer == nil || h.lang == "Text" {
		return result
	}

	iterator, err := h.lexer.Tokenise(nil, text)
	if err != nil {
		return result
	}

	lineIdx := 0
	colIdx := 0

	for _, token := range iterator.Tokens() {
		fg := tokenColor(token.Type)
		bold := tokenBold(token.Type)

		for _, ch := range token.Value {
			if ch == '\n' {
				lineIdx++
				colIdx = 0
				continue
			}
			if lineIdx < len(result) && colIdx < len(result[lineIdx].Styles) {
				result[lineIdx].Styles[colIdx] = CharStyle{Fg: fg, Bold: bold}
			}
			colIdx++
		}
	}

	return result
}

func tokenColor(t chroma.TokenType) tcell.Color {
	switch {
	case t == chroma.Keyword || t == chroma.KeywordConstant || t == chroma.KeywordDeclaration ||
		t == chroma.KeywordNamespace || t == chroma.KeywordPseudo || t == chroma.KeywordReserved ||
		t == chroma.KeywordType:
		return ui.ColorKeyword
	case t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo:
		return ui.ColorType
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic:
		return ui.ColorFunc
	case t == chroma.NameClass || t == chroma.NameDecorator:
		return ui.ColorType
	case t == chroma.LiteralString || t == chroma.LiteralStringAffix || t == chroma.LiteralStringBacktick ||
		t == chroma.LiteralStringChar || t == chroma.LiteralStringDelimiter || t == chroma.LiteralStringDoc ||
		t == chroma.LiteralStringDouble || t == chroma.LiteralStringEscape || t == chroma.LiteralStringHeredoc ||
		t == chroma.LiteralStringInterpol || t == chroma.LiteralStringOther || t == chroma.LiteralStringRegex ||
		t == chroma.LiteralStringSingle || t == chroma.LiteralStringSymbol:
		return ui.ColorString
	case t == chroma.LiteralNumber || t == chroma.LiteralNumberBin || t == chroma.LiteralNumberFloat ||
		t == chroma.LiteralNumberHex || t == chroma.LiteralNumberInteger || t == chroma.LiteralNumberIntegerLong ||
		t == chroma.LiteralNumberOct:
		return ui.ColorNumber
	case t == chroma.Comment || t == chroma.CommentHashbang || t == chroma.CommentMultiline ||
		t == chroma.CommentPreproc || t == chroma.CommentPreprocFile || t == chroma.CommentSingle ||
		t == chroma.CommentSpecial:
		return ui.ColorComment
	case t == chroma.Operator || t == chroma.OperatorWord:
		return ui.ColorText
	case t == chroma.Punctuation:
		return ui.ColorText
	case t == chroma.NameTag:
		return ui.ColorType
	case t == chroma.NameAttribute:
		return ui.ColorString
	default:
		return ui.ColorText
	}
}

func tokenBold(t chroma.TokenType) bool {
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
