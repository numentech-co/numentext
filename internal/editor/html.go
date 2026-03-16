package editor

import (
	"strings"
)

// htmlEncodeMap maps characters to their HTML entity equivalents.
var htmlEncodeMap = map[rune]string{
	'&':  "&amp;",
	'<':  "&lt;",
	'>':  "&gt;",
	'"':  "&quot;",
	'\'': "&apos;",
}

// htmlDecodeMap maps HTML entity names to their character equivalents.
var htmlDecodeMap = map[string]string{
	"&amp;":    "&",
	"&lt;":     "<",
	"&gt;":     ">",
	"&quot;":   "\"",
	"&apos;":   "'",
	"&nbsp;":   "\u00A0",
	"&copy;":   "\u00A9",
	"&reg;":    "\u00AE",
	"&trade;":  "\u2122",
	"&mdash;":  "\u2014",
	"&ndash;":  "\u2013",
	"&laquo;":  "\u00AB",
	"&raquo;":  "\u00BB",
	"&bull;":   "\u2022",
	"&hellip;": "\u2026",
	"&euro;":   "\u20AC",
	"&pound;":  "\u00A3",
	"&yen;":    "\u00A5",
	"&cent;":   "\u00A2",
	"&deg;":    "\u00B0",
	"&times;":  "\u00D7",
	"&divide;": "\u00F7",
	"&plusmn;": "\u00B1",
	"&frac12;": "\u00BD",
	"&frac14;": "\u00BC",
	"&frac34;": "\u00BE",
}

// HTMLEntity represents an HTML entity for the picker dialog.
type HTMLEntity struct {
	Entity      string
	Character   string
	Description string
}

// HTMLEntities is the list of common HTML entities for the picker.
var HTMLEntities = []HTMLEntity{
	{"&nbsp;", "\u00A0", "Non-breaking space"},
	{"&lt;", "<", "Less than"},
	{"&gt;", ">", "Greater than"},
	{"&amp;", "&", "Ampersand"},
	{"&quot;", "\"", "Double quote"},
	{"&apos;", "'", "Apostrophe"},
	{"&copy;", "\u00A9", "Copyright"},
	{"&reg;", "\u00AE", "Registered"},
	{"&trade;", "\u2122", "Trademark"},
	{"&mdash;", "\u2014", "Em dash"},
	{"&ndash;", "\u2013", "En dash"},
	{"&laquo;", "\u00AB", "Left angle quote"},
	{"&raquo;", "\u00BB", "Right angle quote"},
	{"&bull;", "\u2022", "Bullet"},
	{"&hellip;", "\u2026", "Ellipsis"},
	{"&euro;", "\u20AC", "Euro sign"},
	{"&pound;", "\u00A3", "Pound sign"},
	{"&yen;", "\u00A5", "Yen sign"},
	{"&cent;", "\u00A2", "Cent sign"},
	{"&deg;", "\u00B0", "Degree"},
	{"&times;", "\u00D7", "Multiplication"},
	{"&divide;", "\u00F7", "Division"},
	{"&plusmn;", "\u00B1", "Plus-minus"},
	{"&frac12;", "\u00BD", "One half"},
	{"&frac14;", "\u00BC", "One quarter"},
	{"&frac34;", "\u00BE", "Three quarters"},
}

// HTMLEncode converts special characters to HTML entities in the given text.
func HTMLEncode(text string) string {
	var sb strings.Builder
	for _, ch := range text {
		if entity, ok := htmlEncodeMap[ch]; ok {
			sb.WriteString(entity)
		} else {
			sb.WriteRune(ch)
		}
	}
	return sb.String()
}

// HTMLDecode converts HTML entities back to their character equivalents.
func HTMLDecode(text string) string {
	result := text
	// Decode named entities (decode &amp; last so we don't double-decode)
	for entity, char := range htmlDecodeMap {
		if entity == "&amp;" {
			continue
		}
		result = strings.ReplaceAll(result, entity, char)
	}
	// Decode &amp; last
	result = strings.ReplaceAll(result, "&amp;", "&")
	return result
}

// HTMLEncodeSelection encodes HTML special characters in the selection or entire buffer.
// Returns true if any changes were made.
func (e *Editor) HTMLEncodeSelection() bool {
	tab := e.ActiveTab()
	if tab == nil {
		return false
	}

	if tab.HasSelect {
		// Encode only the selection
		sel := e.selectedText(tab)
		encoded := HTMLEncode(sel)
		if encoded == sel {
			return false
		}
		sr, sc, _, _ := e.selectionRange(tab)
		e.deleteSelection(tab)
		cursor := [2]int{sr, sc}
		newPos := tab.Buffer.Insert(sr, sc, encoded, cursor)
		tab.CursorRow = newPos[0]
		tab.CursorCol = newPos[1]
		e.clearSelection(tab)
		e.notifyChange()
		return true
	}

	// Encode entire file
	text := tab.Buffer.Text()
	encoded := HTMLEncode(text)
	if encoded == text {
		return false
	}
	tab.Buffer = NewBufferFromText(encoded)
	tab.Buffer.SetModified(true)
	tab.CursorRow = 0
	tab.CursorCol = 0
	e.clearSelection(tab)
	e.notifyChange()
	return true
}

// HTMLDecodeSelection decodes HTML entities in the selection or entire buffer.
// Returns true if any changes were made.
func (e *Editor) HTMLDecodeSelection() bool {
	tab := e.ActiveTab()
	if tab == nil {
		return false
	}

	if tab.HasSelect {
		// Decode only the selection
		sel := e.selectedText(tab)
		decoded := HTMLDecode(sel)
		if decoded == sel {
			return false
		}
		sr, sc, _, _ := e.selectionRange(tab)
		e.deleteSelection(tab)
		cursor := [2]int{sr, sc}
		newPos := tab.Buffer.Insert(sr, sc, decoded, cursor)
		tab.CursorRow = newPos[0]
		tab.CursorCol = newPos[1]
		e.clearSelection(tab)
		e.notifyChange()
		return true
	}

	// Decode entire file
	text := tab.Buffer.Text()
	decoded := HTMLDecode(text)
	if decoded == text {
		return false
	}
	tab.Buffer = NewBufferFromText(decoded)
	tab.Buffer.SetModified(true)
	tab.CursorRow = 0
	tab.CursorCol = 0
	e.clearSelection(tab)
	e.notifyChange()
	return true
}

// InsertAtCursor inserts text at the current cursor position.
func (e *Editor) InsertAtCursor(text string) {
	tab := e.ActiveTab()
	if tab == nil {
		return
	}
	if tab.HasSelect {
		e.deleteSelection(tab)
	}
	cursor := [2]int{tab.CursorRow, tab.CursorCol}
	newPos := tab.Buffer.Insert(tab.CursorRow, tab.CursorCol, text, cursor)
	tab.CursorRow = newPos[0]
	tab.CursorCol = newPos[1]
	e.notifyChange()
}
