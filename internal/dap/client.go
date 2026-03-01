package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Client manages communication with a debug adapter
type Client struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	running bool
	nextSeq int
	pending map[int]chan *Response

	// Callbacks
	OnStopped    func(body StoppedEventBody)
	OnOutput     func(body OutputEventBody)
	OnTerminated func()
	OnCrash      func()
}

// NewClient creates a new DAP client
func NewClient() *Client {
	return &Client{
		pending: make(map[int]chan *Response),
	}
}

// Start launches the debug adapter process
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

// Initialize sends the initialize request
func (c *Client) Initialize() (*Capabilities, error) {
	resp, err := c.call("initialize", InitializeRequestArgs{
		ClientID:        "numentext",
		ClientName:      "NumenText",
		AdapterID:       "numentext",
		LinesStartAt1:   true,
		ColumnsStartAt1: true,
		PathFormat:       "path",
	})
	if err != nil {
		return nil, err
	}

	var caps Capabilities
	if err := remarshal(resp.Body, &caps); err != nil {
		return nil, fmt.Errorf("decode capabilities: %w", err)
	}
	return &caps, nil
}

// Launch sends a launch request
func (c *Client) Launch(program string, args []string, cwd string, stopOnEntry bool) error {
	_, err := c.call("launch", LaunchRequestArgs{
		Program:     program,
		StopOnEntry: stopOnEntry,
		Args:        args,
		Cwd:         cwd,
	})
	return err
}

// ConfigurationDone signals the adapter that configuration is complete
func (c *Client) ConfigurationDone() error {
	_, err := c.call("configurationDone", nil)
	return err
}

// SetBreakpoints sets breakpoints for a source file
func (c *Client) SetBreakpoints(filePath string, lines []int) ([]Breakpoint, error) {
	bps := make([]SourceBreakpoint, len(lines))
	for i, line := range lines {
		bps[i] = SourceBreakpoint{Line: line}
	}

	resp, err := c.call("setBreakpoints", SetBreakpointsArgs{
		Source:      Source{Path: filePath},
		Breakpoints: bps,
	})
	if err != nil {
		return nil, err
	}

	var body SetBreakpointsResponseBody
	if err := remarshal(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("decode breakpoints: %w", err)
	}
	return body.Breakpoints, nil
}

// Continue resumes execution
func (c *Client) Continue(threadID int) error {
	_, err := c.call("continue", map[string]int{"threadId": threadID})
	return err
}

// Next steps over
func (c *Client) Next(threadID int) error {
	_, err := c.call("next", map[string]int{"threadId": threadID})
	return err
}

// StepIn steps into
func (c *Client) StepIn(threadID int) error {
	_, err := c.call("stepIn", map[string]int{"threadId": threadID})
	return err
}

// StepOut steps out
func (c *Client) StepOut(threadID int) error {
	_, err := c.call("stepOut", map[string]int{"threadId": threadID})
	return err
}

// Threads gets all threads
func (c *Client) Threads() ([]Thread, error) {
	resp, err := c.call("threads", nil)
	if err != nil {
		return nil, err
	}
	var body ThreadsResponseBody
	if err := remarshal(resp.Body, &body); err != nil {
		return nil, err
	}
	return body.Threads, nil
}

// StackTrace gets the stack trace for a thread
func (c *Client) StackTrace(threadID int) ([]StackFrame, error) {
	resp, err := c.call("stackTrace", StackTraceArgs{ThreadID: threadID})
	if err != nil {
		return nil, err
	}
	var body StackTraceResponseBody
	if err := remarshal(resp.Body, &body); err != nil {
		return nil, err
	}
	return body.StackFrames, nil
}

// Scopes gets scopes for a stack frame
func (c *Client) Scopes(frameID int) ([]Scope, error) {
	resp, err := c.call("scopes", ScopesArgs{FrameID: frameID})
	if err != nil {
		return nil, err
	}
	var body ScopesResponseBody
	if err := remarshal(resp.Body, &body); err != nil {
		return nil, err
	}
	return body.Scopes, nil
}

// Variables gets variables for a scope
func (c *Client) Variables(ref int) ([]Variable, error) {
	resp, err := c.call("variables", VariablesArgs{VariablesReference: ref})
	if err != nil {
		return nil, err
	}
	var body VariablesResponseBody
	if err := remarshal(resp.Body, &body); err != nil {
		return nil, err
	}
	return body.Variables, nil
}

// Disconnect ends the debug session
func (c *Client) Disconnect(terminateDebuggee bool) error {
	_, err := c.call("disconnect", map[string]bool{"terminateDebuggee": terminateDebuggee})
	return err
}

// Stop terminates the debug adapter
func (c *Client) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	_ = c.Disconnect(true)

	c.mu.Lock()
	c.running = false
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	c.mu.Unlock()
}

// Running returns whether the adapter is alive
func (c *Client) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// --- Transport ---

func (c *Client) call(command string, args interface{}) (*Response, error) {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil, fmt.Errorf("DAP client not running")
	}
	c.nextSeq++
	seq := c.nextSeq
	ch := make(chan *Response, 1)
	c.pending[seq] = ch
	c.mu.Unlock()

	req := Request{
		Seq:     seq,
		Type:    "request",
		Command: command,
		Args:    args,
	}

	if err := c.send(req); err != nil {
		c.mu.Lock()
		delete(c.pending, seq)
		c.mu.Unlock()
		return nil, err
	}

	resp := <-ch
	if resp == nil {
		return nil, fmt.Errorf("connection closed")
	}
	if !resp.Success {
		return nil, fmt.Errorf("DAP error: %s", resp.Message)
	}
	return resp, nil
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
	var msg struct {
		Seq        int             `json:"seq"`
		Type       string          `json:"type"`
		Command    string          `json:"command"`
		Event      string          `json:"event"`
		RequestSeq int             `json:"request_seq"`
		Success    bool            `json:"success"`
		Message    string          `json:"message"`
		Body       json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "response":
		resp := &Response{
			Seq:        msg.Seq,
			Type:       "response",
			RequestSeq: msg.RequestSeq,
			Success:    msg.Success,
			Command:    msg.Command,
			Message:    msg.Message,
		}
		if msg.Body != nil {
			var body interface{}
			_ = json.Unmarshal(msg.Body, &body)
			resp.Body = body
		}
		c.mu.Lock()
		ch, ok := c.pending[msg.RequestSeq]
		if ok {
			delete(c.pending, msg.RequestSeq)
		}
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	case "event":
		c.handleEvent(msg.Event, msg.Body)
	}
}

func (c *Client) handleEvent(event string, body json.RawMessage) {
	switch event {
	case "stopped":
		if c.OnStopped != nil {
			var b StoppedEventBody
			if err := json.Unmarshal(body, &b); err == nil {
				c.OnStopped(b)
			}
		}
	case "output":
		if c.OnOutput != nil {
			var b OutputEventBody
			if err := json.Unmarshal(body, &b); err == nil {
				c.OnOutput(b)
			}
		}
	case "terminated":
		if c.OnTerminated != nil {
			c.OnTerminated()
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

func remarshal(src interface{}, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
