package lsp

// JSON-RPC 2.0 message types for LSP communication

// Request is a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *int             `json:"id,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *ResponseError   `json:"error,omitempty"`
}

// Notification is a JSON-RPC 2.0 notification (no id)
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// ResponseError is an error in a JSON-RPC response
type ResponseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// --- Initialize ---

type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Completion    *CompletionClientCapabilities    `json:"completion,omitempty"`
	Hover         *HoverClientCapabilities         `json:"hover,omitempty"`
	Definition    *DefinitionClientCapabilities    `json:"definition,omitempty"`
	PublishDiagnostics *PublishDiagnosticsClientCapabilities `json:"publishDiagnostics,omitempty"`
}

type CompletionClientCapabilities struct {
	CompletionItem *CompletionItemCapabilities `json:"completionItem,omitempty"`
}

type CompletionItemCapabilities struct {
	SnippetSupport bool `json:"snippetSupport,omitempty"`
}

type HoverClientCapabilities struct{}
type DefinitionClientCapabilities struct{}
type PublishDiagnosticsClientCapabilities struct{}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	TextDocumentSync   interface{} `json:"textDocumentSync,omitempty"`
	CompletionProvider interface{} `json:"completionProvider,omitempty"`
	HoverProvider      interface{} `json:"hoverProvider,omitempty"`
	DefinitionProvider interface{} `json:"definitionProvider,omitempty"`
}

// --- Text Document ---

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"` // full content sync for simplicity
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// --- Completion ---

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation interface{} `json:"documentation,omitempty"`
	InsertText    string `json:"insertText,omitempty"`
}

// CompletionItemKind constants
const (
	CompletionKindText          = 1
	CompletionKindMethod        = 2
	CompletionKindFunction      = 3
	CompletionKindConstructor   = 4
	CompletionKindField         = 5
	CompletionKindVariable      = 6
	CompletionKindClass         = 7
	CompletionKindInterface     = 8
	CompletionKindModule        = 9
	CompletionKindProperty      = 10
	CompletionKindKeyword       = 14
	CompletionKindSnippet       = 15
	CompletionKindFile          = 17
	CompletionKindConstant      = 21
)

// --- Hover ---

type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// --- Definition ---

type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// --- Diagnostics ---

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// DiagnosticSeverity constants
const (
	DiagnosticSeverityError       = 1
	DiagnosticSeverityWarning     = 2
	DiagnosticSeverityInformation = 3
	DiagnosticSeverityHint        = 4
)

// --- Document Symbols ---

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DocumentSymbol represents a symbol in a document (hierarchical).
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation is the flat representation (older servers).
type SymbolInformation struct {
	Name     string   `json:"name"`
	Kind     int      `json:"kind"`
	Location Location `json:"location"`
}

// --- Formatting ---

type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

// TextEdit represents a change to a text document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// SymbolKind constants
const (
	SymbolKindFile          = 1
	SymbolKindModule        = 2
	SymbolKindNamespace     = 3
	SymbolKindPackage       = 4
	SymbolKindClass         = 5
	SymbolKindMethod        = 6
	SymbolKindProperty      = 7
	SymbolKindField         = 8
	SymbolKindConstructor   = 9
	SymbolKindEnum          = 10
	SymbolKindInterface     = 11
	SymbolKindFunction      = 12
	SymbolKindVariable      = 13
	SymbolKindConstant      = 14
	SymbolKindString        = 15
	SymbolKindNumber        = 16
	SymbolKindBoolean       = 17
	SymbolKindArray         = 18
	SymbolKindStruct        = 23
)
