package keymode

import (
	"github.com/gdamore/tcell/v2"
)

// ViMode implements Vi/Vim-style modal editing
type ViMode struct {
	subMode    SubMode
	pendingOp  rune   // Pending operator: d, c, y
	countBuf   string // Accumulating count digits
	commandBuf string // : command buffer
	Callbacks  *ViCommandCallback

	// Status bar command line integration
	OnCommandStart  func(prompt string)                         // Show ":" in status bar
	OnCommandUpdate func(text string)                           // Update command text
	OnCommandEnd    func()                                      // Hide command line
}

func NewViMode() *ViMode {
	return &ViMode{
		subMode: SubModeNormal,
	}
}

func (v *ViMode) ProcessKey(ev *tcell.EventKey, ctx KeyContext) KeyResult {
	switch v.subMode {
	case SubModeInsert:
		return v.processInsert(ev, ctx)
	case SubModeNormal:
		return v.processNormal(ev, ctx)
	case SubModeVisual, SubModeVisualLine:
		return v.processVisual(ev, ctx)
	case SubModeCommand:
		return v.processCommand(ev, ctx)
	}
	return KeyResult{Handled: false}
}

func (v *ViMode) processInsert(ev *tcell.EventKey, ctx KeyContext) KeyResult {
	key := ev.Key()

	if key == tcell.KeyEscape {
		v.subMode = SubModeNormal
		// Move cursor left one if possible (Vi convention)
		return KeyResult{
			Action:  int(ActionCursorLeft),
			Handled: true,
		}
	}

	// In insert mode, use standard key mappings
	action := MapKey(ev)
	if action != ActionNone {
		return KeyResult{
			Action:  int(action),
			Char:    ev.Rune(),
			Handled: true,
		}
	}
	return KeyResult{Handled: false}
}

func (v *ViMode) processNormal(ev *tcell.EventKey, ctx KeyContext) KeyResult {
	key := ev.Key()
	mod := ev.Modifiers()
	ctrl := mod&tcell.ModCtrl != 0

	// Handle Escape — clear pending state
	if key == tcell.KeyEscape {
		v.clearPending()
		return KeyResult{Handled: true}
	}

	// Handle special keys in normal mode
	switch key {
	case tcell.KeyEnter:
		return v.result(ActionCursorDown)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return v.result(ActionCursorLeft)
	case tcell.KeyDelete:
		return v.result(ActionDeleteCharForward)
	}

	if key != tcell.KeyRune {
		// Ctrl combinations
		if ctrl {
			switch key {
			case tcell.KeyCtrlR:
				return v.result(ActionRedo)
			case tcell.KeyCtrlU:
				return v.result(ActionCursorPageUp)
			case tcell.KeyCtrlD:
				return v.result(ActionCursorPageDown)
			case tcell.KeyCtrlF:
				return v.result(ActionCursorPageDown)
			case tcell.KeyCtrlB:
				return v.result(ActionCursorPageUp)
			}
		}
		return KeyResult{Handled: false}
	}

	ch := ev.Rune()

	// Count prefix (digits 1-9, or 0 if already accumulating)
	if (ch >= '1' && ch <= '9') || (ch == '0' && v.countBuf != "") {
		v.countBuf += string(ch)
		return KeyResult{Handled: true}
	}

	count := v.getCount()

	// If there's a pending operator, this rune is the motion
	if v.pendingOp != 0 {
		return v.resolveOperatorMotion(ch, count, ctx)
	}

	// Normal mode key dispatch
	switch ch {
	// Motions
	case 'h':
		return v.repeatAction(ActionCursorLeft, count)
	case 'j':
		return v.repeatAction(ActionCursorDown, count)
	case 'k':
		return v.repeatAction(ActionCursorUp, count)
	case 'l':
		return v.repeatAction(ActionCursorRight, count)
	case 'w':
		return v.repeatAction(ActionCursorWordRight, count)
	case 'b':
		return v.repeatAction(ActionCursorWordLeft, count)
	case 'e':
		// Word end — approximate with word right
		return v.repeatAction(ActionCursorWordRight, count)
	case '0':
		return v.result(ActionCursorHome)
	case '$':
		return v.result(ActionCursorEnd)
	case '^':
		return v.result(ActionCursorFirstNonBlank)
	case 'G':
		return v.result(ActionCursorDocEnd)

	// Enter insert mode
	case 'i':
		v.subMode = SubModeInsert
		return KeyResult{Handled: true}
	case 'I':
		v.subMode = SubModeInsert
		return v.result(ActionCursorHome)
	case 'a':
		v.subMode = SubModeInsert
		return v.result(ActionCursorRight)
	case 'A':
		v.subMode = SubModeInsert
		return v.result(ActionCursorEnd)
	case 'o':
		v.subMode = SubModeInsert
		return v.result(ActionOpenLineBelow)
	case 'O':
		v.subMode = SubModeInsert
		return v.result(ActionOpenLineAbove)

	// Operators (wait for motion)
	case 'd':
		v.pendingOp = 'd'
		return KeyResult{Handled: true}
	case 'c':
		v.pendingOp = 'c'
		return KeyResult{Handled: true}
	case 'y':
		v.pendingOp = 'y'
		return KeyResult{Handled: true}

	// Single-key actions
	case 'x':
		return v.repeatAction(ActionDeleteCharForward, count)
	case 'X':
		return v.repeatAction(ActionBackspace, count)
	case 'p':
		return v.result(ActionPasteAfter)
	case 'P':
		return v.result(ActionPasteBefore)
	case 'J':
		return v.result(ActionJoinLine)
	case 'u':
		return v.result(ActionUndo)
	case 'D':
		return v.result(ActionDeleteToLineEnd)
	case 'C':
		v.subMode = SubModeInsert
		return v.result(ActionChangeToLineEnd)

	// Visual mode
	case 'v':
		v.subMode = SubModeVisual
		return KeyResult{Handled: true}
	case 'V':
		v.subMode = SubModeVisualLine
		return v.result(ActionSelectLine)

	// Search
	case '/':
		return v.result(ActionSearchForward)
	case 'n':
		return v.result(ActionSearchNext)
	case 'N':
		return v.result(ActionSearchPrev)

	// Command mode
	case ':':
		v.subMode = SubModeCommand
		v.commandBuf = ""
		if v.OnCommandStart != nil {
			v.OnCommandStart(":")
		}
		return KeyResult{Handled: true}

	// gg — go to document start
	case 'g':
		v.pendingOp = 'g'
		return KeyResult{Handled: true}
	}

	return KeyResult{Handled: false}
}

func (v *ViMode) resolveOperatorMotion(ch rune, count int, ctx KeyContext) KeyResult {
	op := v.pendingOp
	v.clearPending()

	// Handle doubled operators: dd, yy, cc, gg
	if op == ch {
		switch op {
		case 'd':
			return v.result(ActionDeleteLine)
		case 'y':
			return v.result(ActionYankLine)
		case 'c':
			// cc = change entire line
			return KeyResult{
				Actions: []int{
					int(ActionCursorHome),
					int(ActionSelectEnd),
					int(ActionCut),
				},
				Handled: true,
			}
		case 'g':
			return v.result(ActionCursorDocStart)
		}
	}

	// Handle g as prefix for other commands
	if op == 'g' {
		return KeyResult{Handled: false}
	}

	// Compose operator + motion
	actions := viOperatorMotion(op, ch, count, ctx)
	if actions == nil {
		return KeyResult{Handled: false}
	}

	result := KeyResult{
		Actions: actions,
		Handled: true,
	}

	// 'c' operator switches to insert mode after executing
	if op == 'c' {
		v.subMode = SubModeInsert
	}

	return result
}

func (v *ViMode) processVisual(ev *tcell.EventKey, ctx KeyContext) KeyResult {
	key := ev.Key()

	if key == tcell.KeyEscape {
		v.subMode = SubModeNormal
		v.clearPending()
		return KeyResult{
			Action:  int(ActionCursorLeft),
			Handled: true,
		}
	}

	if key != tcell.KeyRune {
		return KeyResult{Handled: false}
	}

	ch := ev.Rune()
	count := v.getCount()

	switch ch {
	// Selection movement
	case 'h':
		return v.repeatAction(ActionSelectLeft, count)
	case 'j':
		return v.repeatAction(ActionSelectDown, count)
	case 'k':
		return v.repeatAction(ActionSelectUp, count)
	case 'l':
		return v.repeatAction(ActionSelectRight, count)
	case 'w':
		return v.repeatAction(ActionSelectWordRight, count)
	case 'b':
		return v.repeatAction(ActionSelectWordLeft, count)
	case '0':
		return v.result(ActionSelectHome)
	case '$':
		return v.result(ActionSelectEnd)

	// Actions on selection
	case 'd', 'x':
		v.subMode = SubModeNormal
		return v.result(ActionCut)
	case 'y':
		v.subMode = SubModeNormal
		return v.result(ActionCopy)
	case 'c':
		v.subMode = SubModeInsert
		return v.result(ActionCut)
	}

	return KeyResult{Handled: false}
}

func (v *ViMode) processCommand(ev *tcell.EventKey, ctx KeyContext) KeyResult {
	key := ev.Key()

	switch key {
	case tcell.KeyEscape:
		v.subMode = SubModeNormal
		v.commandBuf = ""
		if v.OnCommandEnd != nil {
			v.OnCommandEnd()
		}
		return KeyResult{Handled: true}
	case tcell.KeyEnter:
		v.subMode = SubModeNormal
		cmd := v.commandBuf
		v.commandBuf = ""
		if v.OnCommandEnd != nil {
			v.OnCommandEnd()
		}
		viCommandParse(cmd, v.Callbacks)
		return KeyResult{Handled: true}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(v.commandBuf) > 0 {
			v.commandBuf = v.commandBuf[:len(v.commandBuf)-1]
			if v.OnCommandUpdate != nil {
				v.OnCommandUpdate(":" + v.commandBuf)
			}
		} else {
			v.subMode = SubModeNormal
			if v.OnCommandEnd != nil {
				v.OnCommandEnd()
			}
		}
		return KeyResult{Handled: true}
	case tcell.KeyRune:
		v.commandBuf += string(ev.Rune())
		if v.OnCommandUpdate != nil {
			v.OnCommandUpdate(":" + v.commandBuf)
		}
		return KeyResult{Handled: true}
	}

	return KeyResult{Handled: true}
}

func (v *ViMode) result(action Action) KeyResult {
	v.countBuf = ""
	return KeyResult{
		Action:  int(action),
		Handled: true,
	}
}

func (v *ViMode) repeatAction(action Action, count int) KeyResult {
	v.countBuf = ""
	if count <= 1 {
		return KeyResult{
			Action:  int(action),
			Handled: true,
		}
	}
	actions := make([]int, count)
	for i := range actions {
		actions[i] = int(action)
	}
	return KeyResult{
		Actions: actions,
		Handled: true,
	}
}

func (v *ViMode) getCount() int {
	if v.countBuf == "" {
		return 1
	}
	n := 0
	for _, ch := range v.countBuf {
		n = n*10 + int(ch-'0')
	}
	v.countBuf = ""
	if n < 1 {
		return 1
	}
	return n
}

func (v *ViMode) clearPending() {
	v.pendingOp = 0
	v.countBuf = ""
}

func (v *ViMode) Mode() string { return "Vi" }

func (v *ViMode) SubMode() SubMode { return v.subMode }

func (v *ViMode) SubModeLabel() string {
	switch v.subMode {
	case SubModeInsert:
		return "INSERT"
	case SubModeNormal:
		return "NORMAL"
	case SubModeVisual:
		return "VISUAL"
	case SubModeVisualLine:
		return "V-LINE"
	case SubModeCommand:
		return "COMMAND"
	}
	return "NORMAL"
}

func (v *ViMode) PendingDisplay() string {
	s := v.countBuf
	if v.pendingOp != 0 {
		s += string(v.pendingOp)
	}
	return s
}

func (v *ViMode) CursorStyle() CursorStyle {
	if v.subMode == SubModeInsert {
		return CursorLine
	}
	return CursorBlock
}

func (v *ViMode) Reset() {
	v.clearPending()
	v.commandBuf = ""
}
