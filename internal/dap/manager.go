package dap

import (
	"path/filepath"
	"sync"
)

// SessionState tracks the current debug session state
type SessionState int

const (
	SessionInactive SessionState = iota
	SessionStarting
	SessionRunning
	SessionPaused
)

// Manager manages debug sessions
type Manager struct {
	mu     sync.Mutex
	client *Client
	state  SessionState

	// Current debug context
	currentFile string
	breakpoints map[string][]int // filePath -> line numbers
	threadID    int              // current stopped thread

	// Callbacks
	OnStopped    func(file string, line int, reason string)
	OnOutput     func(text string)
	OnTerminated func()
	OnStatus     func(msg string)
}

// NewManager creates a new debug manager
func NewManager() *Manager {
	return &Manager{
		breakpoints: make(map[string][]int),
	}
}

// State returns the current session state
func (m *Manager) State() SessionState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// ToggleBreakpoint adds or removes a breakpoint at the given line
func (m *Manager) ToggleBreakpoint(filePath string, line int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lines := m.breakpoints[filePath]
	for i, l := range lines {
		if l == line {
			// Remove
			m.breakpoints[filePath] = append(lines[:i], lines[i+1:]...)
			return
		}
	}
	// Add
	m.breakpoints[filePath] = append(lines, line)
}

// Breakpoints returns the breakpoint lines for a file
func (m *Manager) Breakpoints(filePath string) []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.breakpoints[filePath]
}

// HasBreakpoint returns whether there's a breakpoint at the given line
func (m *Manager) HasBreakpoint(filePath string, line int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range m.breakpoints[filePath] {
		if l == line {
			return true
		}
	}
	return false
}

// StartSession starts a new debug session for the given file
func (m *Manager) StartSession(filePath string) error {
	cfg := AdapterForFile(filePath)
	if cfg == nil {
		if m.OnStatus != nil {
			m.OnStatus("No debug adapter for this file type")
		}
		return nil
	}

	m.mu.Lock()
	if m.client != nil {
		m.client.Stop()
	}
	m.state = SessionStarting
	m.currentFile = filePath
	m.mu.Unlock()

	client := NewClient()
	client.OnStopped = func(body StoppedEventBody) {
		m.mu.Lock()
		m.state = SessionPaused
		m.threadID = body.ThreadID
		m.mu.Unlock()

		if m.OnStopped != nil {
			// Get the current location
			frames, err := client.StackTrace(body.ThreadID)
			if err == nil && len(frames) > 0 {
				file := ""
				if frames[0].Source != nil {
					file = frames[0].Source.Path
				}
				m.OnStopped(file, frames[0].Line, body.Reason)
			}
		}
	}
	client.OnOutput = func(body OutputEventBody) {
		if m.OnOutput != nil {
			m.OnOutput(body.Output)
		}
	}
	client.OnTerminated = func() {
		m.mu.Lock()
		m.state = SessionInactive
		m.mu.Unlock()
		if m.OnTerminated != nil {
			m.OnTerminated()
		}
	}
	client.OnCrash = func() {
		m.mu.Lock()
		m.state = SessionInactive
		m.mu.Unlock()
		if m.OnStatus != nil {
			m.OnStatus("Debug adapter disconnected")
		}
	}

	if err := client.Start(cfg.Command, cfg.Args...); err != nil {
		if m.OnStatus != nil {
			m.OnStatus("Failed to start debug adapter: " + err.Error())
		}
		m.mu.Lock()
		m.state = SessionInactive
		m.mu.Unlock()
		return err
	}

	if _, err := client.Initialize(); err != nil {
		client.Stop()
		if m.OnStatus != nil {
			m.OnStatus("DAP initialize failed: " + err.Error())
		}
		m.mu.Lock()
		m.state = SessionInactive
		m.mu.Unlock()
		return err
	}

	// Set breakpoints for the file
	m.mu.Lock()
	bps := m.breakpoints[filePath]
	m.client = client
	m.mu.Unlock()

	if len(bps) > 0 {
		_, _ = client.SetBreakpoints(filePath, bps)
	}

	// Launch
	cwd := filepath.Dir(filePath)
	if err := client.Launch(filePath, nil, cwd, false); err != nil {
		client.Stop()
		if m.OnStatus != nil {
			m.OnStatus("DAP launch failed: " + err.Error())
		}
		m.mu.Lock()
		m.state = SessionInactive
		m.mu.Unlock()
		return err
	}

	_ = client.ConfigurationDone()

	m.mu.Lock()
	m.state = SessionRunning
	m.mu.Unlock()

	if m.OnStatus != nil {
		m.OnStatus("Debugging")
	}
	return nil
}

// Continue resumes execution
func (m *Manager) Continue() {
	m.mu.Lock()
	client := m.client
	threadID := m.threadID
	m.state = SessionRunning
	m.mu.Unlock()
	if client != nil {
		_ = client.Continue(threadID)
	}
}

// StepOver steps over
func (m *Manager) StepOver() {
	m.mu.Lock()
	client := m.client
	threadID := m.threadID
	m.mu.Unlock()
	if client != nil {
		_ = client.Next(threadID)
	}
}

// StepIn steps into
func (m *Manager) StepIn() {
	m.mu.Lock()
	client := m.client
	threadID := m.threadID
	m.mu.Unlock()
	if client != nil {
		_ = client.StepIn(threadID)
	}
}

// StepOut steps out
func (m *Manager) StepOut() {
	m.mu.Lock()
	client := m.client
	threadID := m.threadID
	m.mu.Unlock()
	if client != nil {
		_ = client.StepOut(threadID)
	}
}

// GetVariables returns variables for the current stopped context
func (m *Manager) GetVariables() ([]Variable, error) {
	m.mu.Lock()
	client := m.client
	threadID := m.threadID
	m.mu.Unlock()

	if client == nil {
		return nil, nil
	}

	frames, err := client.StackTrace(threadID)
	if err != nil || len(frames) == 0 {
		return nil, err
	}

	scopes, err := client.Scopes(frames[0].ID)
	if err != nil || len(scopes) == 0 {
		return nil, err
	}

	// Get variables from the first (local) scope
	return client.Variables(scopes[0].VariablesReference)
}

// StopSession ends the current debug session
func (m *Manager) StopSession() {
	m.mu.Lock()
	client := m.client
	m.client = nil
	m.state = SessionInactive
	m.mu.Unlock()

	if client != nil {
		client.Stop()
	}
	if m.OnStatus != nil {
		m.OnStatus("Debug session ended")
	}
}
