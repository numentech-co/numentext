package dap

// DAP (Debug Adapter Protocol) message types
// DAP uses a similar JSON-RPC-like format with Content-Length headers

// Request is a DAP request
type Request struct {
	Seq     int         `json:"seq"`
	Type    string      `json:"type"` // "request"
	Command string      `json:"command"`
	Args    interface{} `json:"arguments,omitempty"`
}

// Response is a DAP response
type Response struct {
	Seq        int         `json:"seq"`
	Type       string      `json:"type"` // "response"
	RequestSeq int         `json:"request_seq"`
	Success    bool        `json:"success"`
	Command    string      `json:"command"`
	Message    string      `json:"message,omitempty"`
	Body       interface{} `json:"body,omitempty"`
}

// Event is a DAP event
type Event struct {
	Seq   int         `json:"seq"`
	Type  string      `json:"type"` // "event"
	Event string      `json:"event"`
	Body  interface{} `json:"body,omitempty"`
}

// --- Initialize ---

type InitializeRequestArgs struct {
	ClientID                     string `json:"clientID"`
	ClientName                   string `json:"clientName"`
	AdapterID                    string `json:"adapterID"`
	LinesStartAt1                bool   `json:"linesStartAt1"`
	ColumnsStartAt1              bool   `json:"columnsStartAt1"`
	PathFormat                   string `json:"pathFormat,omitempty"`
	SupportsVariableType         bool   `json:"supportsVariableType,omitempty"`
	SupportsVariablePaging       bool   `json:"supportsVariablePaging,omitempty"`
}

type Capabilities struct {
	SupportsConfigurationDoneRequest bool `json:"supportsConfigurationDoneRequest,omitempty"`
	SupportsFunctionBreakpoints      bool `json:"supportsFunctionBreakpoints,omitempty"`
	SupportsConditionalBreakpoints   bool `json:"supportsConditionalBreakpoints,omitempty"`
	SupportsSetVariable              bool `json:"supportsSetVariable,omitempty"`
}

// --- Launch/Attach ---

type LaunchRequestArgs struct {
	Program     string `json:"program"`
	StopOnEntry bool   `json:"stopOnEntry,omitempty"`
	Args        []string `json:"args,omitempty"`
	Cwd         string `json:"cwd,omitempty"`
}

type AttachRequestArgs struct {
	ProcessID int `json:"processId,omitempty"`
}

// --- Breakpoints ---

type SetBreakpointsArgs struct {
	Source      Source             `json:"source"`
	Breakpoints []SourceBreakpoint `json:"breakpoints"`
}

type Source struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

type SourceBreakpoint struct {
	Line      int    `json:"line"`
	Condition string `json:"condition,omitempty"`
}

type Breakpoint struct {
	ID       int    `json:"id,omitempty"`
	Verified bool   `json:"verified"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message,omitempty"`
}

type SetBreakpointsResponseBody struct {
	Breakpoints []Breakpoint `json:"breakpoints"`
}

// --- Threads/Stacks ---

type Thread struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ThreadsResponseBody struct {
	Threads []Thread `json:"threads"`
}

type StackTraceArgs struct {
	ThreadID   int `json:"threadId"`
	StartFrame int `json:"startFrame,omitempty"`
	Levels     int `json:"levels,omitempty"`
}

type StackFrame struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Source *Source `json:"source,omitempty"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type StackTraceResponseBody struct {
	StackFrames []StackFrame `json:"stackFrames"`
	TotalFrames int          `json:"totalFrames,omitempty"`
}

// --- Variables ---

type ScopesArgs struct {
	FrameID int `json:"frameId"`
}

type Scope struct {
	Name               string `json:"name"`
	VariablesReference int    `json:"variablesReference"`
	Expensive          bool   `json:"expensive,omitempty"`
}

type ScopesResponseBody struct {
	Scopes []Scope `json:"scopes"`
}

type VariablesArgs struct {
	VariablesReference int `json:"variablesReference"`
}

type Variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference,omitempty"`
}

type VariablesResponseBody struct {
	Variables []Variable `json:"variables"`
}

// --- Events ---

type StoppedEventBody struct {
	Reason   string `json:"reason"` // "breakpoint", "step", "exception", etc.
	ThreadID int    `json:"threadId"`
	Text     string `json:"text,omitempty"`
}

type OutputEventBody struct {
	Category string `json:"category,omitempty"` // "console", "stdout", "stderr"
	Output   string `json:"output"`
}

type TerminatedEventBody struct {
	Restart bool `json:"restart,omitempty"`
}
