package terminal

import (
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// Terminal manages a pseudo-terminal session
type Terminal struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	ptmx      *os.File
	vt        *VT
	running   bool
	onData    func() // callback when new data arrives
	drawDirty atomic.Bool // debounce flag for redraws
	waited    chan struct{} // closed when cmd.Wait() returns
}

// NewTerminal creates a terminal with the given dimensions
func NewTerminal(cols, rows int) *Terminal {
	return &Terminal{
		vt: NewVT(cols, rows),
	}
}

// Start spawns a shell in a PTY
func (t *Terminal) Start(shell string) error {
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(t.vt.Cols()),
		Rows: uint16(t.vt.Rows()),
	})
	if err != nil {
		return err
	}

	t.mu.Lock()
	t.cmd = cmd
	t.ptmx = ptmx
	t.running = true
	t.waited = make(chan struct{})
	t.mu.Unlock()

	// Read PTY output in background
	go t.readLoop()

	// Wait for process exit in background
	go func() {
		_ = cmd.Wait()
		t.mu.Lock()
		t.running = false
		t.mu.Unlock()
		close(t.waited)
	}()

	return nil
}

func (t *Terminal) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.ptmx.Read(buf)
		if n > 0 {
			t.mu.Lock()
			t.vt.Write(buf[:n])
			cb := t.onData
			t.mu.Unlock()
			// Debounce: only call back if we haven't already queued a redraw
			if cb != nil && t.drawDirty.CompareAndSwap(false, true) {
				cb()
			}
		}
		if err != nil {
			return
		}
	}
}

// WriteInput sends keyboard input to the PTY
func (t *Terminal) WriteInput(data []byte) {
	t.mu.Lock()
	ptmx := t.ptmx
	bt := t.vt.Blocks()

	if !bt.HasOSC133() {
		// When a command is active (running), release control — don't track
		// user input or create blocks. Only detect shell prompt return on Enter.
		commandActive := bt.ActiveBlock() != nil && !bt.ActiveBlock().Finished

		if commandActive {
			// Only check Enter for shell prompt detection (command finished?)
			if len(data) == 1 && data[0] == '\r' {
				t.vt.snapshotActiveBlock()
				bt.HeuristicEnter()
			}
		} else {
			// At shell prompt — track user input for command detection
			if len(data) == 1 {
				switch data[0] {
				case '\r': // Enter
					t.vt.snapshotActiveBlock()
					prevCount := bt.BlockCount()
					bt.HeuristicEnter()
					// If a new block was created, clear VT so previous
					// command output doesn't bleed into the new display
					if bt.BlockCount() > prevCount {
						t.vt.ClearGrid()
					}
				case 0x7f, 0x08: // Backspace / BS
					bt.BackspaceUserInput()
				case 0x15: // Ctrl+U (kill line)
					bt.ClearUserInput()
				default:
					if data[0] >= 0x20 && data[0] < 0x7f {
						bt.FeedUserInput(rune(data[0]))
					}
				}
			} else if len(data) > 0 && data[0] != 0x1b {
				// Multi-byte UTF-8 character
				for _, r := range string(data) {
					if r >= 0x20 {
						bt.FeedUserInput(r)
					}
				}
			}
		}
	}
	t.mu.Unlock()

	if ptmx != nil {
		_, _ = ptmx.Write(data)
	}
}

// Resize updates the PTY and VT dimensions
func (t *Terminal) Resize(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	t.mu.Lock()
	t.vt.Resize(cols, rows)
	ptmx := t.ptmx
	t.mu.Unlock()
	if ptmx != nil {
		_ = pty.Setsize(ptmx, &pty.Winsize{
			Cols: uint16(cols),
			Rows: uint16(rows),
		})
	}
}

// VT returns the virtual terminal state (caller must not hold lock)
func (t *Terminal) VT() *VT {
	return t.vt
}

// SetOnData sets a callback invoked when new data arrives from PTY
func (t *Terminal) SetOnData(fn func()) {
	t.mu.Lock()
	t.onData = fn
	t.mu.Unlock()
}

// Running returns whether the shell process is alive
func (t *Terminal) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// RunningNoLock returns running state; caller must hold the lock
func (t *Terminal) RunningNoLock() bool {
	return t.running
}

// MarkClean resets the dirty flag after a draw
func (t *Terminal) MarkClean() {
	t.drawDirty.Store(false)
}

// Stop terminates the shell process and waits for it to exit.
func (t *Terminal) Stop() {
	t.mu.Lock()
	cmd := t.cmd
	ptmx := t.ptmx
	waited := t.waited
	t.running = false
	t.mu.Unlock()

	// Close PTY first — this unblocks readLoop and signals the shell
	if ptmx != nil {
		_ = ptmx.Close()
	}

	if cmd != nil && cmd.Process != nil {
		// Send SIGTERM, then wait for the goroutine that already called cmd.Wait()
		_ = cmd.Process.Signal(syscall.SIGTERM)

		if waited != nil {
			select {
			case <-waited:
			case <-time.After(2 * time.Second):
				// Force kill if SIGTERM didn't work
				_ = cmd.Process.Kill()
				<-waited
			}
		}
	}
}

// Lock acquires the terminal mutex for safe VT access
func (t *Terminal) Lock() {
	t.mu.Lock()
}

// Unlock releases the terminal mutex
func (t *Terminal) Unlock() {
	t.mu.Unlock()
}
