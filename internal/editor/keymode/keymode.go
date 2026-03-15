package keymode

import "github.com/gdamore/tcell/v2"

// Action represents an editor action
type Action int

const (
	ActionNone Action = iota
	// Cursor movement
	ActionCursorLeft
	ActionCursorRight
	ActionCursorUp
	ActionCursorDown
	ActionCursorHome
	ActionCursorEnd
	ActionCursorPageUp
	ActionCursorPageDown
	ActionCursorWordLeft
	ActionCursorWordRight
	ActionCursorDocStart
	ActionCursorDocEnd
	// Selection
	ActionSelectLeft
	ActionSelectRight
	ActionSelectUp
	ActionSelectDown
	ActionSelectHome
	ActionSelectEnd
	ActionSelectPageUp
	ActionSelectPageDown
	ActionSelectWordLeft
	ActionSelectWordRight
	ActionSelectAll
	// Editing
	ActionInsertChar
	ActionInsertNewline
	ActionInsertTab
	ActionDeleteChar
	ActionBackspace
	ActionDeleteWord
	ActionDeleteLine
	// Clipboard
	ActionCut
	ActionCopy
	ActionPaste
	// Undo/Redo
	ActionUndo
	ActionRedo

	// Extended actions for keyboard modes
	ActionOverwriteChar       // Replace char under cursor (overwrite mode)
	ActionJoinLine            // Vi J — join current line with next
	ActionOpenLineBelow       // Vi o — open new line below and enter insert
	ActionOpenLineAbove       // Vi O — open new line above and enter insert
	ActionDeleteCharForward   // Vi x — delete char under cursor
	ActionPasteAfter          // Vi p — paste after cursor
	ActionPasteBefore         // Vi P — paste before cursor
	ActionYankLine            // Vi yy — yank current line
	ActionDeleteToLineEnd     // Vi D — delete from cursor to end of line
	ActionChangeToLineEnd     // Vi C — delete to end of line and enter insert
	ActionCursorFirstNonBlank // Vi ^ — move to first non-blank char
	ActionSearchForward       // Vi / — enter search mode
	ActionSearchNext          // Vi n — find next match
	ActionSearchPrev          // Vi N — find previous match
	ActionEnterCommandMode    // Vi : — enter command line mode
	ActionSelectLine          // Helix x — select current line
	ActionExtendLineSelect    // Helix X — extend selection by line
	ActionMatchBracket        // Jump to matching bracket
)

// MapKey maps a tcell key event to an editor action (standard non-modal mapping)
func MapKey(ev *tcell.EventKey) Action {
	mod := ev.Modifiers()
	key := ev.Key()

	shift := mod&tcell.ModShift != 0
	ctrl := mod&tcell.ModCtrl != 0

	alt := mod&tcell.ModAlt != 0

	switch key {
	case tcell.KeyLeft:
		if ctrl && shift {
			return ActionSelectWordLeft
		}
		if ctrl {
			return ActionCursorWordLeft
		}
		if shift {
			return ActionSelectLeft
		}
		return ActionCursorLeft
	case tcell.KeyRight:
		if ctrl && shift {
			return ActionSelectWordRight
		}
		if ctrl {
			return ActionCursorWordRight
		}
		if shift {
			return ActionSelectRight
		}
		return ActionCursorRight
	case tcell.KeyUp:
		if shift {
			return ActionSelectUp
		}
		return ActionCursorUp
	case tcell.KeyDown:
		if shift {
			return ActionSelectDown
		}
		return ActionCursorDown
	case tcell.KeyHome:
		if ctrl {
			return ActionCursorDocStart
		}
		if shift {
			return ActionSelectHome
		}
		return ActionCursorHome
	case tcell.KeyEnd:
		if ctrl {
			return ActionCursorDocEnd
		}
		if shift {
			return ActionSelectEnd
		}
		return ActionCursorEnd
	case tcell.KeyPgUp:
		if shift {
			return ActionSelectPageUp
		}
		return ActionCursorPageUp
	case tcell.KeyPgDn:
		if shift {
			return ActionSelectPageDown
		}
		return ActionCursorPageDown
	case tcell.KeyEnter:
		return ActionInsertNewline
	case tcell.KeyTab:
		return ActionInsertTab
	case tcell.KeyDelete:
		if ctrl {
			return ActionDeleteWord
		}
		return ActionDeleteChar
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return ActionBackspace
	case tcell.KeyRune:
		if alt && ev.Rune() == ']' {
			return ActionMatchBracket
		}
		if ctrl {
			switch ev.Rune() {
			case 'a':
				return ActionSelectAll
			case 'c':
				return ActionCopy
			case 'x':
				return ActionCut
			case 'v':
				return ActionPaste
			case 'z':
				return ActionUndo
			case 'y':
				return ActionRedo
			case 'd':
				return ActionDeleteLine
			}
			return ActionNone
		}
		return ActionInsertChar
	}
	return ActionNone
}

// SubMode represents the current sub-mode within a keyboard mode
type SubMode int

const (
	SubModeInsert SubMode = iota
	SubModeOverwrite
	SubModeNormal
	SubModeVisual
	SubModeVisualLine
	SubModeCommand
)

// CursorStyle controls how the cursor is rendered
type CursorStyle int

const (
	CursorLine  CursorStyle = iota // Thin beam (hardware cursor)
	CursorBlock                    // Full cell with reversed colors
)

// KeyContext provides read-only editor state for key processing
type KeyContext struct {
	CursorRow    int
	CursorCol    int
	LineLen      int
	LineCount    int
	CurrentLine  string
	HasSelection bool
	PageHeight   int
}

// KeyResult holds the outcome of processing a key event
type KeyResult struct {
	Action  int    // Single action (from Action constants)
	Char    rune   // Character for insert actions
	Actions []int  // Multiple actions for compound operations (e.g., dw = [SelectWordRight, Cut])
	Handled bool   // Whether the key was consumed
}

// KeyMapper is the interface for keyboard mode implementations
type KeyMapper interface {
	// ProcessKey handles a key event and returns the resulting action(s)
	ProcessKey(ev *tcell.EventKey, ctx KeyContext) KeyResult

	// Mode returns the mode name: "Default", "Vi", "Helix"
	Mode() string

	// SubMode returns the current sub-mode
	SubMode() SubMode

	// SubModeLabel returns a display label: "INS", "OVR", "NORMAL", "VISUAL", etc.
	SubModeLabel() string

	// PendingDisplay returns text for in-progress key sequences (e.g., "d2" while typing d2w)
	PendingDisplay() string

	// CursorStyle returns the cursor rendering style for the current sub-mode
	CursorStyle() CursorStyle

	// Reset clears any pending state (called on focus/tab change)
	Reset()
}

// ViCommandCallback is called when Vi : command mode issues a command
type ViCommandCallback struct {
	OnSave     func()
	OnQuit     func()
	OnSaveQuit func()
	OnGoToLine func(line int)
}
