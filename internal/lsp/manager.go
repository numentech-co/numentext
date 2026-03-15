package lsp

import (
	"sync"
)

// Manager manages LSP clients for different languages
type Manager struct {
	mu       sync.Mutex
	clients  map[string]*Client // keyed by server command
	rootDir  string

	OnDiagnostics func(params PublishDiagnosticsParams)
	OnStatus      func(msg string)
}

// NewManager creates a new LSP manager
func NewManager(rootDir string) *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		rootDir: rootDir,
	}
}

// ClientForFile returns (or starts) the LSP client for a file.
// Returns nil if no language server is available.
func (m *Manager) ClientForFile(filePath string) *Client {
	cfg := ServerForFile(filePath)
	if cfg == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[cfg.Command]; ok && client.Running() {
		return client
	}

	// Start new client
	client := NewClient(m.rootDir)
	client.OnDiagnostics = m.OnDiagnostics
	client.OnCrash = func() {
		if m.OnStatus != nil {
			m.OnStatus("Language server crashed: " + cfg.Command)
		}
		m.mu.Lock()
		delete(m.clients, cfg.Command)
		m.mu.Unlock()
	}

	if err := client.Start(cfg.Command, cfg.Args...); err != nil {
		if m.OnStatus != nil {
			m.OnStatus("Failed to start " + cfg.Command + ": " + err.Error())
		}
		return nil
	}

	if _, err := client.Initialize(); err != nil {
		if m.OnStatus != nil {
			m.OnStatus("LSP initialize failed: " + err.Error())
		}
		client.Stop()
		return nil
	}

	m.clients[cfg.Command] = client
	if m.OnStatus != nil {
		m.OnStatus("LSP: " + cfg.Command + " ready")
	}
	return client
}

// NotifyOpen tells the appropriate LSP server about a newly opened file
func (m *Manager) NotifyOpen(filePath, text string) {
	langID := LanguageIDForFile(filePath)
	if langID == "" {
		return
	}
	client := m.ClientForFile(filePath)
	if client == nil {
		return
	}
	client.DidOpen(filePath, langID, text)
}

// NotifyChange tells the appropriate LSP server about a document change
func (m *Manager) NotifyChange(filePath, text string) {
	cfg := ServerForFile(filePath)
	if cfg == nil {
		return
	}
	m.mu.Lock()
	client, ok := m.clients[cfg.Command]
	m.mu.Unlock()
	if !ok || !client.Running() {
		return
	}
	client.DidChange(filePath, text)
}

// NotifyClose tells the appropriate LSP server about a closed file
func (m *Manager) NotifyClose(filePath string) {
	cfg := ServerForFile(filePath)
	if cfg == nil {
		return
	}
	m.mu.Lock()
	client, ok := m.clients[cfg.Command]
	m.mu.Unlock()
	if !ok || !client.Running() {
		return
	}
	client.DidClose(filePath)
}

// Format requests formatting edits from the LSP server for a file.
// Returns nil, nil if no LSP server is available.
func (m *Manager) Format(filePath string, tabSize int, insertSpaces bool) ([]TextEdit, error) {
	client := m.ClientForFile(filePath)
	if client == nil {
		return nil, nil
	}
	return client.Format(filePath, tabSize, insertSpaces)
}

// DocumentSymbols returns document symbols for a file
func (m *Manager) DocumentSymbols(filePath string) ([]DocumentSymbol, error) {
	client := m.ClientForFile(filePath)
	if client == nil {
		return nil, nil
	}
	return client.DocumentSymbols(filePath)
}

// StopAll shuts down all running language servers
func (m *Manager) StopAll() {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.clients = make(map[string]*Client)
	m.mu.Unlock()

	for _, c := range clients {
		c.Stop()
	}
}

// RestartForFile restarts the language server for a given file type
func (m *Manager) RestartForFile(filePath string) {
	cfg := ServerForFile(filePath)
	if cfg == nil {
		return
	}
	m.mu.Lock()
	if client, ok := m.clients[cfg.Command]; ok {
		client.Stop()
		delete(m.clients, cfg.Command)
	}
	m.mu.Unlock()

	// ClientForFile will start a new one
	m.ClientForFile(filePath)
}
