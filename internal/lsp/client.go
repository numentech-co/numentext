package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client manages communication with a language server
type Client struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	running  bool
	nextID   int
	pending  map[int]chan *Response
	rootURI  string
	versions map[string]int // uri -> version counter

	// Callbacks
	OnDiagnostics func(params PublishDiagnosticsParams)
	OnError       func(err error)
	OnCrash       func()
}

// NewClient creates a new LSP client
func NewClient(rootDir string) *Client {
	return &Client{
		pending:  make(map[int]chan *Response),
		rootURI:  pathToURI(rootDir),
		versions: make(map[string]int),
	}
}

// Start launches the language server process
func (c *Client) Start(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", command, err)
	}

	c.mu.Lock()
	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)
	c.running = true
	c.mu.Unlock()

	go c.readLoop()
	go c.waitLoop()

	return nil
}

// Initialize sends the initialize request and waits for capabilities
func (c *Client) Initialize() (*InitializeResult, error) {
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   c.rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Completion: &CompletionClientCapabilities{
					CompletionItem: &CompletionItemCapabilities{
						SnippetSupport: false,
					},
				},
				Hover:              &HoverClientCapabilities{},
				Definition:         &DefinitionClientCapabilities{},
				PublishDiagnostics: &PublishDiagnosticsClientCapabilities{},
			},
		},
	}

	resp, err := c.call("initialize", params)
	if err != nil {
		return nil, err
	}

	var result InitializeResult
	if err := remarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode initialize result: %w", err)
	}

	// Send initialized notification
	c.notify("initialized", struct{}{})

	return &result, nil
}

// DidOpen notifies the server that a file was opened
func (c *Client) DidOpen(filePath, languageID, text string) {
	uri := pathToURI(filePath)
	c.mu.Lock()
	c.versions[uri] = 1
	c.mu.Unlock()

	c.notify("textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	})
}

// DidChange notifies the server about document changes (full sync)
func (c *Client) DidChange(filePath, text string) {
	uri := pathToURI(filePath)
	c.mu.Lock()
	c.versions[uri]++
	version := c.versions[uri]
	c.mu.Unlock()

	c.notify("textDocument/didChange", DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: text},
		},
	})
}

// DidClose notifies the server that a file was closed
func (c *Client) DidClose(filePath string) {
	uri := pathToURI(filePath)
	c.notify("textDocument/didClose", DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
}

// Completion requests completion items at the given position
func (c *Client) Completion(filePath string, line, col int) ([]CompletionItem, error) {
	uri := pathToURI(filePath)
	resp, err := c.call("textDocument/completion", CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
	})
	if err != nil {
		return nil, err
	}

	// Response can be CompletionList or []CompletionItem
	var list CompletionList
	if err := remarshal(resp.Result, &list); err != nil {
		// Try as array
		var items []CompletionItem
		if err2 := remarshal(resp.Result, &items); err2 != nil {
			return nil, fmt.Errorf("decode completion: %w", err)
		}
		return items, nil
	}
	return list.Items, nil
}

// Hover requests hover information at the given position
func (c *Client) Hover(filePath string, line, col int) (*Hover, error) {
	uri := pathToURI(filePath)
	resp, err := c.call("textDocument/hover", HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
	})
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, nil
	}

	var hover Hover
	if err := remarshal(resp.Result, &hover); err != nil {
		return nil, fmt.Errorf("decode hover: %w", err)
	}
	return &hover, nil
}

// Definition requests go-to-definition at the given position
func (c *Client) Definition(filePath string, line, col int) ([]Location, error) {
	uri := pathToURI(filePath)
	resp, err := c.call("textDocument/definition", DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: col},
	})
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, nil
	}

	// Can be Location, []Location, or LocationLink[]
	var locs []Location
	if err := remarshal(resp.Result, &locs); err != nil {
		var loc Location
		if err2 := remarshal(resp.Result, &loc); err2 != nil {
			return nil, fmt.Errorf("decode definition: %w", err)
		}
		return []Location{loc}, nil
	}
	return locs, nil
}

// DocumentSymbols requests document symbols for a file
func (c *Client) DocumentSymbols(filePath string) ([]DocumentSymbol, error) {
	uri := pathToURI(filePath)
	resp, err := c.call("textDocument/documentSymbol", DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, nil
	}

	// Response can be []DocumentSymbol (hierarchical) or []SymbolInformation (flat)
	var symbols []DocumentSymbol
	if err := remarshal(resp.Result, &symbols); err != nil {
		// Try flat SymbolInformation format and convert
		var flat []SymbolInformation
		if err2 := remarshal(resp.Result, &flat); err2 != nil {
			return nil, fmt.Errorf("decode document symbols: %w", err)
		}
		for _, si := range flat {
			symbols = append(symbols, DocumentSymbol{
				Name:           si.Name,
				Kind:           si.Kind,
				Range:          si.Location.Range,
				SelectionRange: si.Location.Range,
			})
		}
	}
	return symbols, nil
}

// Shutdown sends a shutdown request
func (c *Client) Shutdown() error {
	_, err := c.call("shutdown", nil)
	return err
}

// Exit sends an exit notification
func (c *Client) Exit() {
	c.notify("exit", nil)
}

// Stop gracefully shuts down the language server
func (c *Client) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	// Try graceful shutdown with a short timeout
	done := make(chan struct{})
	go func() {
		_ = c.Shutdown()
		c.Exit()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	c.mu.Lock()
	c.running = false
	// Close stdin to unblock readLoop
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	// Close pending channels
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	// Force kill the process
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	c.mu.Unlock()
}

// Running returns whether the server is alive
func (c *Client) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// --- Transport ---

func (c *Client) call(method string, params interface{}) (*Response, error) {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil, fmt.Errorf("LSP client not running")
	}
	c.nextID++
	id := c.nextID
	ch := make(chan *Response, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.send(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp == nil {
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-time.After(10 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("LSP request timed out: %s", method)
	}
}

func (c *Client) notify(method string, params interface{}) {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	_ = c.send(notif)
}

func (c *Client) send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	c.mu.Lock()
	stdin := c.stdin
	c.mu.Unlock()

	if stdin == nil {
		return fmt.Errorf("stdin closed")
	}

	_, err = io.WriteString(stdin, header)
	if err != nil {
		return err
	}
	_, err = stdin.Write(data)
	return err
}

func (c *Client) readLoop() {
	for {
		c.mu.Lock()
		reader := c.stdout
		running := c.running
		c.mu.Unlock()

		if !running || reader == nil {
			return
		}

		msg, err := c.readMessage(reader)
		if err != nil {
			c.mu.Lock()
			c.running = false
			// Notify all pending
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}

		c.handleMessage(msg)
	}
}

func (c *Client) readMessage(reader *bufio.Reader) (json.RawMessage, error) {
	// Read headers
	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(val)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(body), nil
}

func (c *Client) handleMessage(raw json.RawMessage) {
	// Try to determine if this is a response (has id) or notification
	var msg struct {
		ID     *int            `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  *ResponseError  `json:"error"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	if msg.ID != nil && msg.Method == "" {
		// Response
		resp := &Response{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error:   msg.Error,
		}
		if msg.Result != nil {
			var result interface{}
			_ = json.Unmarshal(msg.Result, &result)
			resp.Result = result
		}

		c.mu.Lock()
		ch, ok := c.pending[*msg.ID]
		if ok {
			delete(c.pending, *msg.ID)
		}
		c.mu.Unlock()

		if ok {
			ch <- resp
		}
	} else if msg.Method != "" {
		// Notification from server
		c.handleNotification(msg.Method, msg.Params)
	}
}

func (c *Client) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "textDocument/publishDiagnostics":
		if c.OnDiagnostics != nil {
			var p PublishDiagnosticsParams
			if err := json.Unmarshal(params, &p); err == nil {
				c.OnDiagnostics(p)
			}
		}
	}
}

func (c *Client) waitLoop() {
	if c.cmd != nil {
		_ = c.cmd.Wait()
	}
	c.mu.Lock()
	wasRunning := c.running
	c.running = false
	c.mu.Unlock()

	if wasRunning && c.OnCrash != nil {
		c.OnCrash()
	}
}

// --- Helpers ---

func pathToURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	if runtime.GOOS == "windows" {
		absPath = "/" + strings.ReplaceAll(absPath, "\\", "/")
	}
	return "file://" + url.PathEscape(absPath)
}

func URIToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		path := strings.TrimPrefix(uri, "file://")
		decoded, err := url.PathUnescape(path)
		if err != nil {
			return path
		}
		if runtime.GOOS == "windows" && strings.HasPrefix(decoded, "/") {
			decoded = strings.TrimPrefix(decoded, "/")
		}
		return decoded
	}
	return uri
}

// remarshal converts an interface{} to a typed struct via JSON
func remarshal(src interface{}, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
