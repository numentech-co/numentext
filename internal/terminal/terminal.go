package terminal

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

// Terminal manages a pseudo-terminal session
type Terminal struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	ptmx    *os.File
	vt      *VT
	running bool
	onData  func() // callback when new data arrives
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
	t.mu.Unlock()

	// Read PTY output in background
	go t.readLoop()

	// Wait for process exit in background
	go func() {
		_ = cmd.Wait()
		t.mu.Lock()
		t.running = false
		t.mu.Unlock()
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
			if cb != nil {
				cb()
			}
		}
		if err != nil {
			if err != io.EOF {
				// PTY closed
			}
			return
		}
	}
}

// WriteInput sends keyboard input to the PTY
func (t *Terminal) WriteInput(data []byte) {
	t.mu.Lock()
	ptmx := t.ptmx
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

// Stop terminates the shell process
func (t *Terminal) Stop() {
	t.mu.Lock()
	cmd := t.cmd
	ptmx := t.ptmx
	t.running = false
	t.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	if ptmx != nil {
		_ = ptmx.Close()
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
