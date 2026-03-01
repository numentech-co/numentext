package editor

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
)

// MapKey maps a tcell key event to an editor action
func MapKey(ev *tcell.EventKey) Action {
	mod := ev.Modifiers()
	key := ev.Key()

	shift := mod&tcell.ModShift != 0
	ctrl := mod&tcell.ModCtrl != 0

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
