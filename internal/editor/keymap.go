package editor

import (
	"numentext/internal/editor/keymode"
)

// Action is an alias for keymode.Action to maintain backward compatibility
type Action = keymode.Action

// Re-export all action constants
const (
	ActionNone              = keymode.ActionNone
	ActionCursorLeft        = keymode.ActionCursorLeft
	ActionCursorRight       = keymode.ActionCursorRight
	ActionCursorUp          = keymode.ActionCursorUp
	ActionCursorDown        = keymode.ActionCursorDown
	ActionCursorHome        = keymode.ActionCursorHome
	ActionCursorEnd         = keymode.ActionCursorEnd
	ActionCursorPageUp      = keymode.ActionCursorPageUp
	ActionCursorPageDown    = keymode.ActionCursorPageDown
	ActionCursorWordLeft    = keymode.ActionCursorWordLeft
	ActionCursorWordRight   = keymode.ActionCursorWordRight
	ActionCursorDocStart    = keymode.ActionCursorDocStart
	ActionCursorDocEnd      = keymode.ActionCursorDocEnd
	ActionSelectLeft        = keymode.ActionSelectLeft
	ActionSelectRight       = keymode.ActionSelectRight
	ActionSelectUp          = keymode.ActionSelectUp
	ActionSelectDown        = keymode.ActionSelectDown
	ActionSelectHome        = keymode.ActionSelectHome
	ActionSelectEnd         = keymode.ActionSelectEnd
	ActionSelectPageUp      = keymode.ActionSelectPageUp
	ActionSelectPageDown    = keymode.ActionSelectPageDown
	ActionSelectWordLeft    = keymode.ActionSelectWordLeft
	ActionSelectWordRight   = keymode.ActionSelectWordRight
	ActionSelectAll         = keymode.ActionSelectAll
	ActionInsertChar        = keymode.ActionInsertChar
	ActionInsertNewline     = keymode.ActionInsertNewline
	ActionInsertTab         = keymode.ActionInsertTab
	ActionDeleteChar        = keymode.ActionDeleteChar
	ActionBackspace         = keymode.ActionBackspace
	ActionDeleteWord        = keymode.ActionDeleteWord
	ActionDeleteLine        = keymode.ActionDeleteLine
	ActionCut               = keymode.ActionCut
	ActionCopy              = keymode.ActionCopy
	ActionPaste             = keymode.ActionPaste
	ActionUndo              = keymode.ActionUndo
	ActionRedo              = keymode.ActionRedo

	// Extended actions
	ActionOverwriteChar       = keymode.ActionOverwriteChar
	ActionJoinLine            = keymode.ActionJoinLine
	ActionOpenLineBelow       = keymode.ActionOpenLineBelow
	ActionOpenLineAbove       = keymode.ActionOpenLineAbove
	ActionDeleteCharForward   = keymode.ActionDeleteCharForward
	ActionPasteAfter          = keymode.ActionPasteAfter
	ActionPasteBefore         = keymode.ActionPasteBefore
	ActionYankLine            = keymode.ActionYankLine
	ActionDeleteToLineEnd     = keymode.ActionDeleteToLineEnd
	ActionChangeToLineEnd     = keymode.ActionChangeToLineEnd
	ActionCursorFirstNonBlank = keymode.ActionCursorFirstNonBlank
	ActionSearchForward       = keymode.ActionSearchForward
	ActionSearchNext          = keymode.ActionSearchNext
	ActionSearchPrev          = keymode.ActionSearchPrev
	ActionEnterCommandMode    = keymode.ActionEnterCommandMode
	ActionSelectLine          = keymode.ActionSelectLine
	ActionExtendLineSelect    = keymode.ActionExtendLineSelect
	ActionMatchBracket        = keymode.ActionMatchBracket
)

// MapKey is re-exported from keymode for backward compatibility
var MapKey = keymode.MapKey
