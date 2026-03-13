package terminal

import "strings"

// CommandBlock represents a single command+output region in the terminal.
// Blocks are detected via OSC 133 shell integration sequences or prompt heuristics.
type CommandBlock struct {
	Command   string   // the command text (what was typed)
	Output    []string // output lines (plain text, stripped of ANSI)
	Collapsed bool     // whether this block is collapsed in the UI
	Finished  bool     // true once the command has completed (OSC 133;D received)
}

// BlockTracker accumulates CommandBlocks from OSC 133 events and heuristics.
type BlockTracker struct {
	Blocks        []*CommandBlock
	active        *CommandBlock // block being built (between prompt and completion)
	inPrompt      bool          // between OSC 133;A (prompt start) and OSC 133;B (command start)
	inCommand     bool          // between OSC 133;B and first newline (capturing command text)
	inOutput      bool          // between command start and OSC 133;D (command done)
	altScreen     bool          // alternate screen active — suspend block tracking
	hasSeenOSC133 bool          // true once any OSC 133 sequence has been seen
	promptLine    string        // accumulates prompt text for heuristic fallback
	commandLine   string        // accumulates command text
	curLine       string        // current line being written
	curCol        int           // current column position (for cursor tracking)
	pendingLine   string        // saved by CR for the next LF (\r\n handling)
	userInput     string        // accumulates typed characters from WriteInput (not PTY echo)
}

// NewBlockTracker creates a new block tracker.
func NewBlockTracker() *BlockTracker {
	return &BlockTracker{}
}

// SetAltScreen enables or disables alternate screen mode.
// When alternate screen is active, block tracking is suspended.
func (bt *BlockTracker) SetAltScreen(on bool) {
	bt.altScreen = on
}

// AltScreen returns whether alternate screen is active.
func (bt *BlockTracker) AltScreen() bool {
	return bt.altScreen
}

// HandleOSC133 processes an OSC 133 shell integration sequence.
// The param is the single letter after "133;", e.g. 'A', 'B', 'C', 'D'.
func (bt *BlockTracker) HandleOSC133(param byte) {
	if bt.altScreen {
		return
	}
	bt.hasSeenOSC133 = true
	switch param {
	case 'A': // Prompt start — a new prompt is being drawn
		bt.inPrompt = true
		bt.inCommand = false
		bt.inOutput = false
		bt.promptLine = ""
		bt.commandLine = ""
	case 'B': // Command start — user has pressed Enter, command text follows
		bt.inPrompt = false
		bt.inCommand = true
		bt.inOutput = false
	case 'C': // Command executed — output begins
		bt.inCommand = false
		bt.inOutput = true
		cmd := bt.commandLine
		if cmd == "" {
			cmd = bt.curLine
		}
		bt.active = &CommandBlock{
			Command: cmd,
		}
		bt.Blocks = append(bt.Blocks, bt.active)
		bt.curLine = ""
	case 'D': // Command finished — mark block done
		if bt.active != nil {
			bt.active.Finished = true
		}
		bt.active = nil
		bt.inPrompt = false
		bt.inCommand = false
		bt.inOutput = false
		bt.curLine = ""
	}
}

// FeedChar is called for each printable character written to the terminal.
// It helps track command text and output lines for block assembly.
func (bt *BlockTracker) FeedChar(ch rune) {
	if bt.altScreen {
		return
	}
	if bt.inCommand {
		bt.commandLine += string(ch)
	}
	bt.curLine += string(ch)
	bt.curCol++
}

// FeedCursorMove is called when the cursor moves to a new column position.
// If the cursor moves right, spaces are inserted to preserve text spacing.
// If it moves left (like CR), the curLine may be overwritten.
func (bt *BlockTracker) FeedCursorMove(newCol int) {
	if bt.altScreen {
		return
	}
	if newCol > bt.curCol {
		// Cursor moved right — fill with spaces
		gap := newCol - bt.curCol
		for i := 0; i < gap; i++ {
			bt.curLine += " "
			if bt.inCommand {
				bt.commandLine += " "
			}
		}
		bt.curCol = newCol
	} else if newCol < bt.curCol {
		// Cursor moved left — truncate curLine if longer than newCol
		runes := []rune(bt.curLine)
		if newCol < len(runes) {
			bt.curLine = string(runes[:newCol])
		}
		bt.curCol = newCol
	}
}

// FeedNewline is called when a newline (LF) occurs.
func (bt *BlockTracker) FeedNewline() {
	if bt.altScreen {
		return
	}
	// Use pendingLine if CR cleared curLine (handles \r\n sequences)
	line := bt.curLine
	if line == "" && bt.pendingLine != "" {
		line = bt.pendingLine
	}
	bt.pendingLine = ""
	// Capture output: either in OSC 133 output phase, or in heuristic mode
	// with an active unfinished block
	if bt.active != nil && !bt.active.Finished {
		if bt.inOutput || !bt.hasSeenOSC133 {
			// Skip empty leading output lines (from echoed \r\n after Enter)
			if line != "" || len(bt.active.Output) > 0 {
				bt.active.Output = append(bt.active.Output, line)
			}
		}
	}
	bt.curLine = ""
	bt.curCol = 0
}

// FeedCR is called on carriage return.
func (bt *BlockTracker) FeedCR() {
	// Save curLine before clearing — \r\n sequences need this for FeedNewline
	if bt.curLine != "" {
		bt.pendingLine = bt.curLine
	}
	bt.curLine = ""
	bt.curCol = 0
}

// HeuristicPrompt attempts prompt detection when OSC 133 is not available.
// Call this when a line ending in a common prompt char ($ % > #) is seen
// after a completed block or at the start.
func (bt *BlockTracker) HeuristicPrompt(line string) {
	if bt.altScreen {
		return
	}
	// If there's an active unfinished block, finish it
	if bt.active != nil && !bt.active.Finished {
		bt.active.Finished = true
		bt.active = nil
	}
}

// HeuristicCommand records a command submitted via Enter key press.
// The command text is the current line content.
func (bt *BlockTracker) HeuristicCommand(cmd string) {
	if bt.altScreen || cmd == "" {
		return
	}
	bt.active = &CommandBlock{
		Command: cmd,
	}
	bt.Blocks = append(bt.Blocks, bt.active)
}

// FeedUserInput records a character typed by the user (from WriteInput, not PTY echo).
// This gives us clean command text without prompt prefixes or echo artifacts.
func (bt *BlockTracker) FeedUserInput(ch rune) {
	bt.userInput += string(ch)
}

// BackspaceUserInput removes the last character from user input (for backspace/delete).
func (bt *BlockTracker) BackspaceUserInput() {
	if len(bt.userInput) > 0 {
		// Remove last rune
		runes := []rune(bt.userInput)
		bt.userInput = string(runes[:len(runes)-1])
	}
}

// ClearUserInput resets the user input buffer.
func (bt *BlockTracker) ClearUserInput() {
	bt.userInput = ""
}

// HeuristicEnter is called when the user presses Enter.
// If OSC 133 is not active and there's no current active block,
// it uses the tracked user input as the command text.
func (bt *BlockTracker) HeuristicEnter() {
	if bt.altScreen {
		return
	}
	// Only use heuristic if no OSC 133 sequence is in progress
	if bt.inPrompt || bt.inCommand || bt.inOutput {
		return
	}
	// Prefer userInput (typed chars) over curLine (PTY echo which includes prompt)
	cmd := strings.TrimSpace(bt.userInput)
	if cmd == "" {
		// Fallback to curLine with prompt stripping
		cmd = strings.TrimSpace(bt.curLine)
		if cmd != "" {
			cmd = stripPrompt(cmd)
		}
	}
	if cmd == "" {
		bt.userInput = ""
		return
	}
	// If there's an active unfinished block, finish it — the user pressing
	// Enter means a new command is being submitted. If OSC 133 were available,
	// it would handle block boundaries; heuristic mode relies on Enter presses.
	if bt.active != nil && !bt.active.Finished {
		bt.active.Finished = true
	}
	bt.active = &CommandBlock{
		Command: cmd,
	}
	bt.Blocks = append(bt.Blocks, bt.active)
	bt.curLine = ""
	bt.pendingLine = ""
	bt.userInput = ""
}

// isAtShellPrompt checks whether the current line looks like a shell prompt
// followed by user input. Shell prompts typically end in $, %, or #.
// Deliberately excludes > (used by Claude Code, PowerShell-like prompts).
func (bt *BlockTracker) isAtShellPrompt() bool {
	// Extract the prompt prefix by removing user input from the end of curLine
	line := bt.curLine
	if bt.userInput != "" && strings.HasSuffix(line, bt.userInput) {
		line = strings.TrimSuffix(line, bt.userInput)
	}
	line = strings.TrimRight(line, " ")
	if len(line) < 2 {
		return false
	}
	last := line[len(line)-1]
	return last == '$' || last == '%' || last == '#'
}

// stripPrompt removes a shell prompt prefix from a line.
// Looks for the last occurrence of common prompt characters ($ % > #)
// followed by a space, and returns everything after it.
// Returns empty string if a prompt char is found but nothing was typed after it.
func stripPrompt(line string) string {
	// Search from the right for prompt chars to handle prompts like
	// "user@host ~/dir % cmd" or "bash-5.2$ cmd"
	foundPromptChar := false
	for i := len(line) - 1; i >= 0; i-- {
		ch := line[i]
		if ch == '$' || ch == '%' || ch == '>' || ch == '#' {
			foundPromptChar = true
			rest := strings.TrimSpace(line[i+1:])
			if rest != "" {
				return rest
			}
		}
	}
	if foundPromptChar {
		return "" // prompt found but nothing typed after it
	}
	return line
}

// SnapshotVTOutput replaces the active block's tracked Output with a snapshot
// of the actual VT cell contents. This produces clean output for TUI programs
// that use cursor positioning instead of sequential line output.
func (bt *BlockTracker) SnapshotVTOutput(rows, cols int, cellFn func(row, col int) rune) {
	if bt.active == nil {
		return
	}
	bt.active.Output = nil
	for r := 0; r < rows; r++ {
		var line []rune
		lastNonSpace := -1
		for c := 0; c < cols; c++ {
			ch := cellFn(r, c)
			if ch == 0 {
				ch = ' '
			}
			line = append(line, ch)
			if ch != ' ' {
				lastNonSpace = len(line)
			}
		}
		s := ""
		if lastNonSpace > 0 {
			s = string(line[:lastNonSpace])
		}
		bt.active.Output = append(bt.active.Output, s)
	}
	// Trim trailing empty lines
	for len(bt.active.Output) > 0 && bt.active.Output[len(bt.active.Output)-1] == "" {
		bt.active.Output = bt.active.Output[:len(bt.active.Output)-1]
	}
	// Trim leading empty lines
	for len(bt.active.Output) > 0 && bt.active.Output[0] == "" {
		bt.active.Output = bt.active.Output[1:]
	}
}

// FinishActiveBlock marks the active block as finished and clears it.
func (bt *BlockTracker) FinishActiveBlock() {
	if bt.active != nil && !bt.active.Finished {
		bt.active.Finished = true
		bt.active = nil
	}
}

// IsLikelyShellPrompt checks whether curLine looks like a shell prompt.
// Requires both a prompt-ending char ($, %, #) and a user@host pattern (@).
// This is used for proactive prompt detection when a command exits.
func (bt *BlockTracker) IsLikelyShellPrompt() bool {
	line := strings.TrimRight(bt.curLine, " ")
	if len(line) < 2 {
		return false
	}
	last := line[len(line)-1]
	if last != '$' && last != '%' && last != '#' {
		return false
	}
	return strings.Contains(line, "@")
}

// HasOSC133 returns true if at least one OSC 133 sequence has been processed.
func (bt *BlockTracker) HasOSC133() bool {
	return bt.inPrompt || bt.inCommand || bt.inOutput || bt.hasSeenOSC133
}

// ActiveBlock returns the currently building block (may be nil).
func (bt *BlockTracker) ActiveBlock() *CommandBlock {
	return bt.active
}

// SelectedBlock returns the block at the given index, or nil.
func (bt *BlockTracker) SelectedBlock(idx int) *CommandBlock {
	if idx < 0 || idx >= len(bt.Blocks) {
		return nil
	}
	return bt.Blocks[idx]
}

// BlockCount returns the number of blocks tracked.
func (bt *BlockTracker) BlockCount() int {
	return len(bt.Blocks)
}

// FinishedCount returns the number of finished blocks.
func (bt *BlockTracker) FinishedCount() int {
	n := 0
	for _, blk := range bt.Blocks {
		if blk.Finished {
			n++
		}
	}
	return n
}

// PlainText returns the full text of a block: command + output joined by newlines.
func (b *CommandBlock) PlainText() string {
	result := b.Command
	for _, line := range b.Output {
		result += "\n" + line
	}
	return result
}

// OutputText returns just the output lines joined by newlines.
func (b *CommandBlock) OutputText() string {
	result := ""
	for i, line := range b.Output {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
